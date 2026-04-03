package arbitrage

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/zeroclap/gembo/internal/domain"
)

// testLegDefaults returns an ArbitrageLeg with Price=1, ContractSize=1 so
// that USD-mode floor-rounding is a no-op ($1000 ÷ $1/contract = 1000 contracts).
func testLegDefaults() domain.ArbitrageLeg {
	return domain.ArbitrageLeg{
		Price:        d("1"),
		ContractSize: d("1"),
	}
}

// mergeLeg copies non-zero fields from override into base.
func mergeLeg(base, override domain.ArbitrageLeg) domain.ArbitrageLeg {
	if override.ExchangeID != "" {
		base.ExchangeID = override.ExchangeID
	}
	if !override.FundingRate.IsZero() {
		base.FundingRate = override.FundingRate
	}
	if override.IntervalHours != 0 {
		base.IntervalHours = override.IntervalHours
	}
	if !override.Rate8h.IsZero() {
		base.Rate8h = override.Rate8h
	}
	if !override.NextFundingAt.IsZero() {
		base.NextFundingAt = override.NextFundingAt
	}
	if !override.NextFundingRate.IsZero() {
		base.NextFundingRate = override.NextFundingRate
	}
	if !override.Price.IsZero() {
		base.Price = override.Price
	}
	if !override.ContractSize.IsZero() {
		base.ContractSize = override.ContractSize
	}
	return base
}

func TestSimulate_BasicCalculation(t *testing.T) {
	arb := domain.FundingArbitrage{
		Symbol: "BTC/USDT:USDT",
		Long: mergeLeg(testLegDefaults(), domain.ArbitrageLeg{
			ExchangeID:    "bybit",
			FundingRate:   d("0.0001"),
			IntervalHours: 8,
			Rate8h:        d("0.0001"),
			NextFundingAt: time.Now().Add(2 * time.Hour),
		}),
		Short: mergeLeg(testLegDefaults(), domain.ArbitrageLeg{
			ExchangeID:    "binance",
			FundingRate:   d("0.0005"),
			IntervalHours: 8,
			Rate8h:        d("0.0005"),
			NextFundingAt: time.Now().Add(2 * time.Hour),
		}),
		NetRate8h:  d("0.0004"), // short_rate_8h - long_rate_8h
		OpenSpread: d("0.05"),   // positive spread (favorable)
		Fees: domain.ArbitrageFees{
			OpenFeesPct:  d("0.11"), // 0.055% * 2 legs * 100
			CloseFeesPct: d("0.11"), // same
		},
		FullLoop: domain.FullLoopInfo{
			DurationHours: 8,
			NetPctPerLoop: d("0.0004"),
		},
	}

	req := domain.SimulationRequest{
		Symbol:          "BTC/USDT:USDT",
		LongExchangeID:  "bybit",
		ShortExchangeID: "binance",
		PositionSize:    d("1000"),
		SlippagePct:     d("0.05"),
	}

	result := Simulate(arb, req)

	// Position size and capital should match.
	assertDecimalEqual(t, "PositionSize", d("1000"), result.PositionSize)
	assertDecimalEqual(t, "Capital", d("1000"), result.Capital)
	assertDecimalEqual(t, "Quantity", d("1000"), result.Quantity)
	assertDecimalEqual(t, "ActualPositionSize", d("1000"), result.ActualPositionSize)

	// Funding P&L: 0.0004 * 1000 = 0.4 per 8h.
	assertDecimalEqual(t, "Funding8h", d("0.4"), result.Funding8h)

	// Funding24h: 0.4 * 3 = 1.2 per day.
	assertDecimalEqual(t, "Funding24h", d("1.2"), result.Funding24h)

	// LCM P&L: 0.0004 * 1000 = 0.4.
	assertDecimalEqual(t, "LcmPnL", d("0.4"), result.LcmPnL)

	// Open spread is positive, so no spread cost.
	// EntryCost = openFees + slippage = (0.11/100)*1000 + (0.05/100)*1000 = 1.1 + 0.5 = 1.6
	assertDecimalEqual(t, "EntryCost", d("1.6"), result.EntryCost)

	// ExitCost = closeFees + slippage = (0.11/100)*1000 + (0.05/100)*1000 = 1.1 + 0.5 = 1.6
	assertDecimalEqual(t, "ExitCost", d("1.6"), result.ExitCost)

	// TotalCost = 1.6 + 1.6 = 3.2
	assertDecimalEqual(t, "TotalCost", d("3.2"), result.TotalCost)

	// Breakeven: 3.2 / 1.2 ≈ 2.666...
	expectedBreakeven := d("3.2").Div(d("1.2"))
	assertDecimalClose(t, "BreakevenDays", expectedBreakeven, result.BreakevenDays, d("0.001"))
}

