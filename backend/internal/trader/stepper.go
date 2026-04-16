package trader

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
	"golang.org/x/sync/errgroup"

	"github.com/zeroclap/gembo/internal/domain"
	"github.com/zeroclap/gembo/internal/exchange"
	"github.com/zeroclap/gembo/pkg/rediskey"
)

// isStepped returns true if the position should use stepped execution.
func isStepped(p *domain.Position) bool {
	return p.PortionSize.GreaterThan(decimal.Zero) && p.StepsTotal > 1
}

// executeSteppedOpen handles the full open flow for a stepped position:
//  1. Validate and transition to "opening"
//  2. Configure leverage on both exchanges
//  3. Optionally check margin capital availability
//  4. Loop over each step:
//     a. Check price range against live tickers
//     b. Check slippage tolerance
//     c. Place one portion on both legs concurrently
//     d. Accumulate fill data (VWAP entry price, total qty, cost, fees)
//     e. Persist progress after each step
//  5. Finalize: mark as "open" (all steps) or "partially_open" (stopped early)
func (t *Trader) executeSteppedOpen(ctx context.Context, p *domain.Position) error {
	longExch, err := t.registry.Get(p.LongExchangeID)
	if err != nil {
		return t.failPosition(ctx, p, fmt.Errorf("long exchange: %w", err))
	}
	shortExch, err := t.registry.Get(p.ShortExchangeID)
	if err != nil {
		return t.failPosition(ctx, p, fmt.Errorf("short exchange: %w", err))
	}

	// Configure leverage using per-position value.
	if err := t.configureExchangeWithLeverage(ctx, longExch, p.Symbol, p.Leverage); err != nil {
		return t.failPosition(ctx, p, fmt.Errorf("configure long exchange: %w", err))
	}
	if err := t.configureExchangeWithLeverage(ctx, shortExch, p.Symbol, p.Leverage); err != nil {
		return t.failPosition(ctx, p, fmt.Errorf("configure short exchange: %w", err))
	}

	// Optional margin capital check before starting.
	if p.MarginCapital.GreaterThan(decimal.Zero) {
		if err := t.checkMarginCapital(ctx, longExch, shortExch, p); err != nil {
			return t.failPosition(ctx, p, fmt.Errorf("margin capital check: %w", err))
		}
	}

	// Resolve per-step amount in base currency.
	portionAmount, err := t.resolvePortionAmount(ctx, p, longExch, shortExch)
	if err != nil {
		return t.failPosition(ctx, p, fmt.Errorf("resolve portion amount: %w", err))
	}

	t.logger.Info("starting stepped execution",
		"position_id", p.ID,
		"symbol", p.Symbol,
		"steps_total", p.StepsTotal,
		"portion_amount", portionAmount,
	)

	// Accumulate fill data across steps for VWAP calculation.
	var (
		longTotalQty  decimal.Decimal
		longTotalCost decimal.Decimal
		longTotalFees decimal.Decimal

		shortTotalQty  decimal.Decimal
		shortTotalCost decimal.Decimal
		shortTotalFees decimal.Decimal
	)

	for step := range p.StepsTotal {
		stepNum := step + 1

		// For the last step, the portion might be smaller than portion_size
		// to avoid exceeding the total requested_size.
		stepAmount := portionAmount
		if p.SizeType == domain.SizeTypeQuantity {
			remaining := p.RequestedSize.Sub(longTotalQty)
			portionDec := decimal.NewFromFloat(portionAmount)
			if remaining.LessThan(portionDec) && remaining.GreaterThan(decimal.Zero) {
				stepAmount = remaining.InexactFloat64()
			}
		}

		// Check price range from live tickers.
		if ok, reason := t.checkPriceRange(ctx, p); !ok {
			p.StopReason = domain.StopReasonPriceExceeded
			t.logger.Warn("stepped execution stopped: price out of range",
				"position_id", p.ID,
				"step", stepNum,
				"reason", reason,
			)
			break
		}

		// Check slippage tolerance.
		if ok, reason := t.checkSlippage(ctx, p); !ok {
			p.StopReason = domain.StopReasonSlippage
			t.logger.Warn("stepped execution stopped: slippage exceeded",
				"position_id", p.ID,
				"step", stepNum,
				"reason", reason,
			)
			break
		}

		// Create order records for this step.
		longOrder := &domain.ExchangeOrder{
			PositionID: p.ID,
			ExchangeID: p.LongExchangeID,
			Symbol:     p.Symbol,
			Side:       domain.OrderSideBuy,
			Leg:        "long",
			Action:     "open",
			Status:     "pending",
		}
		shortOrder := &domain.ExchangeOrder{
			PositionID: p.ID,
			ExchangeID: p.ShortExchangeID,
			Symbol:     p.Symbol,
			Side:       domain.OrderSideSell,
			Leg:        "short",
			Action:     "open",
			Status:     "pending",
		}

		if err := t.store.CreateOrder(ctx, longOrder); err != nil {
			p.StopReason = domain.StopReasonStepFailed
			t.logger.Error("stepped execution: create long order record", "position_id", p.ID, "step", stepNum, "error", err)
			break
		}
		if err := t.store.CreateOrder(ctx, shortOrder); err != nil {
			p.StopReason = domain.StopReasonStepFailed
			t.logger.Error("stepped execution: create short order record", "position_id", p.ID, "step", stepNum, "error", err)
			break
		}

		// Place both legs concurrently for this step.
		type orderResult struct {
			order *domain.ExchangeOrder
			err   error
		}

		var longRes, shortRes orderResult
		var g errgroup.Group

		g.Go(func() error {
			filled, err := t.placeAndPoll(ctx, longExch, p.Symbol, domain.OrderSideBuy, stepAmount, longOrder)
			longRes = orderResult{order: filled, err: err}
			return nil
		})

		g.Go(func() error {
			filled, err := t.placeAndPoll(ctx, shortExch, p.Symbol, domain.OrderSideSell, stepAmount, shortOrder)
			shortRes = orderResult{order: filled, err: err}
			return nil
		})

		_ = g.Wait()

		if longRes.err != nil || shortRes.err != nil {
			// Step failed. Rollback this step's fills if needed.
			t.attemptRollback(ctx, p, longExch, shortExch, longRes.order, shortRes.order)
			p.StopReason = domain.StopReasonStepFailed
			t.logger.Error("stepped execution: step failed",
				"position_id", p.ID,
				"step", stepNum,
				"long_err", longRes.err,
				"short_err", shortRes.err,
			)
			break
		}

		longFilled, shortFilled := longRes.order, shortRes.order

		// Accumulate fill data.
		longTotalQty = longTotalQty.Add(longFilled.FilledQty)
		longTotalCost = longTotalCost.Add(longFilled.Cost)
		longTotalFees = longTotalFees.Add(longFilled.FeeCost)

		shortTotalQty = shortTotalQty.Add(shortFilled.FilledQty)
		shortTotalCost = shortTotalCost.Add(shortFilled.Cost)
		shortTotalFees = shortTotalFees.Add(shortFilled.FeeCost)

		p.StepsCompleted = stepNum

		// Update position with running totals after each step for durability.
		p.LongFilledQty = longTotalQty
		p.LongEntryCost = longTotalCost
		p.LongEntryFees = longTotalFees
		p.LongEntryPrice = vwapPrice(longTotalCost, longTotalQty)

		p.ShortFilledQty = shortTotalQty
		p.ShortEntryCost = shortTotalCost
		p.ShortEntryFees = shortTotalFees
		p.ShortEntryPrice = vwapPrice(shortTotalCost, shortTotalQty)

		if err := t.store.UpdatePosition(ctx, p); err != nil {
			t.logger.Error("stepped execution: persist step progress",
				"position_id", p.ID, "step", stepNum, "error", err)
			// Continue — the data is in memory even if the persist failed.
		}

		t.logger.Info("step completed",
			"position_id", p.ID,
			"step", stepNum,
			"steps_total", p.StepsTotal,
			"long_qty", longTotalQty,
			"short_qty", shortTotalQty,
		)
	}

	// Finalize the position based on how many steps completed.
	if p.StepsCompleted == 0 {
		return t.failPosition(ctx, p, fmt.Errorf("stepped execution: no steps completed, stop_reason=%s", p.StopReason))
	}

	now := time.Now().UTC()
	p.OpenedAt = &now

	if p.StepsCompleted == p.StepsTotal {
		p.Status = domain.PositionStatusOpen
	} else {
		// Partial fill — position is open but smaller than requested.
		p.Status = domain.PositionStatusOpen
	}

	if err := t.store.UpdatePosition(ctx, p); err != nil {
		return fmt.Errorf("update position to open: %w", err)
	}

	t.logger.Info("stepped position opened",
		"position_id", p.ID,
		"symbol", p.Symbol,
		"steps_completed", p.StepsCompleted,
		"steps_total", p.StepsTotal,
		"stop_reason", p.StopReason,
		"long_price", p.LongEntryPrice,
		"short_price", p.ShortEntryPrice,
		"long_qty", p.LongFilledQty,
		"short_qty", p.ShortFilledQty,
	)

	t.redis.Publish(ctx, rediskey.PositionUpdates(), p.ID)
	return nil
}

