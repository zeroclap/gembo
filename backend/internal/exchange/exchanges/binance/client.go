// Package binance implements the exchange.Exchange interface for Binance via CCXT.
package binance

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	ccxt "github.com/ccxt/ccxt/go/v4"
	ccxtpro "github.com/ccxt/ccxt/go/v4/pro"
	"github.com/zeroclap/gembo/internal/domain"
	"github.com/zeroclap/gembo/internal/exchange/ccxtutil"
)

const (
	exchangeID         = "binance"
	fundingPollDefault = 60 * time.Second
	marketsPollDefault = 60 * time.Second
	maxBackoff         = 60 * time.Second
)

// Client is the Binance exchange adapter.
type Client struct {
	api    *ccxtpro.Binance
	logger *slog.Logger

	fundingPollInterval time.Duration
	marketsPollInterval time.Duration

	// Credentials for authenticated trading methods.
	apiKey string
	secret string
}

// New creates a new Binance client.
func New(logger *slog.Logger, opts ...Option) *Client {
	c := &Client{
		logger:              logger.With("exchange", exchangeID),
		fundingPollInterval: fundingPollDefault,
		marketsPollInterval: marketsPollDefault,
	}
	for _, o := range opts {
		o(c)
	}

	apiCfg := map[string]interface{}{
		"options": map[string]any{
			"defaultType": "swap",
			"fetchMarkets": map[string]any{
				"types": []string{"linear"},
			},
		},
	}
	if c.apiKey != "" {
		apiCfg["apiKey"] = c.apiKey
	}
	if c.secret != "" {
		apiCfg["secret"] = c.secret
	}
	c.api = ccxtpro.NewBinance(apiCfg)

	return c
}

// ID satisfies exchange.Exchange.
func (c *Client) ID() string { return exchangeID }

// Run blocks until ctx is cancelled, keeping WS connections alive.
func (c *Client) Run(ctx context.Context) error {
	<-ctx.Done()

	done := make(chan struct{})
	go func() {
		c.api.Close()
		close(done)
	}()

	select {
	case <-done:
		c.logger.Info("websocket closed cleanly")
	case <-time.After(3 * time.Second):
		c.logger.Warn("websocket close timeout, forcing shutdown")
	}

	return ctx.Err()
}

// Markets fetches all available markets via REST and returns them as domain.Market.
func (c *Client) Markets(ctx context.Context) ([]domain.Market, error) {
	raw, err := c.api.FetchMarkets()
	if err != nil {
		return nil, fmt.Errorf("binance fetch markets: %w", err)
	}

	markets := make([]domain.Market, 0, len(raw))
	now := time.Now().UTC()
	for _, m := range raw {
		dm := mapMarket(m, now)
		markets = append(markets, dm)
	}
	return markets, nil
}

// Tickers starts two WebSocket subscriptions (WatchTickers for price/volume data
// and WatchBidsAsks for bid/ask prices) and merges them into a single stream.
// Binance does not return bid/ask from WatchTickers, so WatchBidsAsks is required
// to populate the Bid and Ask fields on domain.Ticker.
//
// We subscribe to the aggregate "all tickers" streams (!miniTicker@arr and
// !bookTicker) rather than per-symbol streams. This is critical: CCXT's
// WatchMultiple creates N goroutines via FutureRace (one per symbol), and only
// the first to resolve exits cleanly — the rest leak permanently. With the
// aggregate stream there is only 1 messageHash → 1 Future → 0 leaked goroutines
// per call. We filter to the requested symbols on our side.
//
// The returned channel is closed when ctx is cancelled.
func (c *Client) Tickers(ctx context.Context, symbols []string) (<-chan domain.Ticker, error) {
	if len(symbols) == 0 {
		return nil, fmt.Errorf("binance: no symbols provided for ticker subscription")
	}

	c.logger.Info("subscribing to tickers (all-tickers stream)", "filter_symbols", len(symbols))

	// Build a set for O(1) filtering.
	allowed := make(map[string]struct{}, len(symbols))
	for _, s := range symbols {
		allowed[s] = struct{}{}
	}

	ch := make(chan domain.Ticker, 256)

	// Internal channels for each WebSocket stream.
	tickerCh := make(chan ccxt.Tickers, 64)
	bidAskCh := make(chan ccxt.Tickers, 64)

	go func() {
		defer close(ch)

		// Start the two producer goroutines plus the merge loop.
		// When ctx is cancelled all three exit and ch is closed.
		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			c.watchTickersLoop(ctx, tickerCh)
		}()
		go func() {
			defer wg.Done()
			c.watchBidsAsksLoop(ctx, bidAskCh)
		}()

		// Close internal channels once both producers are done so the
		// merge loop below can drain remaining messages and exit.
		go func() {
			wg.Wait()
			close(tickerCh)
			close(bidAskCh)
		}()

		c.mergeTickerStreams(ctx, allowed, tickerCh, bidAskCh, ch)
	}()

	return ch, nil
}