func TestSimulate_NegativeSpread(t *testing.T) {
	arb := domain.FundingArbitrage{
		Symbol: "ETH/USDT:USDT",
		Long: mergeLeg(testLegDefaults(), domain.ArbitrageLeg{
			ExchangeID:    "bybit",
			FundingRate:   d("0.0001"),
			IntervalHours: 8,
			Rate8h:        d("0.0001"),
			NextFundingAt: time.Now().Add(2 * time.Hour),
		}),
		Short: mergeLeg(testLegDefaults(), domain.ArbitrageLeg{
			ExchangeID:    "binance",
			FundingRate:   d("0.0005"),
			IntervalHours: 8,
			Rate8h:        d("0.0005"),
			NextFundingAt: time.Now().Add(2 * time.Hour),
		}),
		NetRate8h:  d("0.0004"),
		OpenSpread: d("-0.10"), // negative spread (adverse: -0.10%)
		Fees: domain.ArbitrageFees{
			OpenFeesPct:  d("0.11"),
			CloseFeesPct: d("0.11"),
		},
		FullLoop: domain.FullLoopInfo{
			DurationHours: 8,
			NetPctPerLoop: d("0.0004"),
		},
	}

	req := domain.SimulationRequest{
		PositionSize: d("1000"),
		SlippagePct:  d("0"),
	}

	result := Simulate(arb, req)

	// Spread cost = 0.10% of 1000 = 1.0
	// EntryCost = spreadCost + openFees = 1.0 + 1.1 = 2.1
	assertDecimalEqual(t, "EntryCost", d("2.1"), result.EntryCost)
}

func TestSimulate_ZeroFunding_BreakevenNever(t *testing.T) {
	arb := domain.FundingArbitrage{
		Symbol: "XRP/USDT:USDT",
		Long: mergeLeg(testLegDefaults(), domain.ArbitrageLeg{
			ExchangeID:    "bybit",
			FundingRate:   d("0.0001"),
			IntervalHours: 8,
			Rate8h:        d("0.0001"),
		}),
		Short: mergeLeg(testLegDefaults(), domain.ArbitrageLeg{
			ExchangeID:    "binance",
			FundingRate:   d("0.0001"),
			IntervalHours: 8,
			Rate8h:        d("0.0001"),
		}),
		NetRate8h:  d("0"), // zero net rate
		OpenSpread: d("0"),
		Fees: domain.ArbitrageFees{
			OpenFeesPct:  d("0.11"),
			CloseFeesPct: d("0.11"),
		},
		FullLoop: domain.FullLoopInfo{
			DurationHours: 8,
			NetPctPerLoop: d("0"),
		},
	}

	req := domain.SimulationRequest{
		PositionSize: d("1000"),
		SlippagePct:  d("0.05"),
	}

	result := Simulate(arb, req)

	// Breakeven should be -1 (never).
	assertDecimalEqual(t, "BreakevenDays", d("-1"), result.BreakevenDays)
}

func TestComputeNextPaymentPct_BothAtSameTime(t *testing.T) {
	now := time.Now().Add(2 * time.Hour)
	arb := domain.FundingArbitrage{
		Long: domain.ArbitrageLeg{
			FundingRate:   d("0.0001"),
			NextFundingAt: now,
		},
		Short: domain.ArbitrageLeg{
			FundingRate:   d("0.0005"),
			NextFundingAt: now,
		},
	}

	pct := computeNextPaymentPct(arb)
	// long P&L = -0.0001, short P&L = +0.0005 => net = 0.0004
	expected := d("0.0004")
	assertDecimalEqual(t, "NextPaymentPct", expected, pct)
}

func TestComputeNextPaymentPct_LongFires_First(t *testing.T) {
	arb := domain.FundingArbitrage{
		Long: domain.ArbitrageLeg{
			FundingRate:   d("0.0001"),
			NextFundingAt: time.Now().Add(1 * time.Hour),
		},
		Short: domain.ArbitrageLeg{
			FundingRate:   d("0.0005"),
			NextFundingAt: time.Now().Add(4 * time.Hour),
		},
	}

	pct := computeNextPaymentPct(arb)
	// Only long fires: P&L = -0.0001
	expected := d("-0.0001")
	assertDecimalEqual(t, "NextPaymentPct", expected, pct)
}

