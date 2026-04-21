package bybit

import (
	"context"
	"fmt"
	"time"

	ccxt "github.com/ccxt/ccxt/go/v4"
	"github.com/shopspring/decimal"

	"github.com/zeroclap/gembo/internal/domain"
	"github.com/zeroclap/gembo/internal/exchanges/base"
	"github.com/zeroclap/gembo/internal/exchanges/base/ccxtutil"
)

const (
	balancePollInterval        = 15 * time.Second
	fundingHistoryPollInterval = 2 * time.Minute
	fundingHistoryFetchLimit   = int64(200)
)

// isPositionAlreadyClosed checks whether a CCXT error from Bybit indicates
// that a reduce-only order was rejected because the position is already at zero.
func isPositionAlreadyClosed(err error) bool {
	ccxtErr, ok := err.(*ccxt.Error)
	if !ok {
		return false
	}
	return ccxtErr.Type == "InvalidOrder"
}

// HasCredentials returns true if API credentials are configured.
func (c *Client) HasCredentials() bool {
	return c.apiKey != "" && c.secret != ""
}

// CreateMarketOrder places a market order on Bybit.
func (c *Client) CreateMarketOrder(ctx context.Context, symbol string, side domain.OrderSide, amount float64) (*domain.ExchangeOrder, error) {
	order, err := c.api.CreateOrder(symbol, "market", string(side), amount)
	if err != nil {
		return nil, fmt.Errorf("bybit create market order %s %s: %w", side, symbol, err)
	}
	return ccxtutil.MapOrder(order, exchangeID), nil
}

// CloseMarketOrder places a reduce-only market order on Bybit.
// Returns ErrPositionAlreadyClosed when the position is already at zero.
func (c *Client) CloseMarketOrder(ctx context.Context, symbol string, side domain.OrderSide, amount float64) (*domain.ExchangeOrder, error) {
	order, err := c.api.CreateOrder(
		symbol, "market", string(side), amount,
		ccxt.WithCreateOrderParams(map[string]interface{}{
			"reduceOnly": true,
		}),
	)
	if err != nil {
		if isPositionAlreadyClosed(err) {
			return nil, fmt.Errorf("bybit close market order %s %s: %w: %w", side, symbol, base.ErrPositionAlreadyClosed, err)
		}
		return nil, fmt.Errorf("bybit close market order %s %s: %w", side, symbol, err)
	}
	return ccxtutil.MapOrder(order, exchangeID), nil
}

// FetchOrder retrieves an order by its exchange-assigned ID.
func (c *Client) FetchOrder(ctx context.Context, orderID, symbol string) (*domain.ExchangeOrder, error) {
	order, err := c.api.FetchOrder(
		orderID,
		ccxt.WithFetchOrderSymbol(symbol),
		ccxt.WithFetchOrderParams(map[string]any{
			"acknowledged": true,
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("bybit fetch order %s: %w", orderID, err)
	}
	return ccxtutil.MapOrder(order, exchangeID), nil
}

// FetchPosition returns the current position for a symbol, or nil if none is open.
func (c *Client) FetchPosition(ctx context.Context, symbol string) (*base.ExchangePosition, error) {
	pos, err := c.api.FetchPosition(symbol)
	if err != nil {
		return nil, fmt.Errorf("bybit fetch position %s: %w", symbol, err)
	}
	ep := base.MapPosition(pos)
	if ep.Contracts.IsZero() {
		return nil, nil
	}
	return ep, nil
}

// FetchBalance returns account balances (single REST call, for on-demand use).
func (c *Client) FetchBalance(ctx context.Context) ([]base.Balance, error) {
	balances, err := c.api.FetchBalance()
	if err != nil {
		return nil, fmt.Errorf("bybit fetch balance: %w", err)
	}
	return base.MapBalances(balances), nil
}

// SetMarginMode sets the margin mode ("cross" or "isolated") for a symbol.
func (c *Client) SetMarginMode(ctx context.Context, symbol string, mode string, leverage int) error {
	opts := []ccxt.SetMarginModeOptions{ccxt.WithSetMarginModeSymbol(symbol)}
	if leverage > 0 {
		opts = append(opts, ccxt.WithSetMarginModeParams(map[string]any{
			"leverage": leverage,
		}))
	}
	_, err := c.api.SetMarginMode(mode, opts...)
	if err != nil {
		return fmt.Errorf("bybit set margin mode %s %s: %w", symbol, mode, err)
	}
	return nil
}

// Positions subscribes to real-time position updates via WatchPositions WebSocket.
// Each emission is the full set of current positions on the exchange.
func (c *Client) Positions(ctx context.Context) (<-chan []base.ExchangePosition, error) {
	if !c.HasCredentials() {
		return nil, fmt.Errorf("bybit: credentials required for position watching")
	}

	c.logger.Info("subscribing to position updates")
	ch := make(chan []base.ExchangePosition, 256)

	go func() {
		defer close(ch)
		backoff := ccxtutil.InitialBackoff

		for {
			if ctx.Err() != nil {
				return
			}

			positions, err := ccxtutil.WatchWithContext(ctx, func() ([]ccxt.Position, error) {
				return c.api.WatchPositions()
			})
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				if ccxtutil.IsRetryable(err) {
					c.logger.Warn("watch positions error, retrying", "error", err, "backoff", backoff)
					select {
					case <-ctx.Done():
						return
					case <-time.After(backoff):
					}
					backoff = ccxtutil.NextBackoff(backoff, maxBackoff)
					continue
				}
				c.logger.Error("watch positions fatal error", "error", err)
				return
			}

			backoff = ccxtutil.InitialBackoff

			eps := make([]base.ExchangePosition, 0, len(positions))
			for _, p := range positions {
				c.logger.Debug("position update", "symbol", p.Symbol, "contracts", p.Contracts)
				eps = append(eps, *base.MapPosition(p))
			}

			select {
			case ch <- eps:
			case <-ctx.Done():
				return
			}
		}
	}()

	return ch, nil
}

// Balances emits periodic account balance snapshots. The polling interval is
// managed internally — callers simply read from the returned channel.
func (c *Client) Balances(ctx context.Context) (<-chan []base.Balance, error) {
	if !c.HasCredentials() {
		return nil, fmt.Errorf("bybit: credentials required for balance streaming")
	}

	ch := make(chan []base.Balance, 16)

	go func() {
		defer close(ch)

		c.fetchAndEmitBalances(ctx, ch)

		ticker := time.NewTicker(balancePollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				c.fetchAndEmitBalances(ctx, ch)
			}
		}
	}()

	return ch, nil
}

