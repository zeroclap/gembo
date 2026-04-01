package trader

import (
	"context"
	"time"

	"github.com/shopspring/decimal"

	"github.com/zeroclap/gembo/internal/domain"
)

// runFundingTracker periodically checks for new funding payments on open positions.
// It cross-references the funding rate schedule with position state to detect
// when a funding payment has occurred.
func (t *Trader) runFundingTracker(ctx context.Context) error {
	ticker := time.NewTicker(t.fundingCheckInterval())
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			t.trackFundingOnce(ctx)
		}
	}
}

// trackFundingOnce iterates over all open positions and checks each leg for
// new funding payments based on the funding rate schedule.
func (t *Trader) trackFundingOnce(ctx context.Context) {
	positions, err := t.store.ListPositions(ctx, domain.PositionFilter{
		Status: []domain.PositionStatus{
			domain.PositionStatusOpen,
			domain.PositionStatusPartiallyOpen,
		},
	})
	if err != nil {
		t.logger.Warn("funding tracker: list open positions", "error", err)
		return
	}

	if len(positions) == 0 {
		return
	}

	t.logger.Debug("checking funding payments", "positions", len(positions))

	for i := range positions {
		p := &positions[i]
		t.checkFundingForPosition(ctx, p)
	}
}

// checkFundingForPosition checks both legs of a position for new funding payments.
// A funding payment is detected when:
//  1. The position has been open past a funding interval boundary
//  2. We haven't already recorded a payment for that boundary
func (t *Trader) checkFundingForPosition(ctx context.Context, p *domain.Position) {
	// Check long leg.
	t.checkFundingForLeg(ctx, p, p.LongExchangeID, "long", p.LongFilledQty)

	// Check short leg.
	t.checkFundingForLeg(ctx, p, p.ShortExchangeID, "short", p.ShortFilledQty)
}

// checkFundingForLeg checks a single leg for new funding payments.
// It fetches the current position from the exchange to get the latest funding
// rate data and compares with our last recorded payment.
func (t *Trader) checkFundingForLeg(
	ctx context.Context,
	p *domain.Position,
	exchangeID, leg string,
	qty decimal.Decimal,
) {
	exch, err := t.registry.Get(exchangeID)
	if err != nil {
		t.logger.Debug("funding tracker: exchange not found", "exchange", exchangeID, "error", err)
		return
	}

	// Get the latest funding payment we've recorded for this leg.
	latest, err := t.store.LatestFundingPayment(ctx, p.ID, exchangeID)
	if err != nil {
		t.logger.Warn("funding tracker: latest payment lookup",
			"position_id", p.ID, "exchange", exchangeID, "error", err)
		return
	}

	// Fetch the current position from the exchange.
	// We use this to confirm the position is still open and get the mark price.
	pos, err := exch.FetchPosition(ctx, p.Symbol)
	if err != nil {
		t.logger.Debug("funding tracker: fetch position",
			"exchange", exchangeID, "symbol", p.Symbol, "error", err)
		return
	}
	if pos == nil {
		return
	}

	// Determine if a funding payment has likely occurred since the last check.
	// We use a heuristic: check the position's opened_at time and compute
	// how many funding intervals have passed. If that count exceeds our
	// recorded payment count, we record a synthetic payment.
	now := time.Now().UTC()
	openedAt := p.CreatedAt
	if p.OpenedAt != nil {
		openedAt = *p.OpenedAt
	}

	// Try to determine the funding interval and current rate from exchange data.
	// The funding rate data is available in Redis — we read it via the store.
	// For now, we use 8h as default and detect based on time boundaries.
	fundingInterval := 8 * time.Hour

	// Calculate the most recent funding time before now.
	lastFundingTime := lastFundingBoundary(now, fundingInterval)

	// If position wasn't open at the last funding time, skip.
	if openedAt.After(lastFundingTime) {
		return
	}

	// If we already have a payment at or after the last funding time, skip.
	if latest != nil && !latest.FundedAt.Before(lastFundingTime) {
		return
	}

	// A funding payment occurred. Estimate the payment amount.
	// payment = qty * mark_price * funding_rate
	// Since we don't have the exact rate from the exchange at funding time,
	// we approximate using the mark price from the current position state.
	// The sign depends on which side we're on:
	//   - Long pays short when rate is positive: long pays (negative), short receives (positive)
	//   - Long receives from short when rate is negative
	rate := t.estimateFundingRate(ctx, p.Symbol, exchangeID, leg)
	if rate.IsZero() {
		t.logger.Debug("funding tracker: rate is zero, skipping",
			"position_id", p.ID, "exchange", exchangeID, "leg", leg)
		return
	}

	notional := qty.Mul(pos.MarkPrice)
	payment := notional.Mul(rate)

	// For long positions: if rate > 0, long pays -> payment is negative
	// For short positions: if rate > 0, short receives -> payment is positive
	if leg == "long" {
		payment = payment.Neg()
	}

	fp := &domain.FundingPayment{
		PositionID: p.ID,
		ExchangeID: exchangeID,
		Leg:        leg,
		Symbol:     p.Symbol,
		Rate:       rate,
		Payment:    payment,
		FundedAt:   lastFundingTime,
	}

	if err := t.store.CreateFundingPayment(ctx, fp); err != nil {
		t.logger.Warn("funding tracker: create payment",
			"position_id", p.ID, "exchange", exchangeID, "leg", leg, "error", err)
		return
	}

	t.logger.Info("funding payment recorded",
		"position_id", p.ID,
		"exchange", exchangeID,
		"leg", leg,
		"symbol", p.Symbol,
		"rate", rate,
		"payment", payment,
		"funded_at", lastFundingTime,
	)
}

// estimateFundingRate retrieves the current funding rate for a symbol on an
// exchange from Redis. Returns the raw rate value.
func (t *Trader) estimateFundingRate(ctx context.Context, symbol, exchangeID, leg string) decimal.Decimal {
	// We don't have direct access to the Redis store.Store interface in the trader.
	// Instead, we read the funding rate from Redis using the key convention.
	key := "gembo:" + exchangeID + ":funding:" + symbol
	result, err := t.redis.HGetAll(ctx, key).Result()
	if err != nil || len(result) == 0 {
		return decimal.Zero
	}

	rateStr, ok := result["rate"]
	if !ok || rateStr == "" {
		return decimal.Zero
	}

	rate, err := decimal.NewFromString(rateStr)
	if err != nil {
		return decimal.Zero
	}

	return rate
}

// lastFundingBoundary returns the most recent funding time boundary before t.
// Funding times are aligned to UTC midnight (00:00, 08:00, 16:00 for 8h intervals).
func lastFundingBoundary(t time.Time, interval time.Duration) time.Time {
	// Truncate to the start of the day, then step by interval.
	dayStart := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	intervalSecs := int64(interval.Seconds())
	elapsed := int64(t.Sub(dayStart).Seconds())
	periods := elapsed / intervalSecs
	return dayStart.Add(time.Duration(periods*intervalSecs) * time.Second)
}