func TestComputeNextPaymentPct_ShortFiresFirst(t *testing.T) {
	arb := domain.FundingArbitrage{
		Long: domain.ArbitrageLeg{
			FundingRate:   d("0.0001"),
			NextFundingAt: time.Now().Add(4 * time.Hour),
		},
		Short: domain.ArbitrageLeg{
			FundingRate:   d("0.0005"),
			NextFundingAt: time.Now().Add(1 * time.Hour),
		},
	}

	pct := computeNextPaymentPct(arb)
	// Only short fires: P&L = +0.0005
	expected := d("0.0005")
	assertDecimalEqual(t, "NextPaymentPct", expected, pct)
}

func TestSimulate_LoopPayments(t *testing.T) {
	arb := domain.FundingArbitrage{
		Symbol: "BTC/USDT:USDT",
		Long: mergeLeg(testLegDefaults(), domain.ArbitrageLeg{
			ExchangeID:    "bybit",
			FundingRate:   d("0.0001"),
			IntervalHours: 8,
			Rate8h:        d("0.0001"),
			NextFundingAt: time.Now().Add(2 * time.Hour),
		}),
		Short: mergeLeg(testLegDefaults(), domain.ArbitrageLeg{
			ExchangeID:    "binance",
			FundingRate:   d("0.0005"),
			IntervalHours: 8,
			Rate8h:        d("0.0005"),
			NextFundingAt: time.Now().Add(2 * time.Hour),
		}),
		NetRate8h:  d("0.0004"),
		OpenSpread: d("0.05"),
		Fees: domain.ArbitrageFees{
			OpenFeesPct:  d("0.11"),
			CloseFeesPct: d("0.11"),
		},
		FullLoop: domain.FullLoopInfo{
			DurationHours: 8,
			NetPctPerLoop: d("0.0004"),
			Payments: []domain.LoopPayment{
				{
					ExchangeID: "bybit",
					Side:       "long",
					Rate:       d("0.0001"),
					PaymentPct: d("-0.0001"),
					AtHour:     0,
					Cumulative: d("-0.0001"),
				},
				{
					ExchangeID: "binance",
					Side:       "short",
					Rate:       d("0.0005"),
					PaymentPct: d("0.0005"),
					AtHour:     0,
					Cumulative: d("0.0004"),
				},
			},
		},
	}

	req := domain.SimulationRequest{
		PositionSize: d("1000"),
		SlippagePct:  d("0"),
	}

	result := Simulate(arb, req)

	if len(result.LoopPayments) != 2 {
		t.Fatalf("expected 2 loop payments, got %d", len(result.LoopPayments))
	}

	// First payment: -0.0001 * 1000 = -0.1
	p0 := result.LoopPayments[0]
	assertDecimalEqual(t, "Payment[0].PaymentUsd", d("-0.1"), p0.PaymentUsd)
	assertDecimalEqual(t, "Payment[0].CumulativeUsd", d("-0.1"), p0.CumulativeUsd)
	if p0.ExchangeID != "bybit" {
		t.Errorf("Payment[0].ExchangeID: expected bybit, got %s", p0.ExchangeID)
	}
	if p0.Side != "long" {
		t.Errorf("Payment[0].Side: expected long, got %s", p0.Side)
	}

	// Second payment: 0.0005 * 1000 = 0.5
	p1 := result.LoopPayments[1]
	assertDecimalEqual(t, "Payment[1].PaymentUsd", d("0.5"), p1.PaymentUsd)
	assertDecimalEqual(t, "Payment[1].CumulativeUsd", d("0.4"), p1.CumulativeUsd)
	if p1.ExchangeID != "binance" {
		t.Errorf("Payment[1].ExchangeID: expected binance, got %s", p1.ExchangeID)
	}
}