// resolvePortionAmount determines the per-step order amount in base currency.
func (t *Trader) resolvePortionAmount(
	ctx context.Context,
	p *domain.Position,
	longExch, shortExch exchange.TradingExchange,
) (float64, error) {
	if p.SizeType == domain.SizeTypeQuantity {
		return p.PortionSize.InexactFloat64(), nil
	}

	// SizeTypePrice — need a reference price to convert.
	refPrice, err := t.referencePrice(ctx, p.Symbol, longExch, shortExch)
	if err != nil {
		return 0, err
	}

	amount := p.PortionSize.Div(refPrice)
	return amount.InexactFloat64(), nil
}

// referencePrice fetches a mark price from one of the exchanges for USD→base conversion.
func (t *Trader) referencePrice(
	ctx context.Context,
	symbol string,
	longExch, shortExch exchange.TradingExchange,
) (decimal.Decimal, error) {
	longPos, err := longExch.FetchPosition(ctx, symbol)
	if err == nil && longPos != nil && !longPos.MarkPrice.IsZero() {
		return longPos.MarkPrice, nil
	}

	shortPos, err := shortExch.FetchPosition(ctx, symbol)
	if err == nil && shortPos != nil && !shortPos.MarkPrice.IsZero() {
		return shortPos.MarkPrice, nil
	}

	// Fall back to ticker data from Redis.
	ticker, err := t.fetchTicker(ctx, longExch.ID(), symbol)
	if err == nil && ticker != nil && !ticker.Price.IsZero() {
		return ticker.Price, nil
	}
	ticker, err = t.fetchTicker(ctx, shortExch.ID(), symbol)
	if err == nil && ticker != nil && !ticker.Price.IsZero() {
		return ticker.Price, nil
	}

	return decimal.Zero, fmt.Errorf("unable to determine reference price for %s", symbol)
}

