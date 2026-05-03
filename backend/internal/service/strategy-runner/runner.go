// Package strategyrunner evaluates automated arbitrage strategies against live
// opportunity streams and opens positions on behalf of users when criteria are met.
package strategyrunner

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/zeroclap/gembo/internal/domain"
	"github.com/zeroclap/gembo/internal/store"
	"github.com/zeroclap/gembo/pkg/rediskey"
)

const debounceInterval = 3 * time.Second

// Runner evaluates automated strategies against live arbitrage opportunities
// and creates positions when criteria are satisfied.
type Runner struct {
	strategyStore store.StrategyStore
	positionStore store.PositionStore
	marketStore   store.Store
	redis         *goredis.Client
	logger        *slog.Logger
}

// New creates a Runner.
func New(
	ss store.StrategyStore,
	ps store.PositionStore,
	ms store.Store,
	rc *goredis.Client,
	logger *slog.Logger,
) *Runner {
	return &Runner{
		strategyStore: ss,
		positionStore: ps,
		marketStore:   ms,
		redis:         rc,
		logger:        logger.With("component", "strategy-runner"),
	}
}

// Run subscribes to arbitrage update pubsub channels and evaluates strategies
// on debounced events. Blocks until ctx is cancelled.
func (r *Runner) Run(ctx context.Context) error {
	pubsub := r.redis.Subscribe(ctx,
		rediskey.FundingArbitragePubSub(),
		rediskey.SpreadArbitragePubSub(),
	)
	defer pubsub.Close()

	ch := pubsub.Channel()

	timer := time.NewTimer(debounceInterval)
	timer.Stop()
	pending := false

	r.logger.Info("strategy runner started")

	for {
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()

		case _, ok := <-ch:
			if !ok {
				return nil
			}
			if !pending {
				timer.Reset(debounceInterval)
				pending = true
			}

		case <-timer.C:
			pending = false
			r.evaluate(ctx)
		}
	}
}

// evaluate loads all enabled strategies and checks them against current opportunities.
func (r *Runner) evaluate(ctx context.Context) {
	strategies, err := r.strategyStore.ListAllStrategies(ctx)
	if err != nil {
		r.logger.Warn("strategy runner list strategies", "error", err)
		return
	}
	if len(strategies) == 0 {
		return
	}

	// Load all current opportunities once, shared across all strategies.
	fundingOpps, err := r.marketStore.ListFundingArbitrage(ctx)
	if err != nil {
		r.logger.Warn("strategy runner list funding arbitrage", "error", err)
		return
	}
	spreadOpps, err := r.marketStore.ListSpreadArbitrage(ctx)
	if err != nil {
		r.logger.Warn("strategy runner list spread arbitrage", "error", err)
		return
	}

	for i := range strategies {
		s := &strategies[i]
		switch s.ArbitrageType {
		case domain.ArbitrageTypeFunding:
			r.evaluateFunding(ctx, s, fundingOpps)
		case domain.ArbitrageTypeSpread:
			r.evaluateSpread(ctx, s, spreadOpps)
		}
	}
}