func TestSimulate_LoopPayments_Empty(t *testing.T) {
	arb := domain.FundingArbitrage{
		Symbol: "ETH/USDT:USDT",
		Long: mergeLeg(testLegDefaults(), domain.ArbitrageLeg{
			ExchangeID:    "bybit",
			FundingRate:   d("0.0001"),
			IntervalHours: 8,
			Rate8h:        d("0.0001"),
		}),
		Short: mergeLeg(testLegDefaults(), domain.ArbitrageLeg{
			ExchangeID:    "binance",
			FundingRate:   d("0.0005"),
			IntervalHours: 8,
			Rate8h:        d("0.0005"),
		}),
		NetRate8h:  d("0.0004"),
		OpenSpread: d("0"),
		Fees: domain.ArbitrageFees{
			OpenFeesPct:  d("0.11"),
			CloseFeesPct: d("0.11"),
		},
		FullLoop: domain.FullLoopInfo{
			DurationHours: 8,
			NetPctPerLoop: d("0.0004"),
			// No payments
		},
	}

	req := domain.SimulationRequest{
		PositionSize: d("500"),
		SlippagePct:  d("0"),
	}

	result := Simulate(arb, req)

	if len(result.LoopPayments) != 0 {
		t.Fatalf("expected 0 loop payments, got %d", len(result.LoopPayments))
	}
}

// ---------------------------------------------------------------------------
// New tests: size_type and floor-rounding
// ---------------------------------------------------------------------------

func TestSimulate_UsdMode_FloorRounding(t *testing.T) {
	// Scenario: $100 budget, price=$51, contract_size=1.
	// qty = floor(100 / 51) = 1 contract.
	// actual_position_size = 1 * 51 = $51.
	arb := domain.FundingArbitrage{
		Symbol: "BTC/USDT:USDT",
		Long: domain.ArbitrageLeg{
			ExchangeID:    "bybit",
			FundingRate:   d("0.0001"),
			IntervalHours: 8,
			Rate8h:        d("0.0001"),
			Price:         d("51"),
			ContractSize:  d("1"),
		},
		Short: domain.ArbitrageLeg{
			ExchangeID:    "binance",
			FundingRate:   d("0.0005"),
			IntervalHours: 8,
			Rate8h:        d("0.0005"),
			Price:         d("51"),
			ContractSize:  d("1"),
		},
		NetRate8h:  d("0.0004"),
		OpenSpread: d("0"),
		Fees: domain.ArbitrageFees{
			OpenFeesPct:  d("0.11"),
			CloseFeesPct: d("0.11"),
		},
		FullLoop: domain.FullLoopInfo{
			DurationHours: 8,
			NetPctPerLoop: d("0.0004"),
		},
	}

	req := domain.SimulationRequest{
		SizeType:     domain.SizeTypeUSD,
		PositionSize: d("100"),
		SlippagePct:  d("0"),
	}

	result := Simulate(arb, req)

	// qty = floor(100 / 51) = 1
	assertDecimalEqual(t, "Quantity", d("1"), result.Quantity)
	// actual_position_size = 1 * 51 = 51
	assertDecimalEqual(t, "ActualPositionSize", d("51"), result.ActualPositionSize)
	// Capital = actual_position_size = 51
	assertDecimalEqual(t, "Capital", d("51"), result.Capital)
	// Funding8h = 0.0004 * 51 = 0.0204
	assertDecimalEqual(t, "Funding8h", d("0.0204"), result.Funding8h)
}

func TestSimulate_UsdMode_CantAffordSingleContract(t *testing.T) {
	// $10 budget, price=$100, contract_size=1 → can't even buy 1 contract.
	arb := domain.FundingArbitrage{
		Symbol: "BTC/USDT:USDT",
		Long: domain.ArbitrageLeg{
			ExchangeID:    "bybit",
			FundingRate:   d("0.0001"),
			IntervalHours: 8,
			Rate8h:        d("0.0001"),
			Price:         d("100"),
			ContractSize:  d("1"),
		},
		Short: domain.ArbitrageLeg{
			ExchangeID:    "binance",
			FundingRate:   d("0.0005"),
			IntervalHours: 8,
			Rate8h:        d("0.0005"),
			Price:         d("100"),
			ContractSize:  d("1"),
		},
		NetRate8h:  d("0.0004"),
		OpenSpread: d("0"),
		Fees: domain.ArbitrageFees{
			OpenFeesPct:  d("0.11"),
			CloseFeesPct: d("0.11"),
		},
		FullLoop: domain.FullLoopInfo{
			DurationHours: 8,
			NetPctPerLoop: d("0.0004"),
		},
	}

	req := domain.SimulationRequest{
		SizeType:     domain.SizeTypeUSD,
		PositionSize: d("10"),
		SlippagePct:  d("0"),
	}

	result := Simulate(arb, req)

	// 0 contracts, 0 everything.
	assertDecimalEqual(t, "Quantity", d("0"), result.Quantity)
	assertDecimalEqual(t, "ActualPositionSize", d("0"), result.ActualPositionSize)
	assertDecimalEqual(t, "Funding8h", d("0"), result.Funding8h)
}

