// Package htx implements the exchange.Exchange interface for HTX (formerly Huobi) via CCXT.
package htx

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
	exchangeID         = "htx"
	fundingPollDefault = 60 * time.Second
	marketsPollDefault = 60 * time.Second
	maxBackoff         = 60 * time.Second
)

// Client is the HTX exchange adapter.
type Client struct {
	api    *ccxtpro.Htx
	logger *slog.Logger

	fundingPollInterval time.Duration
	marketsPollInterval time.Duration
}

// New creates a new HTX client.
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
			"defaultType":    "swap",
			"defaultSubType": "linear",
		},
	}
	c.api = ccxtpro.NewHtx(apiCfg)

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

// Markets fetches all available swap markets via REST and returns them as domain.Market.
func (c *Client) Markets(ctx context.Context) ([]domain.Market, error) {
	raw, err := c.api.FetchMarkets()
	if err != nil {
		return nil, fmt.Errorf("htx fetch markets: %w", err)
	}

	markets := make([]domain.Market, 0, len(raw))
	now := time.Now().UTC()
	for _, m := range raw {
		dm := mapMarket(m, now)
		markets = append(markets, dm)
	}
	return markets, nil
}

// Tickers subscribes to per-symbol WatchTicker WebSocket streams. HTX's
// WatchTicker (singular) includes bid/ask data, so no second stream is needed.
//
// Each symbol runs its own WatchTicker retry loop in a separate goroutine.
// All share the same underlying WebSocket connection.
//
// The returned channel is closed when ctx is cancelled.
func (c *Client) Tickers(ctx context.Context, symbols []string) (<-chan domain.Ticker, error) {
	if len(symbols) == 0 {
		return nil, fmt.Errorf("htx: no symbols provided for ticker subscription")
	}

	c.logger.Info("subscribing to tickers", "symbols", len(symbols))

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

func (c *Client) watchTickerSymbol(ctx context.Context, symbol string, out chan<- domain.Ticker) {
	backoff := ccxtutil.InitialBackoff

	for {
		if ctx.Err() != nil {
			return
		}

		ticker, err := ccxtutil.WatchWithContext(ctx, func() (ccxt.Ticker, error) {
			return c.api.WatchTicker(symbol)
		})
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			if ccxtutil.IsRetryable(err) {
				c.logger.Warn("watch ticker error, retrying",
					"symbol", symbol,
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
			c.logger.Error("watch ticker fatal error", "symbol", symbol, "error", err)
			return
		}

		backoff = ccxtutil.InitialBackoff

		dt := mapTicker(ticker, time.Now().UTC())
		if dt.Symbol == "" {
			continue
		}

		select {
		case out <- dt:
		case <-ctx.Done():
			return
		}
	}
}

// FundingRates periodically polls HTX for perpetual funding rates and
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
