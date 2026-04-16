// Package collector orchestrates the pipeline from exchanges to the store and event emitter.
// It provides two independent collectors:
//   - PublicCollector: markets, tickers, funding rates (no API keys required)
//   - PrivateCollector: positions, orders, balances (API keys required)
package collector

import (
	"context"
	"encoding/json"
	"time"

	"github.com/zeroclap/gembo/internal/domain"
	"github.com/zeroclap/gembo/internal/hook"
)

const marketsPollInterval = 60 * time.Second

// --- event emission helpers (used by PublicCollector) ---

func (c *PublicCollector) emitMarket(ctx context.Context, m domain.Market) {
	payload, err := json.Marshal(m)
	if err != nil {
		c.logger.Error("marshal market for event", "error", err)
		return
	}
	ev := hook.Event{
		Type:     hook.EventMarketUpdated,
		Exchange: m.ExchangeID,
		Symbol:   m.Symbol,
		Payload:  payload,
	}
	if err := c.emitter.Emit(ctx, ev); err != nil {
		c.logger.Warn("emit market event error", "exchange", m.ExchangeID, "symbol", m.Symbol, "error", err)
	}
}

func (c *PublicCollector) emitTicker(ctx context.Context, t domain.Ticker) {
	payload, err := json.Marshal(t)
	if err != nil {
		c.logger.Error("marshal ticker for event", "error", err)
		return
	}
	ev := hook.Event{
		Type:     hook.EventTickerUpdated,
		Exchange: t.ExchangeID,
		Symbol:   t.Symbol,
		Payload:  payload,
	}
	if err := c.emitter.Emit(ctx, ev); err != nil {
		c.logger.Warn("emit ticker event error", "exchange", t.ExchangeID, "symbol", t.Symbol, "error", err)
	}
}

func (c *PublicCollector) emitFundingRate(ctx context.Context, fr domain.FundingRate) {
	payload, err := json.Marshal(fr)
	if err != nil {
		c.logger.Error("marshal funding rate for event", "error", err)
		return
	}
	ev := hook.Event{
		Type:     hook.EventFundingUpdated,
		Exchange: fr.ExchangeID,
		Symbol:   fr.Symbol,
		Payload:  payload,
	}
	if err := c.emitter.Emit(ctx, ev); err != nil {
		c.logger.Warn("emit funding rate event error", "exchange", fr.ExchangeID, "symbol", fr.Symbol, "error", err)
	}
}