func TestSimulate_QtyMode(t *testing.T) {
	// User provides 5 contracts, price=$200, contract_size=0.1.
	// actual_position_size = 5 * 200 * 0.1 = $100.
	arb := domain.FundingArbitrage{
		Symbol: "ETH/USDT:USDT",
		Long: domain.ArbitrageLeg{
			ExchangeID:    "bybit",
			FundingRate:   d("0.0001"),
			IntervalHours: 8,
			Rate8h:        d("0.0001"),
			Price:         d("200"),
			ContractSize:  d("0.1"),
		},
		Short: domain.ArbitrageLeg{
			ExchangeID:    "binance",
			FundingRate:   d("0.0005"),
			IntervalHours: 8,
			Rate8h:        d("0.0005"),
			Price:         d("200"),
			ContractSize:  d("0.1"),
		},
		NetRate8h:  d("0.0004"),
		OpenSpread: d("0"),
		Fees: domain.ArbitrageFees{
			OpenFeesPct:  d("0.11"),
			CloseFeesPct: d("0.11"),
		},
		FullLoop: domain.FullLoopInfo{
			DurationHours: 8,
			NetPctPerLoop: d("0.0004"),
		},
	}

	req := domain.SimulationRequest{
		SizeType:    domain.SizeTypeQty,
		Quantity:    d("5"),
		SlippagePct: d("0"),
	}

	result := Simulate(arb, req)

	assertDecimalEqual(t, "Quantity", d("5"), result.Quantity)
	// actual_position_size = 5 * 200 * 0.1 = 100
	assertDecimalEqual(t, "ActualPositionSize", d("100"), result.ActualPositionSize)
	assertDecimalEqual(t, "Capital", d("100"), result.Capital)
	// Funding8h = 0.0004 * 100 = 0.04
	assertDecimalEqual(t, "Funding8h", d("0.04"), result.Funding8h)
}

func TestSimulate_UsdMode_SmallContractSize(t *testing.T) {
	// Scenario: BTC at $50000, contract_size=0.001 (e.g. Binance linear perp min).
	// Value per contract = 50000 * 0.001 = $50.
	// $1000 budget → floor(1000/50) = 20 contracts.
	// actual_position_size = 20 * 50 = $1000.
	arb := domain.FundingArbitrage{
		Symbol: "BTC/USDT:USDT",
		Long: domain.ArbitrageLeg{
			ExchangeID:    "bybit",
			FundingRate:   d("0.0001"),
			IntervalHours: 8,
			Rate8h:        d("0.0001"),
			Price:         d("50000"),
			ContractSize:  d("0.001"),
		},
		Short: domain.ArbitrageLeg{
			ExchangeID:    "binance",
			FundingRate:   d("0.0005"),
			IntervalHours: 8,
			Rate8h:        d("0.0005"),
			Price:         d("50000"),
			ContractSize:  d("0.001"),
		},
		NetRate8h:  d("0.0004"),
		OpenSpread: d("0"),
		Fees: domain.ArbitrageFees{
			OpenFeesPct:  d("0.11"),
			CloseFeesPct: d("0.11"),
		},
		FullLoop: domain.FullLoopInfo{
			DurationHours: 8,
			NetPctPerLoop: d("0.0004"),
		},
	}

	req := domain.SimulationRequest{
		SizeType:     domain.SizeTypeUSD,
		PositionSize: d("1000"),
		SlippagePct:  d("0"),
	}

	result := Simulate(arb, req)

	assertDecimalEqual(t, "Quantity", d("20"), result.Quantity)
	assertDecimalEqual(t, "ActualPositionSize", d("1000"), result.ActualPositionSize)
}