// checkPriceRange verifies the current bid/ask prices are within the position's
// configured min/max range. Returns (ok, reason).
func (t *Trader) checkPriceRange(ctx context.Context, p *domain.Position) (bool, string) {
	// No price limits configured — always pass.
	if p.PriceMin.IsZero() && p.PriceMax.IsZero() {
		return true, ""
	}

	// Fetch tickers for both exchanges.
	longTicker, err := t.fetchTicker(ctx, p.LongExchangeID, p.Symbol)
	if err != nil || longTicker == nil {
		// If we can't fetch ticker data, fail safe by stopping.
		return false, fmt.Sprintf("failed to fetch long ticker: %v", err)
	}
	shortTicker, err := t.fetchTicker(ctx, p.ShortExchangeID, p.Symbol)
	if err != nil || shortTicker == nil {
		return false, fmt.Sprintf("failed to fetch short ticker: %v", err)
	}

	// For the long leg (buying), the relevant price is the ask.
	// For the short leg (selling), the relevant price is the bid.
	longAsk := longTicker.Ask
	shortBid := shortTicker.Bid

	// Use last price as fallback if bid/ask are zero.
	if longAsk.IsZero() {
		longAsk = longTicker.Price
	}
	if shortBid.IsZero() {
		shortBid = shortTicker.Price
	}

	// Check the long leg (buying): ask must be within [min, max].
	if !p.PriceMin.IsZero() && longAsk.LessThan(p.PriceMin) {
		return false, fmt.Sprintf("long ask %s < price_min %s", longAsk, p.PriceMin)
	}
	if !p.PriceMax.IsZero() && longAsk.GreaterThan(p.PriceMax) {
		return false, fmt.Sprintf("long ask %s > price_max %s", longAsk, p.PriceMax)
	}

	// Check the short leg (selling): bid must be within [min, max].
	if !p.PriceMin.IsZero() && shortBid.LessThan(p.PriceMin) {
		return false, fmt.Sprintf("short bid %s < price_min %s", shortBid, p.PriceMin)
	}
	if !p.PriceMax.IsZero() && shortBid.GreaterThan(p.PriceMax) {
		return false, fmt.Sprintf("short bid %s > price_max %s", shortBid, p.PriceMax)
	}

	return true, ""
}

// checkSlippage verifies that the current spread on both exchanges is within
// the position's slippage tolerance. Returns (ok, reason).
func (t *Trader) checkSlippage(ctx context.Context, p *domain.Position) (bool, string) {
	if p.SlippagePct.IsZero() {
		return true, ""
	}

	longTicker, err := t.fetchTicker(ctx, p.LongExchangeID, p.Symbol)
	if err != nil || longTicker == nil {
		return false, fmt.Sprintf("failed to fetch long ticker: %v", err)
	}
	shortTicker, err := t.fetchTicker(ctx, p.ShortExchangeID, p.Symbol)
	if err != nil || shortTicker == nil {
		return false, fmt.Sprintf("failed to fetch short ticker: %v", err)
	}

	// Check long leg slippage: (ask - mid) / mid * 100.
	if err := t.checkLegSlippage(longTicker, p.SlippagePct, "long"); err != nil {
		return false, err.Error()
	}

	// Check short leg slippage: (mid - bid) / mid * 100.
	if err := t.checkLegSlippage(shortTicker, p.SlippagePct, "short"); err != nil {
		return false, err.Error()
	}

	return true, ""
}

