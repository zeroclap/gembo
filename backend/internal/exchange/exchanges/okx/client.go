// Package okx implements the exchange.Exchange interface for OKX via CCXT.
package okx

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
	exchangeID         = "okx"
	fundingPollDefault = 60 * time.Second
	marketsPollDefault = 60 * time.Second
	maxBackoff         = 60 * time.Second
)

// Client is the OKX exchange adapter.
type Client struct {
	api    *ccxtpro.Okx
	logger *slog.Logger

	fundingPollInterval time.Duration
	marketsPollInterval time.Duration

	// Credentials for authenticated trading methods.
	apiKey     string
	secret     string
	passphrase string
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

	apiCfg := map[string]interface{}{
		"options": map[string]any{
			"defaultType": "swap",
			"fetchMarkets": map[string]any{
				"types": []string{"swap"},
				// "types": []string{"spot", "swap"},
			},
		},
	}
	if c.apiKey != "" {
		apiCfg["apiKey"] = c.apiKey
	}
	if c.secret != "" {
		apiCfg["secret"] = c.secret
	}
	if c.passphrase != "" {
		apiCfg["password"] = c.passphrase
	}
	c.api = ccxtpro.NewOkx(apiCfg)

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
		c.api.Close()
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
	raw, err := c.api.FetchMarkets()
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

// Tickers starts WebSocket subscriptions and returns a channel of domain.Ticker.
// Each symbol is subscribed individually to avoid the goroutine leak in CCXT's
// FutureRace mechanism. All subscriptions share the same WebSocket connection.
func (c *Client) Tickers(ctx context.Context, symbols []string) (<-chan domain.Ticker, error) {
	if len(symbols) == 0 {
		return nil, fmt.Errorf("okx: no symbols provided for ticker subscription")
	}

	c.logger.Info("subscribing to tickers", "exchange", "okx", "symbols", len(symbols))

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