func TestSimulate_UsdMode_PartialContract(t *testing.T) {
	// $75 budget at $50/contract → 1 contract, $50 actual (not 1.5 * 50).
	arb := domain.FundingArbitrage{
		Symbol: "TOKEN/USDT:USDT",
		Long: domain.ArbitrageLeg{
			ExchangeID:    "bybit",
			FundingRate:   d("0.0001"),
			IntervalHours: 8,
			Rate8h:        d("0.0001"),
			Price:         d("50"),
			ContractSize:  d("1"),
		},
		Short: domain.ArbitrageLeg{
			ExchangeID:    "binance",
			FundingRate:   d("0.0005"),
			IntervalHours: 8,
			Rate8h:        d("0.0005"),
			Price:         d("50"),
			ContractSize:  d("1"),
		},
		NetRate8h:  d("0.0004"),
		OpenSpread: d("0"),
		Fees: domain.ArbitrageFees{
			OpenFeesPct:  d("0"),
			CloseFeesPct: d("0"),
		},
		FullLoop: domain.FullLoopInfo{
			DurationHours: 8,
			NetPctPerLoop: d("0.0004"),
		},
	}

	req := domain.SimulationRequest{
		SizeType:     domain.SizeTypeUSD,
		PositionSize: d("75"),
		SlippagePct:  d("0"),
	}

	result := Simulate(arb, req)

	// floor(75/50) = 1 contract
	assertDecimalEqual(t, "Quantity", d("1"), result.Quantity)
	assertDecimalEqual(t, "ActualPositionSize", d("50"), result.ActualPositionSize)
}

func TestSimulate_UsdMode_DifferentContractSizes(t *testing.T) {
	// Long leg: contract_size=0.01, Short leg: contract_size=0.1.
	// resolvePositionSize picks the larger (0.1) for conservative rounding.
	// Price = $100 on both. Value per contract = 100 * 0.1 = $10.
	// $55 budget → floor(55/10) = 5 contracts. actual = 5 * 10 = $50.
	arb := domain.FundingArbitrage{
		Symbol: "TOKEN/USDT:USDT",
		Long: domain.ArbitrageLeg{
			ExchangeID:    "bybit",
			FundingRate:   d("0.0001"),
			IntervalHours: 8,
			Rate8h:        d("0.0001"),
			Price:         d("100"),
			ContractSize:  d("0.01"),
		},
		Short: domain.ArbitrageLeg{
			ExchangeID:    "binance",
			FundingRate:   d("0.0005"),
			IntervalHours: 8,
			Rate8h:        d("0.0005"),
			Price:         d("100"),
			ContractSize:  d("0.1"),
		},
		NetRate8h:  d("0.0004"),
		OpenSpread: d("0"),
		Fees: domain.ArbitrageFees{
			OpenFeesPct:  d("0"),
			CloseFeesPct: d("0"),
		},
		FullLoop: domain.FullLoopInfo{
			DurationHours: 8,
			NetPctPerLoop: d("0.0004"),
		},
	}

	req := domain.SimulationRequest{
		SizeType:     domain.SizeTypeUSD,
		PositionSize: d("55"),
		SlippagePct:  d("0"),
	}

	result := Simulate(arb, req)

	assertDecimalEqual(t, "Quantity", d("5"), result.Quantity)
	assertDecimalEqual(t, "ActualPositionSize", d("50"), result.ActualPositionSize)
}

func TestResolvePositionSize_ZeroContractSize_DefaultsToOne(t *testing.T) {
	// ContractSize=0 on both legs should default to 1.
	arb := domain.FundingArbitrage{
		Long: domain.ArbitrageLeg{
			Price: d("10"),
			// ContractSize is zero-value
		},
		Short: domain.ArbitrageLeg{
			Price: d("10"),
		},
	}

	req := domain.SimulationRequest{
		SizeType:     domain.SizeTypeUSD,
		PositionSize: d("25"),
	}

	qty, actual := resolvePositionSize(arb, req)

	// floor(25/10) = 2
	assertDecimalEqual(t, "qty", d("2"), qty)
	assertDecimalEqual(t, "actual", d("20"), actual)
}

// --- helpers ---

func assertDecimalEqual(t *testing.T, name string, expected, actual decimal.Decimal) {
	t.Helper()
	if !expected.Equal(actual) {
		t.Errorf("%s: expected %s, got %s", name, expected, actual)
	}
}

func assertDecimalClose(t *testing.T, name string, expected, actual, tolerance decimal.Decimal) {
	t.Helper()
	diff := expected.Sub(actual).Abs()
	if diff.GreaterThan(tolerance) {
		t.Errorf("%s: expected %s (±%s), got %s (diff=%s)", name, expected, tolerance, actual, diff)
	}
}
