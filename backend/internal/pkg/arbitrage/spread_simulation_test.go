package arbitrage

import (
	"testing"

	"github.com/zeroclap/gembo/internal/domain"
)

func TestSimulateSpread_BasicProfitable(t *testing.T) {
	// Entry spread 2%, target 0.5%, fees 0.11% open + 0.11% close.
	arb := domain.SpreadArbitrage{
		Symbol:         "BTC/USDT:USDT",
		BuyExchangeID:  "binance",
		SellExchangeID: "bybit",
		SpreadPct:      d("2.0"),
		Fees: domain.SpreadArbitrageFees{
			OpenFeesPct:      d("0.11"),
			CloseFeesPct:     d("0.11"),
			RoundTripFeesPct: d("0.22"),
		},
	}

	req := domain.SpreadSimulationRequest{
		PositionSize:    d("10000"),
		TargetSpreadPct: d("0.5"),
		SlippagePct:     d("0.05"),
	}

	result := SimulateSpread(arb, req)

	// Spread capture: 2.0 - 0.5 = 1.5%
	assertDecimalEqual(t, "SpreadCapturePct", d("1.5"), result.SpreadCapturePct)

	// Spread capture USD: 1.5% of 10000 = 150
	assertDecimalEqual(t, "SpreadCaptureUsd", d("150"), result.SpreadCaptureUsd)

	// Entry cost: open_fees(0.11% * 10000 = 11) + slippage(0.05% * 10000 = 5) = 16
	assertDecimalEqual(t, "EntryCost", d("16"), result.EntryCost)

	// Exit cost: close_fees(0.11% * 10000 = 11) + slippage(0.05% * 10000 = 5) = 16
	assertDecimalEqual(t, "ExitCost", d("16"), result.ExitCost)

	// Total cost: 16 + 16 = 32
	assertDecimalEqual(t, "TotalCost", d("32"), result.TotalCost)

	// Gross P&L = spread capture = 150
	assertDecimalEqual(t, "GrossPnl", d("150"), result.GrossPnl)

	// Net P&L = 150 - 32 = 118
	assertDecimalEqual(t, "NetPnl", d("118"), result.NetPnl)

	// Net P&L % = 118/10000 * 100 = 1.18%
	assertDecimalClose(t, "NetPnlPct", d("1.18"), result.NetPnlPct, d("0.001"))

	// Position size preserved.
	assertDecimalEqual(t, "PositionSize", d("10000"), result.PositionSize)
}

func TestSimulateSpread_ZeroSpreadCapture(t *testing.T) {
	// Target = Entry spread → no profit.
	arb := domain.SpreadArbitrage{
		SpreadPct: d("1.0"),
		Fees: domain.SpreadArbitrageFees{
			OpenFeesPct:      d("0.11"),
			CloseFeesPct:     d("0.11"),
			RoundTripFeesPct: d("0.22"),
		},
	}

	req := domain.SpreadSimulationRequest{
		PositionSize:    d("1000"),
		TargetSpreadPct: d("1.0"), // same as entry
		SlippagePct:     d("0"),
	}

	result := SimulateSpread(arb, req)

	assertDecimalEqual(t, "SpreadCapturePct", d("0"), result.SpreadCapturePct)
	assertDecimalEqual(t, "SpreadCaptureUsd", d("0"), result.SpreadCaptureUsd)
	assertDecimalEqual(t, "GrossPnl", d("0"), result.GrossPnl)

	// Net P&L should be negative (paying fees only).
	if !result.NetPnl.IsNegative() {
		t.Errorf("expected negative NetPnl, got %s", result.NetPnl)
	}
}

func TestSimulateSpread_NegativeTarget(t *testing.T) {
	// Entry=2%, target=-0.5% (spread not only converges but inverts).
	// Spread capture = 2.0 - (-0.5) = 2.5%
	arb := domain.SpreadArbitrage{
		SpreadPct: d("2.0"),
		Fees: domain.SpreadArbitrageFees{
			OpenFeesPct:  d("0.1"),
			CloseFeesPct: d("0.1"),
		},
	}

	req := domain.SpreadSimulationRequest{
		PositionSize:    d("5000"),
		TargetSpreadPct: d("-0.5"),
		SlippagePct:     d("0"),
	}

	result := SimulateSpread(arb, req)

	assertDecimalEqual(t, "SpreadCapturePct", d("2.5"), result.SpreadCapturePct)
	assertDecimalEqual(t, "SpreadCaptureUsd", d("125"), result.SpreadCaptureUsd)
}

