package binance

import (
	"context"
	"fmt"
	"strings"
	"time"

	ccxt "github.com/ccxt/ccxt/go/v4"
	"github.com/shopspring/decimal"

	"github.com/zeroclap/gembo/internal/domain"
	"github.com/zeroclap/gembo/internal/exchange"
	"github.com/zeroclap/gembo/internal/exchange/ccxtutil"
)

// isPositionAlreadyClosed checks whether a CCXT error from Binance indicates
// that a reduce-only order was rejected because the position is already at zero.
func isPositionAlreadyClosed(err error) bool {
	ccxtErr, ok := err.(*ccxt.Error)
	if !ok {
		return false
	}
	return ccxtErr.Type == "InvalidOrder"
}

func isAuthOrPermissionError(err error) bool {
	ccxtErr, ok := err.(*ccxt.Error)
	if !ok {
		return false
	}
	if ccxtErr.Type == "AuthenticationError" {
		return true
	}
	return strings.Contains(ccxtErr.Message, "\"code\":-2015")
}

// CreateMarketOrder places a market order on Binance.
func (c *Client) CreateMarketOrder(ctx context.Context, symbol string, side domain.OrderSide, amount float64) (*domain.ExchangeOrder, error) {
	order, err := c.api.CreateOrder(symbol, "market", string(side), amount)
	if err != nil {
		return nil, fmt.Errorf("binance create market order %s %s: %w", side, symbol, err)
	}

	return ccxtutil.MapOrder(order, exchangeID), nil
}

// CloseMarketOrder places a reduce-only market order on Binance.
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
			return nil, fmt.Errorf("binance close market order %s %s: %w: %w", side, symbol, exchange.ErrPositionAlreadyClosed, err)
		}
		return nil, fmt.Errorf("binance close market order %s %s: %w", side, symbol, err)
	}

	return ccxtutil.MapOrder(order, exchangeID), nil
}

// FetchOrder retrieves an order by its exchange-assigned ID.
func (c *Client) FetchOrder(ctx context.Context, orderID, symbol string) (*domain.ExchangeOrder, error) {
	order, err := c.api.FetchOrder(orderID, ccxt.WithFetchOrderSymbol(symbol))
	if err != nil {
		return nil, fmt.Errorf("binance fetch order %s: %w", orderID, err)
	}
	return ccxtutil.MapOrder(order, exchangeID), nil
}

// FetchPosition returns the current position for a symbol, or nil if none is open.
func (c *Client) FetchPosition(ctx context.Context, symbol string) (*exchange.ExchangePosition, error) {
	pos, err := c.api.FetchPosition(symbol)
	if err != nil {
		return nil, fmt.Errorf("binance fetch position %s: %w", symbol, err)
	}
	ep := ccxtutil.MapPosition(pos)
	if ep.Contracts.IsZero() {
		return nil, nil
	}
	return ep, nil
}

// FetchFundingHistory returns exact funding payment entries for a symbol.
func (c *Client) FetchFundingHistory(ctx context.Context, symbol string, since time.Time, limit int64) ([]exchange.FundingHistoryEntry, error) {
	opts := []ccxt.FetchFundingHistoryOptions{
		ccxt.WithFetchFundingHistorySince(since.UTC().UnixMilli()),
		ccxt.WithFetchFundingHistoryLimit(limit),
	}
	if symbol != "" {
		opts = append(opts, ccxt.WithFetchFundingHistorySymbol(symbol))
	}

	rows, err := fetchFundingHistoryWithContext(ctx, func() ([]ccxt.FundingHistory, error) {
		return c.api.FetchFundingHistory(opts...)
	})
	if isAuthOrPermissionError(err) {
		c.logger.Debug("binance funding history unavailable due to auth/permissions", "error", err)
		return []exchange.FundingHistoryEntry{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("binance fetch funding history: %w", err)
	}

	out := make([]exchange.FundingHistoryEntry, 0, len(rows))
	for _, row := range rows {
		if row.Amount == nil || row.Timestamp == nil {
			continue
		}
		out = append(out, exchange.FundingHistoryEntry{
			ID:        ccxtutil.DerefString(row.Id),
			Symbol:    ccxtutil.DerefString(row.Symbol),
			Amount:    decimal.NewFromFloat(*row.Amount),
			Currency:  ccxtutil.DerefString(row.Currency),
			Timestamp: time.UnixMilli(*row.Timestamp).UTC(),
		})
	}
	return out, nil
}

func fetchFundingHistoryWithContext(
	ctx context.Context,
	fn func() ([]ccxt.FundingHistory, error),
) (_ []ccxt.FundingHistory, returnedErr error) {
	type result struct {
		rows []ccxt.FundingHistory
		err  error
	}
	ch := make(chan result, 1)

	go func() {
		rows, err := fn()
		ch <- result{rows: rows, err: err}
	}()

	select {
	case res := <-ch:
		return res.rows, res.err
	case <-ctx.Done():
		go func() { <-ch }()
		return nil, fmt.Errorf("fetch funding history cancelled: %w", ctx.Err())
	}
}

// FetchBalance returns account balances.
func (c *Client) FetchBalance(ctx context.Context) ([]exchange.Balance, error) {
	balances, err := c.api.FetchBalance()
	if err != nil {
		return nil, fmt.Errorf("binance fetch balance: %w", err)
	}
	return ccxtutil.MapBalances(balances), nil
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
		return fmt.Errorf("binance set margin mode %s %s: %w", symbol, mode, err)
	}
	return nil
}

// HasCredentials returns true if API credentials are configured.
func (c *Client) HasCredentials() bool {
	return c.apiKey != "" && c.secret != ""
}

// Positions subscribes to real-time position updates via WatchPositions WebSocket.
// Returns a channel that emits []ExchangePosition on every update.
// Satisfies exchange.PositionWatcher.
func (c *Client) Positions(ctx context.Context) (<-chan []exchange.ExchangePosition, error) {
	if !c.HasCredentials() {
		return nil, fmt.Errorf("binance: credentials required for position watching")
	}

	c.logger.Info("subscribing to position updates")

	ch := make(chan []exchange.ExchangePosition, 256)

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
					c.logger.Warn("watch positions error, retrying",
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
				c.logger.Error("watch positions fatal error", "error", err)
				return
			}

			backoff = ccxtutil.InitialBackoff

			eps := make([]exchange.ExchangePosition, 0, len(positions))
			for _, p := range positions {
				c.logger.Debug("position update", "symbol", p.Symbol, "contracts", p.Contracts)
				eps = append(eps, *ccxtutil.MapPosition(p))
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