// evaluateFunding checks a funding strategy against all current funding opportunities.
func (r *Runner) evaluateFunding(ctx context.Context, s *domain.Strategy, opps []domain.FundingArbitrage) {
	// Check concurrent position cap.
	if !r.hasCapacity(ctx, s) {
		return
	}

	for _, opp := range opps {
		// Criteria checks.
		if !s.MinAPR.IsZero() && opp.APR.LessThan(s.MinAPR) {
			continue
		}
		if len(s.SymbolWhitelist) > 0 && !contains(s.SymbolWhitelist, opp.Symbol) {
			continue
		}
		if len(s.ExchangeWhitelist) > 0 {
			if !contains(s.ExchangeWhitelist, opp.Long.ExchangeID) || !contains(s.ExchangeWhitelist, opp.Short.ExchangeID) {
				continue
			}
		}

		// Resolve credentials.
		longCredID, ok := s.Credentials[opp.Long.ExchangeID]
		if !ok {
			continue
		}
		shortCredID, ok := s.Credentials[opp.Short.ExchangeID]
		if !ok {
			continue
		}

		// Avoid duplicate: skip if an open/opening position already exists for this pair.
		if r.duplicateExists(ctx, s.UserID, opp.Symbol, opp.Long.ExchangeID, opp.Short.ExchangeID) {
			continue
		}

		// Re-check capacity before each creation (a prior iteration may have used a slot).
		if !r.hasCapacity(ctx, s) {
			return
		}

		snapshot, _ := json.Marshal(opp)
		p := &domain.Position{
			UserID:            s.UserID,
			Symbol:            opp.Symbol,
			Status:            domain.PositionStatusPending,
			SizeType:          domain.SizeTypePrice,
			RequestedSize:     s.SizeUSD,
			ArbitrageType:     domain.ArbitrageTypeFunding,
			Leverage:          s.Leverage,
			LongExchangeID:    opp.Long.ExchangeID,
			LongCredentialID:  longCredID,
			ShortExchangeID:   opp.Short.ExchangeID,
			ShortCredentialID: shortCredID,
			Snapshot:          json.RawMessage(snapshot),
		}

		if err := r.createPosition(ctx, p); err != nil {
			r.logger.Error("strategy runner create funding position",
				"strategy", s.ID, "symbol", opp.Symbol, "error", err)
			continue
		}

		r.logger.Info("strategy runner opened funding position",
			"strategy", s.ID, "position", p.ID,
			"symbol", opp.Symbol,
			"long_exchange", opp.Long.ExchangeID,
			"short_exchange", opp.Short.ExchangeID,
			"apr", opp.APR.StringFixed(2),
		)
	}
}

// evaluateSpread checks a spread strategy against all current spread opportunities.
func (r *Runner) evaluateSpread(ctx context.Context, s *domain.Strategy, opps []domain.SpreadArbitrage) {
	// Check concurrent position cap.
	if !r.hasCapacity(ctx, s) {
		return
	}

	for _, opp := range opps {
		// Criteria checks.
		if !s.MinNetSpreadPct.IsZero() && opp.NetSpreadPct.LessThan(s.MinNetSpreadPct) {
			continue
		}
		if len(s.SymbolWhitelist) > 0 && !contains(s.SymbolWhitelist, opp.Symbol) {
			continue
		}
		if len(s.ExchangeWhitelist) > 0 {
			if !contains(s.ExchangeWhitelist, opp.BuyExchangeID) || !contains(s.ExchangeWhitelist, opp.SellExchangeID) {
				continue
			}
		}

		// Resolve credentials: for spread arb, long = buy side, short = sell side.
		longCredID, ok := s.Credentials[opp.BuyExchangeID]
		if !ok {
			continue
		}
		shortCredID, ok := s.Credentials[opp.SellExchangeID]
		if !ok {
			continue
		}

		// Avoid duplicate.
		if r.duplicateExists(ctx, s.UserID, opp.Symbol, opp.BuyExchangeID, opp.SellExchangeID) {
			continue
		}

		// Re-check capacity.
		if !r.hasCapacity(ctx, s) {
			return
		}

		snapshot, _ := json.Marshal(opp)

		// Compute steps for size_usd as a single-shot price entry.
		p := &domain.Position{
			UserID:            s.UserID,
			Symbol:            opp.Symbol,
			Status:            domain.PositionStatusPending,
			SizeType:          domain.SizeTypePrice,
			RequestedSize:     s.SizeUSD,
			ArbitrageType:     domain.ArbitrageTypeSpread,
			Leverage:          s.Leverage,
			LongExchangeID:    opp.BuyExchangeID,
			LongCredentialID:  longCredID,
			ShortExchangeID:   opp.SellExchangeID,
			ShortCredentialID: shortCredID,
			TargetSpreadPct:   s.TargetSpreadPct,
			StopLossSpreadPct: s.StopLossSpreadPct,
			MaxHoldDuration:   s.MaxHoldDuration,
			// Capture entry spread from the live opportunity.
			EntrySpreadPct: opp.SpreadPct,
			Snapshot:       json.RawMessage(snapshot),
		}

		if err := r.createPosition(ctx, p); err != nil {
			r.logger.Error("strategy runner create spread position",
				"strategy", s.ID, "symbol", opp.Symbol, "error", err)
			continue
		}

		r.logger.Info("strategy runner opened spread position",
			"strategy", s.ID, "position", p.ID,
			"symbol", opp.Symbol,
			"buy_exchange", opp.BuyExchangeID,
			"sell_exchange", opp.SellExchangeID,
			"net_spread_pct", opp.NetSpreadPct.StringFixed(4),
		)
	}
}

