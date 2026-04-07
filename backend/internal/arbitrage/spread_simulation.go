package arbitrage

import (
	"github.com/shopspring/decimal"

	"github.com/zeroclap/gembo/internal/domain"
)

// SimulateSpread computes the P&L simulation for a spread arbitrage opportunity.
// This is a pure function — it does not touch the store.
func SimulateSpread(arb domain.SpreadArbitrage, req domain.SpreadSimulationRequest) domain.SpreadSimulationResult {
	size := req.PositionSize
	slippage := req.SlippagePct

	entrySpreadPct := arb.SpreadPct
	targetSpreadPct := req.TargetSpreadPct

	// Spread capture: the percentage points of spread we expect to capture
	// when spread narrows from entry to target.
	spreadCapturePct := entrySpreadPct.Sub(targetSpreadPct)
	spreadCaptureUsd := spreadCapturePct.Div(dec100).Mul(size)

	// Entry cost: open fees + slippage on both legs.
	openFeesPct := arb.Fees.OpenFeesPct
	openFeesUsd := openFeesPct.Div(dec100).Mul(size)
	slippagePerSide := slippage.Div(dec100).Mul(size)
	entryCost := openFeesUsd.Add(slippagePerSide)

	// Exit cost: close fees + slippage on both legs.
	closeFeesPct := arb.Fees.CloseFeesPct
	closeFeesUsd := closeFeesPct.Div(dec100).Mul(size)
	exitCost := closeFeesUsd.Add(slippagePerSide)

	totalCost := entryCost.Add(exitCost)

	grossPnl := spreadCaptureUsd
	netPnl := grossPnl.Sub(totalCost)

	var netPnlPct decimal.Decimal
	if size.IsPositive() {
		netPnlPct = netPnl.Div(size).Mul(dec100)
	}

	// Breakeven: the target spread at which net P&L = 0.
	// breakeven_spread = entry_spread - (total_cost / size * 100)
	var breakevenSpreadPct decimal.Decimal
	if size.IsPositive() {
		costPct := totalCost.Div(size).Mul(dec100)
		breakevenSpreadPct = entrySpreadPct.Sub(costPct)
	}

	return domain.SpreadSimulationResult{
		PositionSize:       size,
		EntrySpreadPct:     entrySpreadPct,
		TargetSpreadPct:    targetSpreadPct,
		SpreadCapturePct:   spreadCapturePct,
		SpreadCaptureUsd:   spreadCaptureUsd,
		EntryCost:          entryCost,
		ExitCost:           exitCost,
		TotalCost:          totalCost,
		GrossPnl:           grossPnl,
		NetPnl:             netPnl,
		NetPnlPct:          netPnlPct,
		BreakevenSpreadPct: breakevenSpreadPct,
	}
}
