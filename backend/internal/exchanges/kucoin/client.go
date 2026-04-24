// Package kucoin implements the exchange.Exchange interface for KuCoin Futures via CCXT.
package kucoin

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
	exchangeID         = "kucoinfutures"
	fundingPollDefault = 60 * time.Second
	marketsPollDefault = 60 * time.Second
	tickersPollDefault = 10 * time.Second
	maxBackoff         = 60 * time.Second
	fetchWorkers       = 10 // concurrent FetchTicker workers
)

// Client is the KuCoin Futures exchange adapter.
type Client struct {
	api    *ccxtpro.Kucoinfutures
	logger *slog.Logger

	fundingPollInterval time.Duration
	marketsPollInterval time.Duration
	tickersPollInterval time.Duration
}

// New creates a new KuCoin Futures client.
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
		},
	}
	c.api = ccxtpro.NewKucoinfutures(apiCfg)

	return c
}

// ID satisfies exchange.Exchange.
func (c *Client) ID() string { return exchangeID }

// Run blocks until ctx is cancelled.
func (c *Client) Run(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

// Markets fetches all available swap markets via REST and returns them as domain.Market.
func (c *Client) Markets(ctx context.Context) ([]domain.Market, error) {
	raw, err := c.api.FetchMarkets()
	if err != nil {
		return nil, fmt.Errorf("kucoinfutures fetch markets: %w", err)
	}

	markets := make([]domain.Market, 0, len(raw))
	now := time.Now().UTC()
	for _, m := range raw {
		dm := mapMarket(m, now)
		markets = append(markets, dm)
	}
	return markets, nil
}

// Tickers polls KuCoin Futures via REST (FetchTickers) on a fixed interval and
// emits each ticker to the returned channel.
//
// KuCoin Futures' CCXT WebSocket implementation spawns background goroutines that
// panic on network errors in ways that cannot be recovered externally (the panics
// occur in goroutines we don't own). REST polling avoids this instability entirely.
//
// The channel is closed when ctx is cancelled.
func (c *Client) Tickers(ctx context.Context, symbols []string) (<-chan domain.Ticker, error) {
	if len(symbols) == 0 {
		return nil, fmt.Errorf("kucoinfutures: no symbols provided for ticker subscription")
	}

	c.logger.Info("starting ticker polling", "symbols", len(symbols), "interval", c.tickersPollInterval)

	ch := make(chan domain.Ticker, 256)

	go func() {
		defer close(ch)

		ticker := time.NewTicker(c.tickersPollInterval)
		defer ticker.Stop()

		c.pollTickers(ctx, symbols, ch)

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				c.pollTickers(ctx, symbols, ch)
			}
		}
	}()

	return ch, nil
}

// fetchTickerRecovered calls FetchTicker for a single symbol and recovers any internal CCXT panic.
func (c *Client) fetchTickerRecovered(symbol string) (result ccxt.Ticker, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("kucoinfutures FetchTicker panic: %v", r)
		}
	}()
	return c.api.FetchTicker(symbol)
}

// fetchFundingRatesRecovered calls FetchFundingRates and recovers any internal CCXT panic.
func (c *Client) fetchFundingRatesRecovered() (result ccxt.FundingRates, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("kucoinfutures FetchFundingRates panic: %v", r)
		}
	}()
	return c.api.FetchFundingRates()
}

// pollTickers fetches real bid/ask for each symbol by calling FetchTicker concurrently
// via a bounded worker pool. KuCoin Futures' bulk FetchTickers endpoint does not return
// bid/ask prices, so per-symbol REST calls are required.
func (c *Client) pollTickers(ctx context.Context, symbols []string, ch chan<- domain.Ticker) {
	if ctx.Err() != nil {
		return
	}

	type result struct {
		ticker ccxt.Ticker
		err    error
	}

	jobs := make(chan string, len(symbols))
	results := make(chan result, len(symbols))

	var wg sync.WaitGroup
	for i := 0; i < fetchWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for sym := range jobs {
				if ctx.Err() != nil {
					return
				}
				t, err := c.fetchTickerRecovered(sym)
				results <- result{ticker: t, err: err}
			}
		}()
	}

	for _, sym := range symbols {
		jobs <- sym
	}
	close(jobs)

	go func() {
		wg.Wait()
		close(results)
	}()

	now := time.Now().UTC()
	for r := range results {
		if r.err != nil {
			if ccxtutil.IsRetryable(r.err) {
				c.logger.Warn("fetch ticker error", "error", r.err)
			} else {
				c.logger.Error("fetch ticker fatal error", "error", r.err)
			}
			continue
		}
		dt := mapTicker(r.ticker, now)
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

// FundingRates periodically polls KuCoin Futures for perpetual funding rates and
// emits each rate to the returned channel. The channel is closed when ctx is cancelled.
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

		rates, err := c.fetchFundingRatesRecovered()
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
			c.logger.Info("fetch funding rates not supported, skipping", "error", err)
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
