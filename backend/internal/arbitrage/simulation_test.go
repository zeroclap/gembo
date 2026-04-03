package arbitrage

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/zeroclap/gembo/internal/domain"
)

// testLegDefaults returns an ArbitrageLeg with Price=1, ContractSize=1 so
// that USD-mode floor-rounding is a no-op ($1000 / $1/contract = 1000 contracts).
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
	if !override.TakerFeePct.IsZero() {
		base.TakerFeePct = override.TakerFeePct
	}
	return base
}

func TestSimulate_BasicCalculation(t *testing.T) {
	now := time.Now().UTC()
	arb := domain.FundingArbitrage{
		Symbol: "BTC/USDT:USDT",
		Long: mergeLeg(testLegDefaults(), domain.ArbitrageLeg{
			ExchangeID:    "bybit",
			FundingRate:   d("0.0001"),
			IntervalHours: 8,
			Rate8h:        d("0.0001"),
			NextFundingAt: now.Add(2 * time.Hour),
		}),
		Short: mergeLeg(testLegDefaults(), domain.ArbitrageLeg{
			ExchangeID:    "binance",
			FundingRate:   d("0.0005"),
			IntervalHours: 8,
			Rate8h:        d("0.0005"),
			NextFundingAt: now.Add(2 * time.Hour),
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

	result := Simulate(arb, req, now)

	// Position size and capital should match.
	assertDecimalEqual(t, "PositionSize", d("1000"), result.PositionSize)
	assertDecimalEqual(t, "Capital", d("1000"), result.Capital)
	assertDecimalEqual(t, "Quantity", d("1000"), result.Quantity)
	assertDecimalEqual(t, "ActualPositionSize", d("1000"), result.ActualPositionSize)

	// Funding P&L: 0.0004 * 1000 = 0.4 per 8h.
	assertDecimalEqual(t, "Funding8h", d("0.4"), result.Funding8h)

	// Funding24h: 0.4 * 3 = 1.2 per day.
	// With default hold=24h, FundingHold = 0.4 * (24/8) = 1.2.
	assertDecimalEqual(t, "FundingHold", d("1.2"), result.FundingHold)

	// LCM P&L: 0.0004 * 1000 = 0.4.
	assertDecimalEqual(t, "LcmPnL", d("0.4"), result.LcmPnL)

	// Open spread is positive, so no spread cost.
	// EntryCost = openFees + slippage = (0.11/100)*1000 + (0.05/100)*1000 = 1.1 + 0.5 = 1.6
	assertDecimalEqual(t, "EntryCost", d("1.6"), result.EntryCost)

	// ExitCost = closeFees + slippage = (0.11/100)*1000 + (0.05/100)*1000 = 1.1 + 0.5 = 1.6
	assertDecimalEqual(t, "ExitCost", d("1.6"), result.ExitCost)

	// TotalCost = 1.6 + 1.6 = 3.2
	assertDecimalEqual(t, "TotalCost", d("3.2"), result.TotalCost)

	// Breakeven: 3.2 / 1.2 ~ 2.666...
	expectedBreakeven := d("3.2").Div(d("1.2"))
	assertDecimalClose(t, "BreakevenDays", expectedBreakeven, result.BreakevenDays, d("0.001"))
}

func TestSimulate_NegativeSpread(t *testing.T) {
	now := time.Now().UTC()
	arb := domain.FundingArbitrage{
		Symbol: "ETH/USDT:USDT",
		Long: mergeLeg(testLegDefaults(), domain.ArbitrageLeg{
			ExchangeID:    "bybit",
			FundingRate:   d("0.0001"),
			IntervalHours: 8,
			Rate8h:        d("0.0001"),
			NextFundingAt: now.Add(2 * time.Hour),
		}),
		Short: mergeLeg(testLegDefaults(), domain.ArbitrageLeg{
			ExchangeID:    "binance",
			FundingRate:   d("0.0005"),
			IntervalHours: 8,
			Rate8h:        d("0.0005"),
			NextFundingAt: now.Add(2 * time.Hour),
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

	result := Simulate(arb, req, now)

	// Spread cost = 0.10% of 1000 = 1.0
	// EntryCost = spreadCost + openFees = 1.0 + 1.1 = 2.1
	assertDecimalEqual(t, "EntryCost", d("2.1"), result.EntryCost)
}

func TestSimulate_ZeroFunding_BreakevenNever(t *testing.T) {
	now := time.Now().UTC()
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

	result := Simulate(arb, req, now)

	// Breakeven should be -1 (never).
	assertDecimalEqual(t, "BreakevenDays", d("-1"), result.BreakevenDays)
}

func TestComputeNextPaymentPct_BothAtSameTime(t *testing.T) {
	futureTime := time.Now().Add(2 * time.Hour)
	arb := domain.FundingArbitrage{
		Long: domain.ArbitrageLeg{
			FundingRate:   d("0.0001"),
			NextFundingAt: futureTime,
		},
		Short: domain.ArbitrageLeg{
			FundingRate:   d("0.0005"),
			NextFundingAt: futureTime,
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
	now := time.Now().UTC()
	arb := domain.FundingArbitrage{
		Symbol: "BTC/USDT:USDT",
		Long: mergeLeg(testLegDefaults(), domain.ArbitrageLeg{
			ExchangeID:    "bybit",
			FundingRate:   d("0.0001"),
			IntervalHours: 8,
			Rate8h:        d("0.0001"),
			NextFundingAt: now.Add(2 * time.Hour),
		}),
		Short: mergeLeg(testLegDefaults(), domain.ArbitrageLeg{
			ExchangeID:    "binance",
			FundingRate:   d("0.0005"),
			IntervalHours: 8,
			Rate8h:        d("0.0005"),
			NextFundingAt: now.Add(2 * time.Hour),
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

	result := Simulate(arb, req, now)

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
	now := time.Now().UTC()
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

	result := Simulate(arb, req, now)

	if len(result.LoopPayments) != 0 {
		t.Fatalf("expected 0 loop payments, got %d", len(result.LoopPayments))
	}
}

// ---------------------------------------------------------------------------
// New tests: size_type and floor-rounding
// ---------------------------------------------------------------------------

func TestSimulate_UsdMode_FloorRounding(t *testing.T) {
	now := time.Now().UTC()
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

	result := Simulate(arb, req, now)

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
	now := time.Now().UTC()
	// $10 budget, price=$100, contract_size=1 -> can't even buy 1 contract.
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

	result := Simulate(arb, req, now)

	// 0 contracts, 0 everything.
	assertDecimalEqual(t, "Quantity", d("0"), result.Quantity)
	assertDecimalEqual(t, "ActualPositionSize", d("0"), result.ActualPositionSize)
	assertDecimalEqual(t, "Funding8h", d("0"), result.Funding8h)
}

func TestSimulate_QtyMode(t *testing.T) {
	now := time.Now().UTC()
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

	result := Simulate(arb, req, now)

	assertDecimalEqual(t, "Quantity", d("5"), result.Quantity)
	// actual_position_size = 5 * 200 * 0.1 = 100
	assertDecimalEqual(t, "ActualPositionSize", d("100"), result.ActualPositionSize)
	assertDecimalEqual(t, "Capital", d("100"), result.Capital)
	// Funding8h = 0.0004 * 100 = 0.04
	assertDecimalEqual(t, "Funding8h", d("0.04"), result.Funding8h)
}

func TestSimulate_UsdMode_SmallContractSize(t *testing.T) {
	now := time.Now().UTC()
	// Scenario: BTC at $50000, contract_size=0.001 (e.g. Binance linear perp min).
	// Value per contract = 50000 * 0.001 = $50.
	// $1000 budget -> floor(1000/50) = 20 contracts.
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

	result := Simulate(arb, req, now)

	assertDecimalEqual(t, "Quantity", d("20"), result.Quantity)
	assertDecimalEqual(t, "ActualPositionSize", d("1000"), result.ActualPositionSize)
}

func TestSimulate_UsdMode_PartialContract(t *testing.T) {
	now := time.Now().UTC()
	// $75 budget at $50/contract -> 1 contract, $50 actual (not 1.5 * 50).
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

	result := Simulate(arb, req, now)

	// floor(75/50) = 1 contract
	assertDecimalEqual(t, "Quantity", d("1"), result.Quantity)
	assertDecimalEqual(t, "ActualPositionSize", d("50"), result.ActualPositionSize)
}

func TestSimulate_UsdMode_DifferentContractSizes(t *testing.T) {
	now := time.Now().UTC()
	// Long leg: contract_size=0.01, Short leg: contract_size=0.1.
	// resolvePositionSize picks the larger (0.1) for conservative rounding.
	// Price = $100 on both. Value per contract = 100 * 0.1 = $10.
	// $55 budget -> floor(55/10) = 5 contracts. actual = 5 * 10 = $50.
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

	result := Simulate(arb, req, now)

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

// ---------------------------------------------------------------------------
// New tests: CleanLoop and HoldSimulation
// ---------------------------------------------------------------------------

func TestSimulate_CleanLoop_HasOpenAndCloseRows(t *testing.T) {
	now := time.Now().UTC()
	arb := domain.FundingArbitrage{
		Symbol: "BTC/USDT:USDT",
		Long: mergeLeg(testLegDefaults(), domain.ArbitrageLeg{
			ExchangeID:    "bybit",
			FundingRate:   d("0.0001"),
			IntervalHours: 8,
			Rate8h:        d("0.0001"),
			NextFundingAt: now.Add(2 * time.Hour),
			TakerFeePct:   d("0.055"),
		}),
		Short: mergeLeg(testLegDefaults(), domain.ArbitrageLeg{
			ExchangeID:    "binance",
			FundingRate:   d("0.0005"),
			IntervalHours: 8,
			Rate8h:        d("0.0005"),
			NextFundingAt: now.Add(2 * time.Hour),
			TakerFeePct:   d("0.055"),
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
			Payments: []domain.LoopPayment{
				{ExchangeID: "binance", Side: "short", Rate: d("0.0005"), PaymentPct: d("0.0005"), AtHour: 8, Cumulative: d("0.0005")},
				{ExchangeID: "bybit", Side: "long", Rate: d("0.0001"), PaymentPct: d("-0.0001"), AtHour: 8, Cumulative: d("0.0004")},
			},
		},
	}

	req := domain.SimulationRequest{
		PositionSize: d("1000"),
		SlippagePct:  d("0"),
	}

	result := Simulate(arb, req, now)
	cl := result.CleanLoop

	// Both funding payments are at the same AtHour=8, so they merge into
	// one parent row: open + 1 merged funding + close = 3 rows.
	if len(cl.Rows) != 3 {
		t.Fatalf("CleanLoop: expected 3 rows, got %d", len(cl.Rows))
	}

	// First row: open fees.
	if cl.Rows[0].Kind != domain.RowKindOpen {
		t.Errorf("CleanLoop row 0: expected kind 'open', got '%s'", cl.Rows[0].Kind)
	}
	if cl.Rows[0].Label != "Open fees" {
		t.Errorf("CleanLoop row 0: expected label 'Open fees', got '%s'", cl.Rows[0].Label)
	}
	// Open fees = -(0.11/100)*1000 = -1.1
	assertDecimalEqual(t, "CleanLoop[0].PnlUsd", d("-1.1"), cl.Rows[0].PnlUsd)
	// Open row should have fee events (long + short).
	if len(cl.Rows[0].Events) != 2 {
		t.Errorf("CleanLoop row 0: expected 2 fee events, got %d", len(cl.Rows[0].Events))
	}
	// Fee events should carry the taker fee rate as a fraction.
	assertDecimalEqual(t, "CleanLoop[0].Events[0].Rate (long fee)", d("0.00055"), cl.Rows[0].Events[0].Rate)
	assertDecimalEqual(t, "CleanLoop[0].Events[1].Rate (short fee)", d("0.00055"), cl.Rows[0].Events[1].Rate)

	// Middle row: merged funding (has events for both exchanges).
	if cl.Rows[1].Kind != domain.RowKindFunding {
		t.Errorf("CleanLoop row 1: expected kind 'funding', got '%s'", cl.Rows[1].Kind)
	}
	// Merged funding: -0.0001 + 0.0005 = 0.0004 ratio -> USD: 0.4
	assertDecimalClose(t, "CleanLoop[1].PnlUsd (merged)", d("0.4"), cl.Rows[1].PnlUsd, d("0.001"))
	if len(cl.Rows[1].Events) != 2 {
		t.Errorf("CleanLoop row 1: expected 2 events for merged funding, got %d", len(cl.Rows[1].Events))
	}

	// Last row: close fees.
	last := cl.Rows[len(cl.Rows)-1]
	if last.Kind != domain.RowKindClose {
		t.Errorf("CleanLoop last row: expected kind 'close', got '%s'", last.Kind)
	}
	if last.Label != "Close fees" {
		t.Errorf("CleanLoop last row: expected label 'Close fees', got '%s'", last.Label)
	}
	// Close row should have fee events (long + short).
	if len(last.Events) != 2 {
		t.Errorf("CleanLoop last row: expected 2 fee events, got %d", len(last.Events))
	}
	// Close fee events should also carry the taker fee rate.
	assertDecimalEqual(t, "CleanLoop[last].Events[0].Rate (long fee)", d("0.00055"), last.Events[0].Rate)
	assertDecimalEqual(t, "CleanLoop[last].Events[1].Rate (short fee)", d("0.00055"), last.Events[1].Rate)

	// Duration should be 8h.
	assertDecimalEqual(t, "CleanLoop.DurationHours", d("8"), cl.DurationHours)
}

func TestSimulate_CleanLoop_CumulativeIncludesFees(t *testing.T) {
	now := time.Now().UTC()
	arb := domain.FundingArbitrage{
		Symbol: "BTC/USDT:USDT",
		Long: mergeLeg(testLegDefaults(), domain.ArbitrageLeg{
			ExchangeID:    "bybit",
			FundingRate:   d("0.0001"),
			IntervalHours: 8,
			Rate8h:        d("0.0001"),
			NextFundingAt: now.Add(2 * time.Hour),
			TakerFeePct:   d("0.05"),
		}),
		Short: mergeLeg(testLegDefaults(), domain.ArbitrageLeg{
			ExchangeID:    "binance",
			FundingRate:   d("0.0005"),
			IntervalHours: 8,
			Rate8h:        d("0.0005"),
			NextFundingAt: now.Add(2 * time.Hour),
			TakerFeePct:   d("0.05"),
		}),
		NetRate8h:  d("0.0004"),
		OpenSpread: d("0"),
		Fees: domain.ArbitrageFees{
			OpenFeesPct:  d("0.10"), // 0.10%
			CloseFeesPct: d("0.10"),
		},
		FullLoop: domain.FullLoopInfo{
			DurationHours: 8,
			NetPctPerLoop: d("0.0004"),
			Payments: []domain.LoopPayment{
				{ExchangeID: "bybit", Side: "long", Rate: d("0.0001"), PaymentPct: d("-0.0001"), AtHour: 8, Cumulative: d("-0.0001")},
				{ExchangeID: "binance", Side: "short", Rate: d("0.0005"), PaymentPct: d("0.0005"), AtHour: 8, Cumulative: d("0.0004")},
			},
		},
	}

	req := domain.SimulationRequest{
		PositionSize: d("10000"),
		SlippagePct:  d("0"),
	}

	result := Simulate(arb, req, now)
	cl := result.CleanLoop

	// Both funding payments are at AtHour=8, so they merge:
	// open + 1 merged funding + close = 3 rows.
	if len(cl.Rows) != 3 {
		t.Fatalf("CleanLoop: expected 3 rows, got %d", len(cl.Rows))
	}

	// Open fees: -(0.10/100)*10000 = -10 USD
	assertDecimalEqual(t, "CleanLoop[0].CumulativeUsd (open)", d("-10"), cl.Rows[0].CumulativeUsd)

	// Merged funding: (-0.0001 + 0.0005) * 10000 = 4 USD -> cumulative = -10 + 4 = -6
	assertDecimalEqual(t, "CleanLoop[1].CumulativeUsd (merged funding)", d("-6"), cl.Rows[1].CumulativeUsd)
	// Verify merged row has events.
	if len(cl.Rows[1].Events) != 2 {
		t.Errorf("CleanLoop[1]: expected 2 events for merged funding, got %d", len(cl.Rows[1].Events))
	}

	// Close fees: -(0.10/100)*10000 = -10 -> cumulative = -6 + (-10) = -16
	assertDecimalEqual(t, "CleanLoop[2].CumulativeUsd (close)", d("-16"), cl.Rows[2].CumulativeUsd)
	assertDecimalEqual(t, "CleanLoop.NetPnlUsd", d("-16"), cl.NetPnlUsd)
}

func TestSimulate_HoldSimulation_ProjectsFundingTimes(t *testing.T) {
	// Set up: now is T, long next funding at T+30min, short at T+2h.
	// Both have 8h intervals. Hold = 30h.
	// Expected long payments at: T+0.5h, T+8.5h, T+16.5h, T+24.5h (4 payments).
	// Expected short payments at: T+2h, T+10h, T+18h, T+26h (4 payments).
	// No simultaneous events, so 8 individual funding rows + open + close = 10 rows.
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	arb := domain.FundingArbitrage{
		Symbol: "BTC/USDT:USDT",
		Long: mergeLeg(testLegDefaults(), domain.ArbitrageLeg{
			ExchangeID:    "bybit",
			FundingRate:   d("0.0001"),
			IntervalHours: 8,
			Rate8h:        d("0.0001"),
			NextFundingAt: now.Add(30 * time.Minute),
			TakerFeePct:   d("0.05"),
		}),
		Short: mergeLeg(testLegDefaults(), domain.ArbitrageLeg{
			ExchangeID:    "binance",
			FundingRate:   d("0.0005"),
			IntervalHours: 8,
			Rate8h:        d("0.0005"),
			NextFundingAt: now.Add(2 * time.Hour),
			TakerFeePct:   d("0.05"),
		}),
		NetRate8h:  d("0.0004"),
		OpenSpread: d("0"),
		Fees: domain.ArbitrageFees{
			OpenFeesPct:  d("0.10"),
			CloseFeesPct: d("0.10"),
		},
		FullLoop: domain.FullLoopInfo{
			DurationHours: 8,
			NetPctPerLoop: d("0.0004"),
		},
	}

	req := domain.SimulationRequest{
		PositionSize: d("1000"),
		SlippagePct:  d("0"),
		HoldHours:    d("30"),
	}

	result := Simulate(arb, req, now)
	hs := result.HoldSimulation

	// Duration should be 30h.
	assertDecimalEqual(t, "HoldSim.DurationHours", d("30"), hs.DurationHours)

	// Count row types.
	var openCount, fundingCount, closeCount int
	for _, row := range hs.Rows {
		switch row.Kind {
		case domain.RowKindOpen:
			openCount++
		case domain.RowKindFunding:
			fundingCount++
		case domain.RowKindClose:
			closeCount++
		}
	}

	if openCount != 1 {
		t.Errorf("HoldSim: expected 1 open row, got %d", openCount)
	}
	if closeCount != 1 {
		t.Errorf("HoldSim: expected 1 close row, got %d", closeCount)
	}
	// Long: T+0.5h, T+8.5h, T+16.5h, T+24.5h = 4
	// Short: T+2h, T+10h, T+18h, T+26h = 4
	// No simultaneous events -> 8 individual rows.
	if fundingCount != 8 {
		t.Errorf("HoldSim: expected 8 funding rows, got %d", fundingCount)
	}

	// First row should be open with fee sub-rows.
	if hs.Rows[0].Kind != domain.RowKindOpen {
		t.Errorf("HoldSim first row: expected 'open', got '%s'", hs.Rows[0].Kind)
	}
	if hs.Rows[0].At == nil {
		t.Error("HoldSim first row: expected absolute timestamp")
	}
	if len(hs.Rows[0].Events) != 2 {
		t.Errorf("HoldSim first row: expected 2 fee events, got %d", len(hs.Rows[0].Events))
	}
	// Fee events should carry the taker fee rate as a fraction.
	assertDecimalEqual(t, "HoldSim[0].Events[0].Rate (long fee)", d("0.0005"), hs.Rows[0].Events[0].Rate)
	assertDecimalEqual(t, "HoldSim[0].Events[1].Rate (short fee)", d("0.0005"), hs.Rows[0].Events[1].Rate)

	// Last row should be close with fee sub-rows.
	lastRow := hs.Rows[len(hs.Rows)-1]
	if lastRow.Kind != domain.RowKindClose {
		t.Errorf("HoldSim last row: expected 'close', got '%s'", lastRow.Kind)
	}
	if len(lastRow.Events) != 2 {
		t.Errorf("HoldSim last row: expected 2 fee events, got %d", len(lastRow.Events))
	}
	assertDecimalEqual(t, "HoldSim[last].Events[0].Rate (long fee)", d("0.0005"), lastRow.Events[0].Rate)
	assertDecimalEqual(t, "HoldSim[last].Events[1].Rate (short fee)", d("0.0005"), lastRow.Events[1].Rate)

	// Check that funding rows have absolute timestamps and are ordered.
	// Also verify every row has at least one event.
	var prevHour decimal.Decimal
	for i, row := range hs.Rows {
		if i > 0 && row.AtHour.LessThan(prevHour) {
			t.Errorf("HoldSim: rows not ordered by hour at index %d: %s < %s", i, row.AtHour, prevHour)
		}
		prevHour = row.AtHour
		if row.Kind == domain.RowKindFunding && row.At == nil {
			t.Errorf("HoldSim: funding row %d missing absolute timestamp", i)
		}
		if len(row.Events) == 0 {
			t.Errorf("HoldSim: row %d has no events", i)
		}
	}
}

func TestSimulate_HoldSimulation_DefaultHoldHours(t *testing.T) {
	now := time.Now().UTC()
	arb := domain.FundingArbitrage{
		Symbol: "BTC/USDT:USDT",
		Long: mergeLeg(testLegDefaults(), domain.ArbitrageLeg{
			ExchangeID:    "bybit",
			FundingRate:   d("0.0001"),
			IntervalHours: 8,
			Rate8h:        d("0.0001"),
			NextFundingAt: now.Add(2 * time.Hour),
		}),
		Short: mergeLeg(testLegDefaults(), domain.ArbitrageLeg{
			ExchangeID:    "binance",
			FundingRate:   d("0.0005"),
			IntervalHours: 8,
			Rate8h:        d("0.0005"),
			NextFundingAt: now.Add(2 * time.Hour),
		}),
		NetRate8h:  d("0.0004"),
		OpenSpread: d("0"),
		Fees: domain.ArbitrageFees{
			OpenFeesPct:  d("0.10"),
			CloseFeesPct: d("0.10"),
		},
		FullLoop: domain.FullLoopInfo{
			DurationHours: 8,
			NetPctPerLoop: d("0.0004"),
		},
	}

	// No HoldHours specified -> should default to 24h.
	req := domain.SimulationRequest{
		PositionSize: d("1000"),
		SlippagePct:  d("0"),
	}

	result := Simulate(arb, req, now)
	assertDecimalEqual(t, "HoldSim.DurationHours default", d("24"), result.HoldSimulation.DurationHours)
}

func TestSimulate_HoldSimulation_DifferentIntervals(t *testing.T) {
	// One leg pays every 4h, other every 8h. Hold 16h.
	// Long (4h): T+1h, T+5h, T+9h, T+13h (4 payments)
	// Short (8h): T+3h, T+11h (2 payments)
	// Total: 6 funding + open + close = 8 rows.
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	arb := domain.FundingArbitrage{
		Symbol: "ETH/USDT:USDT",
		Long: mergeLeg(testLegDefaults(), domain.ArbitrageLeg{
			ExchangeID:    "bybit",
			FundingRate:   d("0.0001"),
			IntervalHours: 4,
			Rate8h:        d("0.0002"),
			NextFundingAt: now.Add(1 * time.Hour),
		}),
		Short: mergeLeg(testLegDefaults(), domain.ArbitrageLeg{
			ExchangeID:    "binance",
			FundingRate:   d("0.0005"),
			IntervalHours: 8,
			Rate8h:        d("0.0005"),
			NextFundingAt: now.Add(3 * time.Hour),
		}),
		NetRate8h:  d("0.0003"),
		OpenSpread: d("0"),
		Fees: domain.ArbitrageFees{
			OpenFeesPct:  d("0"),
			CloseFeesPct: d("0"),
		},
		FullLoop: domain.FullLoopInfo{
			DurationHours: 8,
			NetPctPerLoop: d("0.0003"),
		},
	}

	req := domain.SimulationRequest{
		PositionSize: d("1000"),
		SlippagePct:  d("0"),
		HoldHours:    d("16"),
	}

	result := Simulate(arb, req, now)
	hs := result.HoldSimulation

	var fundingCount int
	for _, row := range hs.Rows {
		if row.Kind == domain.RowKindFunding {
			fundingCount++
		}
	}
	if fundingCount != 6 {
		t.Errorf("HoldSim different intervals: expected 6 funding rows, got %d", fundingCount)
	}
}

func TestSimulate_HoldSimulation_NextFundingInPast(t *testing.T) {
	// Simulate scenario where next_funding_at is in the past relative to now.
	// The hold simulation should advance to the next future occurrence.
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	pastFunding := time.Date(2025, 1, 1, 8, 0, 0, 0, time.UTC) // 4h ago

	arb := domain.FundingArbitrage{
		Symbol: "BTC/USDT:USDT",
		Long: mergeLeg(testLegDefaults(), domain.ArbitrageLeg{
			ExchangeID:    "bybit",
			FundingRate:   d("0.0001"),
			IntervalHours: 8,
			Rate8h:        d("0.0001"),
			NextFundingAt: pastFunding,
		}),
		Short: mergeLeg(testLegDefaults(), domain.ArbitrageLeg{
			ExchangeID:    "binance",
			FundingRate:   d("0.0005"),
			IntervalHours: 8,
			Rate8h:        d("0.0005"),
			NextFundingAt: pastFunding,
		}),
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
		PositionSize: d("1000"),
		SlippagePct:  d("0"),
		HoldHours:    d("10"),
	}

	result := Simulate(arb, req, now)

	// Next funding should be at T+4h (pastFunding + 8h = 16:00).
	// Hold ends at T+10h = 22:00.
	// Both legs fire at the same time (T+4h), so they merge into 1 funding row.
	var fundingCount int
	for _, row := range result.HoldSimulation.Rows {
		if row.Kind == domain.RowKindFunding {
			fundingCount++
			// The merged funding row should be at 4h from now.
			assertDecimalClose(t, "HoldSim past funding AtHour", d("4"), row.AtHour, d("0.01"))
			// Should have 2 events (one long, one short).
			if len(row.Events) != 2 {
				t.Errorf("HoldSim past funding: expected 2 events, got %d", len(row.Events))
			}
		}
	}
	if fundingCount != 1 { // one merged row (long + short at same time)
		t.Errorf("HoldSim past funding: expected 1 merged funding row, got %d", fundingCount)
	}
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
		t.Errorf("%s: expected %s (+/-%s), got %s (diff=%s)", name, expected, tolerance, actual, diff)
	}
}
