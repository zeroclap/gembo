// Package aster implements the exchange.Exchange interface for Aster via CCXT.
package aster

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	ccxt "github.com/ccxt/ccxt/go/v4"
	ccxtpro "github.com/ccxt/ccxt/go/v4/pro"

	"github.com/zeroclap/gembo/internal/domain"
	"github.com/zeroclap/gembo/internal/exchanges/base/ccxtutil"
)

const (
	exchangeID         = "aster"
	fundingPollDefault = 60 * time.Second
	marketsPollDefault = 60 * time.Second
	tickersPollDefault = 10 * time.Second
	maxBackoff         = 60 * time.Second
)

// Client is the Aster exchange adapter.
type Client struct {
	api    *ccxtpro.Aster
	logger *slog.Logger

	fundingPollInterval time.Duration
	marketsPollInterval time.Duration
	tickersPollInterval time.Duration
}

// New creates a new Aster client.
func New(logger *slog.Logger, opts ...Option) *Client {
	c := &Client{
		logger:              logger.With("exchange", exchangeID),
		fundingPollInterval: fundingPollDefault,
		marketsPollInterval: marketsPollDefault,
		tickersPollInterval: tickersPollDefault,
	}
	for _, o := range opts {
		o(c)
	}

	apiCfg := map[string]interface{}{
		"options": map[string]any{
			"defaultType": "swap",
			"fetchMarkets": map[string]any{
				"types": []string{"swap"},
			},
		},
	}
	c.api = ccxtpro.NewAster(apiCfg)

	return c
}

// ID satisfies exchange.Exchange.
func (c *Client) ID() string { return exchangeID }

// Run blocks until ctx is cancelled.
// Aster uses REST polling so there are no WebSocket connections to close.
func (c *Client) Run(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

// Markets fetches all available markets via REST and returns them as domain.Market.
func (c *Client) Markets(ctx context.Context) ([]domain.Market, error) {
	raw, err := c.api.FetchMarkets()
	if err != nil {
		return nil, fmt.Errorf("aster fetch markets: %w", err)
	}

	markets := make([]domain.Market, 0, len(raw))
	now := time.Now().UTC()
	for _, m := range raw {
		dm := mapMarket(m, now)
		markets = append(markets, dm)
	}
	return markets, nil
}

// Tickers polls Aster via bulk REST FetchTickers on a fixed interval and emits
// each ticker to the returned channel.
//
// Aster's CCXT WebSocket WatchTickers uses a shared subscribeHash ("swap") across
// all symbols, so calling it per-symbol causes only the first symbol to be actually
// subscribed on the WS — all others silently wait forever. REST polling avoids this.
//
// The channel is closed when ctx is cancelled.
func (c *Client) Tickers(ctx context.Context, symbols []string) (<-chan domain.Ticker, error) {
	if len(symbols) == 0 {
		return nil, fmt.Errorf("aster: no symbols provided for ticker polling")
	}

	c.logger.Info("starting ticker polling", "symbols", len(symbols), "interval", c.tickersPollInterval)

	ch := make(chan domain.Ticker, 256)

	go func() {
		defer close(ch)

		ticker := time.NewTicker(c.tickersPollInterval)
		defer ticker.Stop()

		c.pollTickers(ctx, ch)

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				c.pollTickers(ctx, ch)
			}
		}
	}()

	return ch, nil
}

func (c *Client) fetchTickersRecovered() (result ccxt.Tickers, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("aster FetchTickers panic: %v", r)
		}
	}()
	return c.api.FetchTickers()
}

func (c *Client) pollTickers(ctx context.Context, ch chan<- domain.Ticker) {
	if ctx.Err() != nil {
		return
	}

	tickers, err := c.fetchTickersRecovered()
	if err != nil {
		if ccxtutil.IsRetryable(err) {
			c.logger.Warn("fetch tickers error", "error", err)
		} else {
			c.logger.Error("fetch tickers fatal error", "error", err)
		}
		return
	}

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

// FundingRates periodically polls Aster for perpetual funding rates and
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
