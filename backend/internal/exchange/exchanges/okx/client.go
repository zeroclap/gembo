// Package okx implements the exchange.Exchange interface for OKX via CCXT.
package okx

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"time"

	ccxt "github.com/ccxt/ccxt/go/v4"
	ccxtpro "github.com/ccxt/ccxt/go/v4/pro"
	"github.com/zeroclap/gembo/internal/domain"
)

const (
	exchangeID         = "okx"
	fundingPollDefault = 60 * time.Second
	marketsPollDefault = 60 * time.Second
	maxBackoff         = 60 * time.Second
	initialBackoff     = 1 * time.Second
)

// Client is the OKX exchange adapter.
type Client struct {
	rest   *ccxt.Okx
	ws     *ccxtpro.Okx
	logger *slog.Logger

	fundingPollInterval time.Duration
	marketsPollInterval time.Duration
}

// New creates a new OKX client.
func New(logger *slog.Logger, opts ...Option) *Client {
	c := &Client{
		logger:              logger.With("exchange", exchangeID),
		fundingPollInterval: fundingPollDefault,
		marketsPollInterval: marketsPollDefault,
	}
	for _, o := range opts {
		o(c)
	}

	restCfg := map[string]interface{}{"enableRateLimit": true}
	wsCfg := map[string]interface{}{"enableRateLimit": true}

	c.rest = ccxt.NewOkx(restCfg)
	c.ws = ccxtpro.NewOkx(wsCfg)
	return c
}

// ID satisfies exchange.Exchange.
func (c *Client) ID() string { return exchangeID }

// Run blocks until ctx is cancelled.
func (c *Client) Run(ctx context.Context) error {
	<-ctx.Done()
	
	// Close WebSocket with timeout to prevent hanging
	done := make(chan struct{})
	go func() {
		c.ws.Close()
		close(done)
	}()
	
	select {
	case <-done:
		c.logger.Info("websocket closed cleanly", "exchange", "okx")
	case <-time.After(3 * time.Second):
		c.logger.Warn("websocket close timeout, forcing shutdown", "exchange", "okx")
	}
	
	return ctx.Err()
}

// Markets fetches all available markets via REST.
func (c *Client) Markets(ctx context.Context) ([]domain.Market, error) {
	raw, err := c.rest.FetchMarkets()
	if err != nil {
		return nil, fmt.Errorf("okx fetch markets: %w", err)
	}

	markets := make([]domain.Market, 0, len(raw))
	now := time.Now().UTC()
	for _, m := range raw {
		dm := mapMarket(m, now)
		markets = append(markets, dm)
	}
	return markets, nil
}

// Tickers starts a WebSocket subscription and returns a channel of domain.Ticker.
func (c *Client) Tickers(ctx context.Context, symbols []string) (<-chan domain.Ticker, error) {
	if len(symbols) == 0 {
		return nil, fmt.Errorf("okx: no symbols provided for ticker subscription")
	}

	c.logger.Info("subscribing to tickers", "exchange", "okx", "symbols", len(symbols))

	ch := make(chan domain.Ticker, 256)

	go func() {
		defer close(ch)
		backoff := initialBackoff

		for {
			if ctx.Err() != nil {
				return
			}

			type result struct {
				tickers ccxt.Tickers
				err     error
			}
			resCh := make(chan result, 1)

			go func() {
				t, e := c.ws.WatchTickers(ccxt.WithWatchTickersSymbols(symbols))
				resCh <- result{tickers: t, err: e}
			}()

			var tickers ccxt.Tickers
			var err error

			select {
			case <-ctx.Done():
				c.logger.Info("ticker subscription cancelled", "exchange", "okx")
				return
			case res := <-resCh:
				tickers = res.tickers
				err = res.err
			}

			if err != nil {
				if ctx.Err() != nil {
					return
				}
				if isRetryable(err) {
					c.logger.Warn("watch tickers error, retrying",
						"error", err,
						"backoff", backoff,
					)
					select {
					case <-ctx.Done():
						return
					case <-time.After(backoff):
					}
					backoff = nextBackoff(backoff)
					continue
				}
				c.logger.Error("watch tickers fatal error", "error", err)
				return
			}

			backoff = initialBackoff
			now := time.Now().UTC()

			// c.logger.Info("received tickers update", "count", len(tickers.Tickers))

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
	}()

	return ch, nil
}

// FundingRates periodically polls OKX for perpetual funding rates.
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
	backoff := initialBackoff
	for {
		if ctx.Err() != nil {
			return
		}

		rates, err := c.rest.FetchFundingRates()
		if err != nil {
			if isRetryable(err) {
				c.logger.Warn("fetch funding rates error, retrying",
					"error", err,
					"backoff", backoff,
				)
				select {
				case <-ctx.Done():
					return
				case <-time.After(backoff):
				}
				backoff = nextBackoff(backoff)
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

// isRetryable returns true for transient CCXT errors.
func isRetryable(err error) bool {
	ccxtErr, ok := err.(*ccxt.Error)
	if !ok {
		return true
	}
	switch ccxtErr.Type {
	case "NetworkError", "RateLimitExceeded", "ExchangeNotAvailable",
		"DDoSProtection", "RequestTimeout", "OnMaintenance":
		return true
	}
	return false
}

// nextBackoff returns the next exponential backoff, capped at maxBackoff.
func nextBackoff(d time.Duration) time.Duration {
	next := time.Duration(float64(d) * math.Phi)
	if next > maxBackoff {
		return maxBackoff
	}
	return next
}