func (c *Client) fetchAndEmitBalances(ctx context.Context, ch chan<- []base.Balance) {
	balances, err := c.api.FetchBalance()
	if err != nil {
		c.logger.Warn("fetch balance error", "error", err)
		return
	}
	mapped := base.MapBalances(balances)
	select {
	case ch <- mapped:
	case <-ctx.Done():
	}
}

// FundingHistory emits periodic funding payment history snapshots.
// The polling interval is managed internally.
func (c *Client) FundingHistory(ctx context.Context) (<-chan []base.FundingHistoryEntry, error) {
	if !c.HasCredentials() {
		return nil, fmt.Errorf("bybit: credentials required for funding history")
	}

	ch := make(chan []base.FundingHistoryEntry, 16)

	go func() {
		defer close(ch)

		c.fetchAndEmitFundingHistory(ctx, ch)

		ticker := time.NewTicker(fundingHistoryPollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				c.fetchAndEmitFundingHistory(ctx, ch)
			}
		}
	}()

	return ch, nil
}

func (c *Client) fetchAndEmitFundingHistory(ctx context.Context, ch chan<- []base.FundingHistoryEntry) {
	since := time.Now().UTC().Add(-48 * time.Hour)
	opts := []ccxt.FetchFundingHistoryOptions{
		ccxt.WithFetchFundingHistorySince(since.UTC().UnixMilli()),
		ccxt.WithFetchFundingHistoryLimit(fundingHistoryFetchLimit),
	}

	rows, err := ccxtutil.WatchWithContext(ctx, func() ([]ccxt.FundingHistory, error) {
		return c.api.FetchFundingHistory(opts...)
	})
	if err != nil {
		if ctx.Err() == nil {
			c.logger.Warn("fetch funding history error", "error", err)
		}
		return
	}

	out := make([]base.FundingHistoryEntry, 0, len(rows))
	for _, row := range rows {
		if row.Amount == nil || row.Timestamp == nil {
			continue
		}
		out = append(out, base.FundingHistoryEntry{
			ID:        ccxtutil.DerefString(row.Id),
			Symbol:    ccxtutil.DerefString(row.Symbol),
			Amount:    decimal.NewFromFloat(*row.Amount),
			Currency:  ccxtutil.DerefString(row.Currency),
			Timestamp: time.UnixMilli(*row.Timestamp).UTC(),
		})
	}

	if len(out) == 0 {
		return
	}

	select {
	case ch <- out:
	case <-ctx.Done():
	}
}
