package trader

import (
	"context"
	"time"

	"github.com/zeroclap/gembo/internal/domain"
)

// runMonitor periodically polls exchange position state for all open positions
// and updates their mark price and unrealized P&L in PostgreSQL.
func (t *Trader) runMonitor(ctx context.Context) error {
	ticker := time.NewTicker(t.monitorInterval())
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			t.monitorOnce(ctx)
		}
	}
}

// monitorOnce fetches state for all open/partially_open positions and updates PG.
func (t *Trader) monitorOnce(ctx context.Context) {
	positions, err := t.store.ListPositions(ctx, domain.PositionFilter{
		Status: []domain.PositionStatus{
			domain.PositionStatusOpen,
			domain.PositionStatusPartiallyOpen,
		},
	})
	if err != nil {
		t.logger.Warn("monitor: list open positions", "error", err)
		return
	}

	if len(positions) == 0 {
		return
	}

	t.logger.Debug("monitoring positions", "count", len(positions))

	for i := range positions {
		p := &positions[i]
		if err := t.updatePositionState(ctx, p); err != nil {
			t.logger.Warn("monitor: update position state",
				"position_id", p.ID, "symbol", p.Symbol, "error", err)
		}
	}
}

// updatePositionState fetches position data from both exchanges and updates
// the position's mark prices, unrealized P&L, and funding totals.
func (t *Trader) updatePositionState(ctx context.Context, p *domain.Position) error {
	longExch, err := t.registry.Get(p.LongExchangeID)
	if err != nil {
		return err
	}
	shortExch, err := t.registry.Get(p.ShortExchangeID)
	if err != nil {
		return err
	}

	updated := false

	// Fetch long leg state.
	longPos, err := longExch.FetchPosition(ctx, p.Symbol)
	if err != nil {
		t.logger.Debug("monitor: fetch long position",
			"exchange", p.LongExchangeID, "symbol", p.Symbol, "error", err)
	} else if longPos != nil {
		p.LongMarkPrice = longPos.MarkPrice
		p.LongUnrealizedPnL = longPos.UnrealizedPnL
		updated = true
	}

	// Fetch short leg state.
	shortPos, err := shortExch.FetchPosition(ctx, p.Symbol)
	if err != nil {
		t.logger.Debug("monitor: fetch short position",
			"exchange", p.ShortExchangeID, "symbol", p.Symbol, "error", err)
	} else if shortPos != nil {
		p.ShortMarkPrice = shortPos.MarkPrice
		p.ShortUnrealizedPnL = shortPos.UnrealizedPnL
		updated = true
	}

	// Update funding totals.
	fundingPnL, err := t.store.SumFundingPnL(ctx, p.ID)
	if err != nil {
		t.logger.Debug("monitor: sum funding pnl", "position_id", p.ID, "error", err)
	} else if !fundingPnL.Equal(p.TotalFundingPnL) {
		p.TotalFundingPnL = fundingPnL
		updated = true
	}

	if updated {
		if err := t.store.UpdatePosition(ctx, p); err != nil {
			return err
		}
	}

	return nil
}
