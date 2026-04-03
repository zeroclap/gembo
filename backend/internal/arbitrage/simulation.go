package arbitrage

import (
	"time"

	"github.com/shopspring/decimal"

	"github.com/zeroclap/gembo/internal/domain"
)

// resolvePositionSize computes the effective quantity and notional position
// size from the simulation request and arbitrage opportunity.
//
// When size_type=usd: quantity = floor(position_size / (price * contract_size)).
// When size_type=qty: quantity is used as-is.
//
// Returns (quantity, actual_position_size). actual_position_size is always
// qty * price * contract_size. If contract_size is zero (unknown) it defaults to 1.
func resolvePositionSize(arb domain.FundingArbitrage, req domain.SimulationRequest) (decimal.Decimal, decimal.Decimal) {
	// Use the average price across both legs as reference price.
	price := arb.Long.Price.Add(arb.Short.Price).Div(dec2)
	if price.IsZero() {
		// Fallback: try individual non-zero prices.
		if !arb.Long.Price.IsZero() {
			price = arb.Long.Price
		} else {
			price = arb.Short.Price
		}
	}

	// Use the long leg's contract size (both legs trade the same symbol,
	// so contract size should be the same — but they may differ across exchanges).
	// Take the larger of the two so that the floor-rounding is conservative.
	contractSize := arb.Long.ContractSize
	if arb.Short.ContractSize.GreaterThan(contractSize) {
		contractSize = arb.Short.ContractSize
	}
	if contractSize.IsZero() {
		contractSize = decimal.NewFromInt(1) // linear perp: 1 contract = 1 base unit
	}

	// Value per contract in USD.
	valuePerContract := price.Mul(contractSize)

	var qty decimal.Decimal
	switch req.SizeType {
	case domain.SizeTypeQty:
		qty = req.Quantity
	default:
		// SizeTypeUSD or legacy (empty string = usd).
		if valuePerContract.IsPositive() {
			// Floor to whole contracts.
			qty = req.PositionSize.Div(valuePerContract).Floor()
			if qty.IsZero() {
				// Can't afford a single contract — clamp to 0.
				return decimal.Zero, decimal.Zero
			}
		} else {
			return decimal.Zero, decimal.Zero
		}
	}

	actualSize := qty.Mul(valuePerContract)
	return qty, actualSize
}

// Simulate computes the P&L simulation for a given arbitrage opportunity
// and position sizing parameters. This is a pure function — it does not
// touch the store. The logic mirrors the former frontend computeSimulation().
func Simulate(arb domain.FundingArbitrage, req domain.SimulationRequest) domain.SimulationResult {
	qty, actualSize := resolvePositionSize(arb, req)

	size := actualSize
	slippage := req.SlippagePct
	capital := size

	// Entry cost: open spread loss + open fees + slippage on entry.
	openSpreadPct := arb.OpenSpread // already in percent
	spreadCostPct := decimal.Zero
	if openSpreadPct.IsNegative() {
		spreadCostPct = openSpreadPct.Neg() // adverse spread only
	}
	spreadCost := spreadCostPct.Div(dec100).Mul(size)

	openFeesPct := arb.Fees.OpenFeesPct // already in percent
	openFees := openFeesPct.Div(dec100).Mul(size)

	closeFeesPct := arb.Fees.CloseFeesPct
	closeFees := closeFeesPct.Div(dec100).Mul(size)

	slippageCostPerSide := slippage.Div(dec100).Mul(size)

	entryCost := spreadCost.Add(openFees).Add(slippageCostPerSide)
	exitCost := closeFees.Add(slippageCostPerSide)
	totalCost := entryCost.Add(exitCost)

	// Funding P&L per 8h period (net_rate_8h is a ratio, e.g. 0.0001).
	netRate8h := arb.NetRate8h
	funding8h := netRate8h.Mul(size)
	funding24h := funding8h.Mul(dec3)

	// Breakeven.
	var breakevenDays decimal.Decimal
	if funding24h.IsPositive() {
		breakevenDays = totalCost.Div(funding24h)
	} else {
		// -1 means "never" — Infinity is not representable in decimal.
		breakevenDays = decimal.NewFromInt(-1)
	}

	// LCM P&L: funding earned over one full LCM loop (no cost amortization).
	netPctPerLoop := arb.FullLoop.NetPctPerLoop // ratio
	lcmPnL := netPctPerLoop.Mul(size)

	// Next payment P&L.
	nextPaymentPct := computeNextPaymentPct(arb)
	nextPnL := nextPaymentPct.Mul(size)

	// 24h P&L = funding24h.
	pnl24h := funding24h

	// Build per-payment USD breakdown for the full loop.
	loopPayments := make([]domain.SimulationLoopPayment, len(arb.FullLoop.Payments))
	for i, p := range arb.FullLoop.Payments {
		loopPayments[i] = domain.SimulationLoopPayment{
			ExchangeID:    p.ExchangeID,
			Side:          p.Side,
			Rate:          p.Rate,
			PaymentPct:    p.PaymentPct,
			AtHour:        p.AtHour,
			Cumulative:    p.Cumulative,
			PaymentUsd:    p.PaymentPct.Mul(size),
			CumulativeUsd: p.Cumulative.Mul(size),
		}
	}

	return domain.SimulationResult{
		PositionSize:       req.PositionSize,
		ActualPositionSize: actualSize,
		Quantity:           qty,
		Capital:            capital,
		EntryCost:          entryCost,
		ExitCost:           exitCost,
		TotalCost:          totalCost,
		Funding8h:          funding8h,
		Funding24h:         funding24h,
		BreakevenDays:      breakevenDays,
		LcmPnL:             lcmPnL,
		NextPnL:            nextPnL,
		Pnl24h:             pnl24h,
		LoopPayments:       loopPayments,
	}
}

// computeNextPaymentPct calculates the net P&L percentage for the next funding cycle.
// The "next" cycle is defined by the soonest next_funding_at across both legs.
// If both legs fire at the same time (within 60s) they are summed.
//
// Long leg P&L  = -fundingRate (positive rate -> you pay -> negative P&L)
// Short leg P&L = +fundingRate (positive rate -> you receive)
func computeNextPaymentPct(arb domain.FundingArbitrage) decimal.Decimal {
	longAt := arb.Long.NextFundingAt
	shortAt := arb.Short.NextFundingAt

	longValid := !longAt.IsZero()
	shortValid := !shortAt.IsZero()

	if !longValid && !shortValid {
		return decimal.Zero
	}

	longPnl := arb.Long.FundingRate.Neg() // long pays opposite
	shortPnl := arb.Short.FundingRate     // short receives as-is

	if !shortValid {
		return longPnl
	}
	if !longValid {
		return shortPnl
	}

	// Both valid — check if they coincide (within 60s tolerance).
	diff := longAt.Sub(shortAt)
	if diff < 0 {
		diff = -diff
	}
	tolerance := 60 * time.Second
	if diff <= tolerance {
		return longPnl.Add(shortPnl)
	}

	// Different times — pick the earlier one.
	if longAt.Before(shortAt) {
		return longPnl
	}
	return shortPnl
}
