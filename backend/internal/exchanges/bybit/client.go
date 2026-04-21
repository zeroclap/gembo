// Package bybit implements the exchange.Exchange interface for Bybit via CCXT.
package bybit

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	ccxt "github.com/ccxt/ccxt/go/v4"
	ccxtpro "github.com/ccxt/ccxt/go/v4/pro"
	"github.com/zeroclap/gembo/internal/domain"
	"github.com/zeroclap/gembo/internal/exchanges/base/ccxtutil"
)

const (
	exchangeID         = "bybit"
	fundingPollDefault = 60 * time.Second
	marketsPollDefault = 60 * time.Second
	maxBackoff         = 60 * time.Second
)

// Client is the Bybit exchange adapter.
type Client struct {
	api    *ccxtpro.Bybit
	logger *slog.Logger

	fundingPollInterval time.Duration
	marketsPollInterval time.Duration

	// Credentials for authenticated trading methods.
	apiKey string
	secret string
}

// New creates a new Bybit client. opts is passed directly to the CCXT constructor.
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
				// "types": []string{"spot", "linear"},
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
	c.api = ccxtpro.NewBybit(apiCfg)

	return c
}

// ID satisfies exchange.Exchange.
func (c *Client) ID() string { return exchangeID }

// Run blocks until ctx is cancelled, keeping WS connections alive.
// The adapter does not use a long-lived Run goroutine; individual data
// loops are managed by Tickers/FundingRates callers. Run returns nil
// when ctx is cancelled.
func (c *Client) Run(ctx context.Context) error {
	<-ctx.Done()

	// Close WebSocket with timeout to prevent hanging
	done := make(chan struct{})
	go func() {
		c.api.Close()
		close(done)
	}()

	select {
	case <-done:
		c.logger.Info("websocket closed cleanly", "exchange", "bybit")
	case <-time.After(3 * time.Second):
		c.logger.Warn("websocket close timeout, forcing shutdown", "exchange", "bybit")
	}

	return ctx.Err()
}

// Markets fetches all available markets via REST and returns them as domain.Market.
func (c *Client) Markets(ctx context.Context) ([]domain.Market, error) {
	raw, err := c.api.FetchMarkets()
	if err != nil {
		return nil, fmt.Errorf("bybit fetch markets: %w", err)
	}

	markets := make([]domain.Market, 0, len(raw))
	now := time.Now().UTC()
	for _, m := range raw {
		dm := mapMarket(m, now)
		markets = append(markets, dm)
	}
	return markets, nil
}

// Tickers starts WebSocket subscriptions and returns a channel that emits
// domain.Ticker on every update. Each symbol is subscribed individually to
// avoid the goroutine leak in CCXT's FutureRace mechanism. All subscriptions
// share the same underlying WebSocket connection.
// The channel is closed when ctx is cancelled.
func (c *Client) Tickers(ctx context.Context, symbols []string) (<-chan domain.Ticker, error) {
	if len(symbols) == 0 {
		return nil, fmt.Errorf("bybit: no symbols provided for ticker subscription")
	}

	c.logger.Info("subscribing to tickers", "exchange", "bybit", "symbols", len(symbols))

	ch := make(chan domain.Ticker, 256)

	go func() {
		defer close(ch)
		var wg sync.WaitGroup
		for _, sym := range symbols {
			wg.Add(1)
			go func() {
				defer wg.Done()
				c.watchTickerSymbol(ctx, sym, ch)
			}()
		}
		wg.Wait()
	}()

	return ch, nil
}

func (c *Client) watchTickerSymbol(ctx context.Context, symbol string, ch chan<- domain.Ticker) {
	backoff := ccxtutil.InitialBackoff

	for {
		if ctx.Err() != nil {
			return
		}

		tickers, err := ccxtutil.WatchWithContext(ctx, func() (ccxt.Tickers, error) {
			return c.api.WatchTickers(ccxt.WithWatchTickersSymbols([]string{symbol}))
		})
		if err != nil {
			if ctx.Err() != nil {
				c.logger.Info("ticker subscription stopped", "reason", ctx.Err())
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
		now := time.Now().UTC()

		for _, t := range tickers.Tickers {
			dt := mapTicker(t, now)
			if dt.Symbol == "" {
				continue
			}
			select {
			case ch <- dt:
			case <-ctx.Done():
				return
			}
		}
	}
}

// FundingRates periodically polls Bybit for perpetual funding rates and
// emits each rate to the returned channel. The channel is closed when ctx
// is cancelled.
func (c *Client) FundingRates(ctx context.Context) (<-chan domain.FundingRate, error) {
	ch := make(chan domain.FundingRate, 256)

	go func() {
		defer close(ch)

		ticker := time.NewTicker(c.fundingPollInterval)
		defer ticker.Stop()

		// Poll immediately, then on interval.
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

		now := time.Now().UTC()
		for _, fr := range rates.FundingRates {
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