// checkLegSlippage computes the slippage for a single leg and returns an error if it exceeds tolerance.
func (t *Trader) checkLegSlippage(ticker *domain.Ticker, maxSlippage decimal.Decimal, leg string) error {
	bid := ticker.Bid
	ask := ticker.Ask

	// Need both bid and ask to compute slippage.
	if bid.IsZero() || ask.IsZero() {
		return nil // can't check, allow the trade
	}

	two := decimal.NewFromInt(2)
	hundred := decimal.NewFromInt(100)
	mid := bid.Add(ask).Div(two)
	if mid.IsZero() {
		return nil
	}

	var slippage decimal.Decimal
	if leg == "long" {
		// Buying at ask: slippage = (ask - mid) / mid * 100
		slippage = ask.Sub(mid).Div(mid).Mul(hundred)
	} else {
		// Selling at bid: slippage = (mid - bid) / mid * 100
		slippage = mid.Sub(bid).Div(mid).Mul(hundred)
	}

	if slippage.GreaterThan(maxSlippage) {
		return fmt.Errorf("%s leg slippage %.4f%% exceeds max %.4f%%", leg, slippage.InexactFloat64(), maxSlippage.InexactFloat64())
	}

	return nil
}

// fetchTicker reads a ticker from Redis for the given exchange and symbol.
func (t *Trader) fetchTicker(ctx context.Context, exchangeID, symbol string) (*domain.Ticker, error) {
	data, err := t.redis.Get(ctx, rediskey.Ticker(exchangeID, symbol)).Bytes()
	if err != nil {
		return nil, fmt.Errorf("read ticker %s/%s: %w", exchangeID, symbol, err)
	}
	var ticker domain.Ticker
	if err := json.Unmarshal(data, &ticker); err != nil {
		return nil, fmt.Errorf("unmarshal ticker %s/%s: %w", exchangeID, symbol, err)
	}
	return &ticker, nil
}

// checkMarginCapital verifies that sufficient collateral is available on both
// exchanges before starting stepped execution. This is a pre-flight check —
// margin can change during execution due to fills.
func (t *Trader) checkMarginCapital(
	ctx context.Context,
	longExch, shortExch exchange.TradingExchange,
	p *domain.Position,
) error {
	// Minimum required margin per exchange: position_notional / leverage.
	// For the pre-check we use half the margin capital per exchange as a heuristic
	// since the user declares total available collateral.
	half := p.MarginCapital.Div(decimal.NewFromInt(2))

	longBalances, err := longExch.FetchBalance(ctx)
	if err != nil {
		return fmt.Errorf("fetch long exchange balance: %w", err)
	}
	longFree := findUSDTFree(longBalances)
	if longFree.LessThan(half) {
		return fmt.Errorf("long exchange %s: available USDT %s < required margin %s",
			p.LongExchangeID, longFree, half)
	}

	shortBalances, err := shortExch.FetchBalance(ctx)
	if err != nil {
		return fmt.Errorf("fetch short exchange balance: %w", err)
	}
	shortFree := findUSDTFree(shortBalances)
	if shortFree.LessThan(half) {
		return fmt.Errorf("short exchange %s: available USDT %s < required margin %s",
			p.ShortExchangeID, shortFree, half)
	}

	t.logger.Info("margin capital check passed",
		"position_id", p.ID,
		"long_free", longFree,
		"short_free", shortFree,
		"required_per_side", half,
	)

	return nil
}

// findUSDTFree finds the free USDT balance from a balance slice.
func findUSDTFree(balances []exchange.Balance) decimal.Decimal {
	for _, b := range balances {
		if b.Currency == "USDT" {
			return b.Free
		}
	}
	return decimal.Zero
}

// configureExchangeWithLeverage sets margin mode and leverage on an exchange
// using per-position leverage rather than the global config.
func (t *Trader) configureExchangeWithLeverage(
	ctx context.Context,
	exch exchange.TradingExchange,
	symbol string,
	leverage int,
) error {
	if t.cfg.MarginMode != "" {
		if err := exch.SetMarginMode(ctx, symbol, t.cfg.MarginMode, leverage); err != nil {
			return fmt.Errorf("set margin mode: %w", err)
		}
	}
	return nil
}

// vwapPrice computes the volume-weighted average price: totalCost / totalQty.
// Returns zero if totalQty is zero.
func vwapPrice(totalCost, totalQty decimal.Decimal) decimal.Decimal {
	if totalQty.IsZero() {
		return decimal.Zero
	}
	return totalCost.Div(totalQty)
}