// createPosition persists the position and signals the position manager via Redis.
func (r *Runner) createPosition(ctx context.Context, p *domain.Position) error {
	if err := r.positionStore.CreatePosition(ctx, p); err != nil {
		return err
	}

	// Enrich contract sizes from market data.
	if lm, err := r.marketStore.GetMarket(ctx, p.LongExchangeID, p.Symbol); err == nil && lm != nil {
		p.LongContractSize = lm.ContractSize
	}
	if sm, err := r.marketStore.GetMarket(ctx, p.ShortExchangeID, p.Symbol); err == nil && sm != nil {
		p.ShortContractSize = sm.ContractSize
	}
	if !p.LongContractSize.IsZero() || !p.ShortContractSize.IsZero() {
		_ = r.positionStore.UpdatePosition(ctx, p)
	}

	// Signal the position manager to execute the open.
	r.redis.Publish(ctx, rediskey.UserPositionPending(p.UserID), p.ID)
	return nil
}

// hasCapacity returns true if the strategy has room for another concurrent position.
func (r *Runner) hasCapacity(ctx context.Context, s *domain.Strategy) bool {
	maxConcurrent := s.MaxConcurrentPositions
	if maxConcurrent <= 0 {
		maxConcurrent = 1
	}

	activeStatuses := []domain.PositionStatus{
		domain.PositionStatusPending,
		domain.PositionStatusOpening,
		domain.PositionStatusOpen,
		domain.PositionStatusPartiallyOpen,
	}
	positions, err := r.positionStore.ListPositions(ctx, domain.PositionFilter{
		UserID: s.UserID,
		Status: activeStatuses,
	})
	if err != nil {
		r.logger.Warn("strategy runner check capacity", "strategy", s.ID, "error", err)
		return false
	}

	// Count only positions created by this strategy (matching arbitrage type).
	// We use a simple heuristic: count all active positions for this user of the
	// same arbitrage type. A dedicated strategy_id field could make this exact,
	// but counting by type gives a conservative bound without schema changes.
	count := 0
	for _, p := range positions {
		if p.ArbitrageType == s.ArbitrageType {
			count++
		}
	}
	return count < maxConcurrent
}

// duplicateExists returns true if an active position already exists for the
// exact same symbol and exchange pair for this user.
func (r *Runner) duplicateExists(ctx context.Context, userID, symbol, longExchangeID, shortExchangeID string) bool {
	activeStatuses := []domain.PositionStatus{
		domain.PositionStatusPending,
		domain.PositionStatusOpening,
		domain.PositionStatusOpen,
		domain.PositionStatusPartiallyOpen,
	}
	positions, err := r.positionStore.ListPositions(ctx, domain.PositionFilter{
		UserID: userID,
		Symbol: symbol,
		Status: activeStatuses,
	})
	if err != nil {
		return true // conservative: don't open if we can't check
	}
	for _, p := range positions {
		if p.LongExchangeID == longExchangeID && p.ShortExchangeID == shortExchangeID {
			return true
		}
	}
	return false
}

// contains reports whether slice contains item.
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
