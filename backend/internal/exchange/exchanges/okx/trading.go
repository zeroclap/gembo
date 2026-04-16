package okx

import (
	"context"
	"fmt"
	"strings"
	"time"

	ccxt "github.com/ccxt/ccxt/go/v4"

	"github.com/zeroclap/gembo/internal/domain"
	"github.com/zeroclap/gembo/internal/exchange"
	"github.com/zeroclap/gembo/internal/exchange/ccxtutil"
)

// isPositionAlreadyClosed checks whether a CCXT error from OKX indicates
// that a reduce-only order was rejected because the position is already at zero.
// OKX returns ExchangeError with sCode 51169 in this case.
func isPositionAlreadyClosed(err error) bool {
	ccxtErr, ok := err.(*ccxt.Error)
	if !ok {
		return false
	}
	return ccxtErr.Type == "ExchangeError" && strings.Contains(ccxtErr.Message, "51169")
}

// CreateMarketOrder places a market order on OKX.
func (c *Client) CreateMarketOrder(ctx context.Context, symbol string, side domain.OrderSide, amount float64) (*domain.ExchangeOrder, error) {
	order, err := c.api.CreateOrder(symbol, "market", string(side), amount)
	if err != nil {
		return nil, fmt.Errorf("okx create market order %s %s: %w", side, symbol, err)
	}
	return ccxtutil.MapOrder(order, exchangeID), nil
}

// CloseMarketOrder places a reduce-only market order on OKX.
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
			return nil, fmt.Errorf("okx close market order %s %s: %w: %w", side, symbol, exchange.ErrPositionAlreadyClosed, err)
		}
		return nil, fmt.Errorf("okx close market order %s %s: %w", side, symbol, err)
	}
	return ccxtutil.MapOrder(order, exchangeID), nil
}

// FetchOrder retrieves an order by its exchange-assigned ID.
func (c *Client) FetchOrder(ctx context.Context, orderID, symbol string) (*domain.ExchangeOrder, error) {
	order, err := c.api.FetchOrder(orderID, ccxt.WithFetchOrderSymbol(symbol))
	if err != nil {
		return nil, fmt.Errorf("okx fetch order %s: %w", orderID, err)
	}
	return ccxtutil.MapOrder(order, exchangeID), nil
}

// FetchPosition returns the current position for a symbol, or nil if none is open.
func (c *Client) FetchPosition(ctx context.Context, symbol string) (*exchange.ExchangePosition, error) {
	pos, err := c.api.FetchPosition(symbol)
	if err != nil {
		return nil, fmt.Errorf("okx fetch position %s: %w", symbol, err)
	}
	ep := ccxtutil.MapPosition(pos)
	if ep.Contracts.IsZero() {
		return nil, nil
	}
	return ep, nil
}

// FetchBalance returns account balances.
func (c *Client) FetchBalance(ctx context.Context) ([]exchange.Balance, error) {
	balances, err := c.api.FetchBalance()
	if err != nil {
		return nil, fmt.Errorf("okx fetch balance: %w", err)
	}
	return ccxtutil.MapBalances(balances), nil
}

// SetMarginMode sets the margin mode ("cross" or "isolated") for a symbol.
func (c *Client) SetMarginMode(ctx context.Context, symbol string, mode string, leverage int) error {
	_, err := c.api.SetMarginMode(
		mode,
		ccxt.WithSetMarginModeSymbol(symbol),
		ccxt.WithSetMarginModeParams(map[string]any{
			"leverage": leverage,
		}),
	)
	if err != nil {
		return fmt.Errorf("okx set margin mode %s %s: %w", symbol, mode, err)
	}
	return nil
}

// HasCredentials returns true if API credentials are configured.
func (c *Client) HasCredentials() bool {
	return c.apiKey != "" && c.secret != "" && c.passphrase != ""
}

// Positions subscribes to real-time position updates via WatchPositions WebSocket.
// Returns a channel that emits []ExchangePosition on every update.
// Satisfies exchange.PositionWatcher.
func (c *Client) Positions(ctx context.Context) (<-chan []exchange.ExchangePosition, error) {
	if !c.HasCredentials() {
		return nil, fmt.Errorf("okx: credentials required for position watching")
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