// watchTickersLoop subscribes to the Binance all-tickers aggregate stream
// (!miniTicker@arr) via WatchTickers with nil symbols. This creates a single
// messageHash and avoids the FutureRace goroutine leak.
func (c *Client) watchTickersLoop(ctx context.Context, out chan<- ccxt.Tickers) {
	backoff := ccxtutil.InitialBackoff

	for {
		if ctx.Err() != nil {
			return
		}

		tickers, err := ccxtutil.WatchWithContext(ctx, func() (ccxt.Tickers, error) {
			return c.api.WatchTickers()
		})
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			if ccxtutil.IsRetryable(err) {
				c.logger.Warn("watch tickers error, retrying",
					"error", err,
					"backoff", backoff,
				)
				select {
				case <-ctx.Done():
					return
				case <-time.After(backoff):
				}
				backoff = ccxtutil.NextBackoff(backoff, maxBackoff)
				continue
			}
			c.logger.Error("watch tickers fatal error", "error", err)
			return
		}

		backoff = ccxtutil.InitialBackoff

		select {
		case out <- tickers:
		case <-ctx.Done():
			return
		}
	}
}

// watchBidsAsksLoop subscribes to the Binance all-book-tickers aggregate stream
// (!bookTicker) via WatchBidsAsks with nil symbols. This creates a single
// messageHash and avoids the FutureRace goroutine leak.
func (c *Client) watchBidsAsksLoop(ctx context.Context, out chan<- ccxt.Tickers) {
	backoff := ccxtutil.InitialBackoff

	for {
		if ctx.Err() != nil {
			return
		}

		tickers, err := ccxtutil.WatchWithContext(ctx, func() (ccxt.Tickers, error) {
			return c.api.WatchBidsAsks()
		})
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			if ccxtutil.IsRetryable(err) {
				c.logger.Warn("watch bids/asks error, retrying",
					"error", err,
					"backoff", backoff,
				)
				select {
				case <-ctx.Done():
					return
				case <-time.After(backoff):
				}
				backoff = ccxtutil.NextBackoff(backoff, maxBackoff)
				continue
			}
			c.logger.Error("watch bids/asks fatal error", "error", err)
			return
		}

		backoff = ccxtutil.InitialBackoff

		select {
		case out <- tickers:
		case <-ctx.Done():
			return
		}
	}
}

// mergeTickerStreams reads from the ticker and bid/ask channels, maintains
// in-memory state per symbol, and emits a fully-populated domain.Ticker on
// every update from either source. Only symbols present in allowed are emitted.
func (c *Client) mergeTickerStreams(
	ctx context.Context,
	allowed map[string]struct{},
	tickerCh <-chan ccxt.Tickers,
	bidAskCh <-chan ccxt.Tickers,
	out chan<- domain.Ticker,
) {
	state := make(map[string]*domain.Ticker)

	for tickerCh != nil || bidAskCh != nil {
		select {
		case <-ctx.Done():
			return

		case tickers, ok := <-tickerCh:
			if !ok {
				tickerCh = nil
				continue
			}
			now := time.Now().UTC()
			for _, t := range tickers.Tickers {
				dt := mapTicker(t, now)
				if dt.Symbol == "" {
					continue
				}
				if _, ok := allowed[dt.Symbol]; !ok {
					continue
				}
				merged := applyTickerUpdate(state, dt)
				select {
				case out <- *merged:
				case <-ctx.Done():
					return
				}
			}

		case tickers, ok := <-bidAskCh:
			if !ok {
				bidAskCh = nil
				continue
			}
			now := time.Now().UTC()
			for _, t := range tickers.Tickers {
				dt := mapBidAsk(t, now)
				if dt.Symbol == "" {
					continue
				}
				if _, ok := allowed[dt.Symbol]; !ok {
					continue
				}
				merged := applyBidAskUpdate(state, dt)
				select {
				case out <- *merged:
				case <-ctx.Done():
					return
				}
			}
		}
	}
}

// FundingRates periodically polls Binance for perpetual funding rates and
// emits each rate to the returned channel.
func (c *Client) FundingRates(ctx context.Context) (<-chan domain.FundingRate, error) {
	ch := make(chan domain.FundingRate, 256)

	go func() {
		defer close(ch)

		ticker := time.NewTicker(c.fundingPollInterval)
		defer ticker.Stop()

		c.pollFundingRates(ctx, ch)

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				c.pollFundingRates(ctx, ch)
			}
		}
	}()

	return ch, nil
}

func (c *Client) pollFundingRates(ctx context.Context, ch chan<- domain.FundingRate) {
	backoff := ccxtutil.InitialBackoff
	for {
		if ctx.Err() != nil {
			return
		}

		rates, err := c.api.FetchFundingRates()
		if err != nil {
			if ccxtutil.IsRetryable(err) {
				c.logger.Warn("fetch funding rates error, retrying",
					"error", err,
					"backoff", backoff,
				)
				select {
				case <-ctx.Done():
					return
				case <-time.After(backoff):
				}
				backoff = ccxtutil.NextBackoff(backoff, maxBackoff)
				continue
			}
			c.logger.Error("fetch funding rates fatal error", "error", err)
			return
		}

		intervals, err := c.api.FetchFundingIntervals()
		if err != nil {
			c.logger.Error("fetch funding intervals error", "error", err)
		} else {
			c.logger.Debug("fetched funding intervals", "count", len(intervals.FundingRates))
		}

		now := time.Now().UTC()
		for _, fr := range rates.FundingRates {
			// set Interval from intervals response if available, since Binance's funding rate API doesn't include it
			if i, ok := intervals.FundingRates[*fr.Symbol]; fr.Interval == nil && ok {
				if (i.Interval != nil) && (*i.Interval != "") {
					fr.Interval = i.Interval
				}
			}

			dfr := mapFundingRate(fr, now)
			if dfr.Symbol == "" {
				continue
			}
			select {
			case ch <- dfr:
			case <-ctx.Done():
				return
			}
		}
		return
	}
}