func TestSimulateSpread_NoSlippage(t *testing.T) {
	arb := domain.SpreadArbitrage{
		SpreadPct: d("1.0"),
		Fees: domain.SpreadArbitrageFees{
			OpenFeesPct:  d("0.10"),
			CloseFeesPct: d("0.10"),
		},
	}

	req := domain.SpreadSimulationRequest{
		PositionSize:    d("1000"),
		TargetSpreadPct: d("0"),
		SlippagePct:     d("0"),
	}

	result := SimulateSpread(arb, req)

	// Entry cost = fees only: 0.10% * 1000 = 1.0
	assertDecimalEqual(t, "EntryCost", d("1"), result.EntryCost)
	// Exit cost = fees only: 0.10% * 1000 = 1.0
	assertDecimalEqual(t, "ExitCost", d("1"), result.ExitCost)
	assertDecimalEqual(t, "TotalCost", d("2"), result.TotalCost)
}

func TestSimulateSpread_ZeroPositionSize(t *testing.T) {
	arb := domain.SpreadArbitrage{
		SpreadPct: d("1.0"),
		Fees: domain.SpreadArbitrageFees{
			OpenFeesPct:  d("0.10"),
			CloseFeesPct: d("0.10"),
		},
	}

	req := domain.SpreadSimulationRequest{
		PositionSize:    d("0"),
		TargetSpreadPct: d("0"),
		SlippagePct:     d("0.05"),
	}

	result := SimulateSpread(arb, req)

	assertDecimalEqual(t, "NetPnl", d("0"), result.NetPnl)
	assertDecimalEqual(t, "NetPnlPct", d("0"), result.NetPnlPct)
	assertDecimalEqual(t, "BreakevenSpreadPct", d("0"), result.BreakevenSpreadPct)
}

func TestSimulateSpread_BreakevenSpread(t *testing.T) {
	// Verify breakeven_spread = entry_spread - (total_cost / size * 100)
	arb := domain.SpreadArbitrage{
		SpreadPct: d("2.0"),
		Fees: domain.SpreadArbitrageFees{
			OpenFeesPct:  d("0.10"),
			CloseFeesPct: d("0.10"),
		},
	}

	req := domain.SpreadSimulationRequest{
		PositionSize:    d("10000"),
		TargetSpreadPct: d("0"),
		SlippagePct:     d("0.05"),
	}

	result := SimulateSpread(arb, req)

	// Total cost = entry + exit
	// entry = 0.10% * 10000 + 0.05% * 10000 = 10 + 5 = 15
	// exit  = 0.10% * 10000 + 0.05% * 10000 = 10 + 5 = 15
	// total = 30
	assertDecimalEqual(t, "TotalCost", d("30"), result.TotalCost)

	// breakeven = 2.0 - (30/10000 * 100) = 2.0 - 0.3 = 1.7%
	assertDecimalClose(t, "BreakevenSpreadPct", d("1.7"), result.BreakevenSpreadPct, d("0.001"))

	// Verify: if we simulate with target=breakeven, net PnL should be ~0.
	req2 := domain.SpreadSimulationRequest{
		PositionSize:    d("10000"),
		TargetSpreadPct: result.BreakevenSpreadPct,
		SlippagePct:     d("0.05"),
	}
	result2 := SimulateSpread(arb, req2)
	assertDecimalClose(t, "NetPnl@breakeven", d("0"), result2.NetPnl, d("0.01"))
}

func TestSimulateSpread_LargeSpread(t *testing.T) {
	// Extreme spread: 10% entry, 0% target.
	arb := domain.SpreadArbitrage{
		SpreadPct: d("10"),
		Fees: domain.SpreadArbitrageFees{
			OpenFeesPct:  d("0.06"),
			CloseFeesPct: d("0.06"),
		},
	}

	req := domain.SpreadSimulationRequest{
		PositionSize:    d("100000"),
		TargetSpreadPct: d("0"),
		SlippagePct:     d("0.1"),
	}

	result := SimulateSpread(arb, req)

	// Spread capture = 10% * 100000 = 10000
	assertDecimalEqual(t, "SpreadCaptureUsd", d("10000"), result.SpreadCaptureUsd)

	// Should be nicely profitable.
	if !result.NetPnl.IsPositive() {
		t.Errorf("expected positive NetPnl for large spread, got %s", result.NetPnl)
	}
}
