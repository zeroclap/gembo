package arbitrage

import (
	"fmt"
	"math"
	"sort"
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
// touch the store.
func Simulate(arb domain.FundingArbitrage, req domain.SimulationRequest, now time.Time) domain.SimulationResult {
	qty, actualSize := resolvePositionSize(arb, req)

	size := actualSize
	slippage := req.SlippagePct
	capital := size

	// Per-leg fee percentages (already in percent, e.g. 0.055).
	longFeePct := arb.Long.TakerFeePct
	shortFeePct := arb.Short.TakerFeePct

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

	// Total open cost as percentage of position size (negative = cost).
	openCostPct := decimal.Zero
	if size.IsPositive() {
		openCostPct = entryCost.Div(size).Neg()
	}
	closeCostPct := decimal.Zero
	if size.IsPositive() {
		closeCostPct = exitCost.Div(size).Neg()
	}

	// Per-exchange fee details for sub-rows.
	halfSlippage := slippage.Div(dec2) // split slippage evenly between legs
	feeDetail := feeBreakdown{
		longExchangeID:  arb.Long.ExchangeID,
		shortExchangeID: arb.Short.ExchangeID,
		longFeePct:      longFeePct,
		shortFeePct:     shortFeePct,
		halfSlippagePct: halfSlippage,
		spreadCostPct:   spreadCostPct,
		size:            size,
	}

	// Funding P&L per 8h period (net_rate_8h is a ratio, e.g. 0.0001).
	netRate8h := arb.NetRate8h
	funding8h := netRate8h.Mul(size)

	// Resolve hold hours (default 24h).
	holdHours := req.HoldHours
	if holdHours.IsZero() || holdHours.IsNegative() {
		holdHours = decimal.NewFromInt(24)
	}

	// Funding P&L over the hold period: funding8h * (holdHours / 8).
	dec8 := decimal.NewFromInt(8)
	fundingHold := funding8h.Mul(holdHours).Div(dec8)

	// Funding per 24h (used for breakeven calculation).
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

	// Hold period P&L = fundingHold.
	pnlHold := fundingHold

	// Build backward-compatible per-payment USD breakdown for the full loop.
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

	// Build clean loop simulation (theoretical LCM-based).
	cleanLoop := buildCleanLoop(arb, size, openCostPct, entryCost, closeCostPct, exitCost, feeDetail)

	// Build hold-based simulation (time-aware).
	holdSim := buildHoldSimulation(arb, size, openCostPct, entryCost, closeCostPct, exitCost, holdHours, now, feeDetail)

	return domain.SimulationResult{
		PositionSize:       req.PositionSize,
		ActualPositionSize: actualSize,
		Quantity:           qty,
		Capital:            capital,
		EntryCost:          entryCost,
		ExitCost:           exitCost,
		TotalCost:          totalCost,
		Funding8h:          funding8h,
		FundingHold:        fundingHold,
		HoldHours:          holdHours,
		BreakevenDays:      breakevenDays,
		LcmPnL:             lcmPnL,
		NextPnL:            nextPnL,
		PnlHold:            pnlHold,
		CleanLoop:          cleanLoop,
		HoldSimulation:     holdSim,
		LoopPayments:       loopPayments,
	}
}

// feeBreakdown holds per-exchange fee info for building sub-rows.
type feeBreakdown struct {
	longExchangeID  string
	shortExchangeID string
	longFeePct      decimal.Decimal // long leg taker fee (percent, e.g. 0.055)
	shortFeePct     decimal.Decimal // short leg taker fee (percent)
	halfSlippagePct decimal.Decimal // half the user-specified slippage (percent)
	spreadCostPct   decimal.Decimal // adverse spread cost (percent, 0 if favorable)
	size            decimal.Decimal
}

// buildFeeSubRows creates per-exchange sub-details for an open or close fee row.
// For open: long fee = takerFee + halfSlippage + spread, short fee = takerFee + halfSlippage.
// For close: both legs = takerFee + halfSlippage.
func buildFeeEvents(kind domain.SimulationRowKind, fd feeBreakdown) []domain.SimulationEvent {
	// Long leg cost %.
	longPct := fd.longFeePct.Add(fd.halfSlippagePct)
	if kind == domain.RowKindOpen {
		longPct = longPct.Add(fd.spreadCostPct) // spread attributed to long (open only)
	}
	longPctNeg := longPct.Div(dec100).Neg()
	longUsd := longPctNeg.Mul(fd.size)

	// Short leg cost %.
	shortPct := fd.shortFeePct.Add(fd.halfSlippagePct)
	shortPctNeg := shortPct.Div(dec100).Neg()
	shortUsd := shortPctNeg.Mul(fd.size)

	return []domain.SimulationEvent{
		{
			ExchangeID: fd.longExchangeID,
			Side:       domain.PositionSideLong,
			Rate:       fd.longFeePct.Div(dec100),
			PnlPct:     longPctNeg,
			PnlUsd:     longUsd,
		},
		{
			ExchangeID: fd.shortExchangeID,
			Side:       domain.PositionSideShort,
			Rate:       fd.shortFeePct.Div(dec100),
			PnlPct:     shortPctNeg,
			PnlUsd:     shortUsd,
		},
	}
}

// fundingItem is an intermediate representation of a single funding event
// before it gets grouped into SimulationRow with Events.
type fundingItem struct {
	label  string
	atHour decimal.Decimal
	at     *time.Time
	event  domain.SimulationEvent
}

// mergeFundingItems groups sorted funding items by AtHour and produces
// SimulationRows. Every row always has Events (1 for single-leg, 2+ for
// simultaneous events). The parent row carries aggregate PnlPct/PnlUsd.
func mergeFundingItems(items []fundingItem) []domain.SimulationRow {
	if len(items) == 0 {
		return nil
	}

	var merged []domain.SimulationRow
	i := 0
	for i < len(items) {
		// Find all items with the same AtHour.
		j := i + 1
		for j < len(items) && items[j].atHour.Equal(items[i].atHour) {
			j++
		}

		group := items[i:j]
		totalPnlPct := decimal.Zero
		totalPnlUsd := decimal.Zero
		events := make([]domain.SimulationEvent, 0, len(group))
		for _, it := range group {
			totalPnlPct = totalPnlPct.Add(it.event.PnlPct)
			totalPnlUsd = totalPnlUsd.Add(it.event.PnlUsd)
			events = append(events, it.event)
		}

		merged = append(merged, domain.SimulationRow{
			Kind:   domain.RowKindFunding,
			Label:  group[0].label,
			AtHour: group[0].atHour,
			PnlPct: totalPnlPct,
			PnlUsd: totalPnlUsd,
			At:     group[0].at,
			Events: events,
		})

		i = j
	}
	return merged
}

// buildCleanLoop creates the theoretical LCM-based loop simulation with
// open fee row at the start and close fee row at the end.
// Simultaneous funding events are merged into a single row with sub-details.
func buildCleanLoop(
	arb domain.FundingArbitrage,
	size decimal.Decimal,
	openCostPct, entryCostUsd decimal.Decimal,
	closeCostPct, exitCostUsd decimal.Decimal,
	fd feeBreakdown,
) domain.LoopSimulation {
	durationHours := decimal.NewFromInt(int64(arb.FullLoop.DurationHours))

	var rows []domain.SimulationRow
	cumPct := decimal.Zero
	cumUsd := decimal.Zero

	// Row 0: Open fees with per-exchange events.
	cumPct = cumPct.Add(openCostPct)
	cumUsd = cumUsd.Add(entryCostUsd.Neg())
	rows = append(rows, domain.SimulationRow{
		Kind:          domain.RowKindOpen,
		Label:         "Open fees",
		AtHour:        decimal.Zero,
		PnlPct:        openCostPct,
		PnlUsd:        entryCostUsd.Neg(),
		CumulativePct: cumPct,
		CumulativeUsd: cumUsd,
		Events:        buildFeeEvents(domain.RowKindOpen, fd),
	})

	// Build individual funding items from the full loop, then merge.
	var fundingItems []fundingItem
	for _, p := range arb.FullLoop.Payments {
		pnlUsd := p.PaymentPct.Mul(size)
		fundingItems = append(fundingItems, fundingItem{
			label:  fmt.Sprintf("%dh", p.AtHour),
			atHour: decimal.NewFromInt(int64(p.AtHour)),
			event: domain.SimulationEvent{
				ExchangeID: p.ExchangeID,
				Side:       p.Side,
				Rate:       p.Rate,
				PnlPct:     p.PaymentPct,
				PnlUsd:     pnlUsd,
			},
		})
	}

	for _, row := range mergeFundingItems(fundingItems) {
		cumPct = cumPct.Add(row.PnlPct)
		cumUsd = cumUsd.Add(row.PnlUsd)
		row.CumulativePct = cumPct
		row.CumulativeUsd = cumUsd
		rows = append(rows, row)
	}

	// Last row: Close fees with per-exchange events.
	cumPct = cumPct.Add(closeCostPct)
	cumUsd = cumUsd.Add(exitCostUsd.Neg())
	rows = append(rows, domain.SimulationRow{
		Kind:          domain.RowKindClose,
		Label:         "Close fees",
		AtHour:        durationHours,
		PnlPct:        closeCostPct,
		PnlUsd:        exitCostUsd.Neg(),
		CumulativePct: cumPct,
		CumulativeUsd: cumUsd,
		Events:        buildFeeEvents(domain.RowKindClose, fd),
	})

	return domain.LoopSimulation{
		DurationHours: durationHours,
		Rows:          rows,
		NetPnlPct:     cumPct,
		NetPnlUsd:     cumUsd,
	}
}

// buildHoldSimulation creates a time-aware simulation for a given hold period.
// It projects forward from `now`, finding all funding payment times within
// [now, now+holdHours] for both legs, and bookends with open/close fee rows.
// Simultaneous funding events are merged into a single row with sub-details.
func buildHoldSimulation(
	arb domain.FundingArbitrage,
	size decimal.Decimal,
	openCostPct, entryCostUsd decimal.Decimal,
	closeCostPct, exitCostUsd decimal.Decimal,
	holdHours decimal.Decimal,
	now time.Time,
	fd feeBreakdown,
) domain.LoopSimulation {
	holdDuration := time.Duration(holdHours.Mul(decimal.NewFromInt(3600)).IntPart()) * time.Second
	endTime := now.Add(holdDuration)

	// Collect all funding events within the hold window.
	type fundingEvent struct {
		at         time.Time
		exchangeID string
		side       domain.PositionSide
		rate       decimal.Decimal
		pnlPct     decimal.Decimal
	}

	var events []fundingEvent

	// Project long leg funding times forward from NextFundingAt.
	if !arb.Long.NextFundingAt.IsZero() && arb.Long.IntervalHours > 0 {
		interval := time.Duration(arb.Long.IntervalHours) * time.Hour
		t := arb.Long.NextFundingAt
		// If next funding is in the past relative to now, advance to next future occurrence.
		for t.Before(now) {
			t = t.Add(interval)
		}
		for !t.After(endTime) {
			pnlPct := arb.Long.FundingRate.Neg() // long pays opposite of rate
			events = append(events, fundingEvent{
				at:         t,
				exchangeID: arb.Long.ExchangeID,
				side:       domain.PositionSideLong,
				rate:       arb.Long.FundingRate,
				pnlPct:     pnlPct,
			})
			t = t.Add(interval)
		}
	}

	// Project short leg funding times forward.
	if !arb.Short.NextFundingAt.IsZero() && arb.Short.IntervalHours > 0 {
		interval := time.Duration(arb.Short.IntervalHours) * time.Hour
		t := arb.Short.NextFundingAt
		for t.Before(now) {
			t = t.Add(interval)
		}
		for !t.After(endTime) {
			pnlPct := arb.Short.FundingRate // short receives rate as-is
			events = append(events, fundingEvent{
				at:         t,
				exchangeID: arb.Short.ExchangeID,
				side:       domain.PositionSideShort,
				rate:       arb.Short.FundingRate,
				pnlPct:     pnlPct,
			})
			t = t.Add(interval)
		}
	}

	// Sort events by time, then exchange ID for determinism.
	sort.Slice(events, func(i, j int) bool {
		if events[i].at.Equal(events[j].at) {
			return events[i].exchangeID < events[j].exchangeID
		}
		return events[i].at.Before(events[j].at)
	})

	// Build individual funding items from events.
	var fundingItems []fundingItem
	for _, ev := range events {
		hoursFromNow := ev.at.Sub(now).Hours()
		atHour := decimal.NewFromFloat(hoursFromNow)
		pnlUsd := ev.pnlPct.Mul(size)
		evAt := ev.at
		label := formatHourLabel(hoursFromNow)
		fundingItems = append(fundingItems, fundingItem{
			label:  label,
			atHour: atHour,
			at:     &evAt,
			event: domain.SimulationEvent{
				ExchangeID: ev.exchangeID,
				Side:       ev.side,
				Rate:       ev.rate,
				PnlPct:     ev.pnlPct,
				PnlUsd:     pnlUsd,
			},
		})
	}

	// Merge simultaneous funding events using time-based grouping.
	// Since hold simulation items have decimal AtHour, we use mergeFundingItems
	// which groups by equal AtHour values.
	mergedFunding := mergeFundingItems(fundingItems)

	var rows []domain.SimulationRow
	cumPct := decimal.Zero
	cumUsd := decimal.Zero

	// Row 0: Open fees at t=0 (now) with per-exchange events.
	cumPct = cumPct.Add(openCostPct)
	cumUsd = cumUsd.Add(entryCostUsd.Neg())
	openTime := now
	rows = append(rows, domain.SimulationRow{
		Kind:          domain.RowKindOpen,
		Label:         "Open fees",
		AtHour:        decimal.Zero,
		PnlPct:        openCostPct,
		PnlUsd:        entryCostUsd.Neg(),
		CumulativePct: cumPct,
		CumulativeUsd: cumUsd,
		At:            &openTime,
		Events:        buildFeeEvents(domain.RowKindOpen, fd),
	})

	// Merged funding rows with running cumulative.
	for _, row := range mergedFunding {
		cumPct = cumPct.Add(row.PnlPct)
		cumUsd = cumUsd.Add(row.PnlUsd)
		row.CumulativePct = cumPct
		row.CumulativeUsd = cumUsd
		rows = append(rows, row)
	}

	// Last row: Close fees at hold end with per-exchange events.
	cumPct = cumPct.Add(closeCostPct)
	cumUsd = cumUsd.Add(exitCostUsd.Neg())
	closeTime := endTime
	rows = append(rows, domain.SimulationRow{
		Kind:          domain.RowKindClose,
		Label:         "Close fees",
		AtHour:        holdHours,
		PnlPct:        closeCostPct,
		PnlUsd:        exitCostUsd.Neg(),
		CumulativePct: cumPct,
		CumulativeUsd: cumUsd,
		At:            &closeTime,
		Events:        buildFeeEvents(domain.RowKindClose, fd),
	})

	return domain.LoopSimulation{
		DurationHours: holdHours,
		Rows:          rows,
		NetPnlPct:     cumPct,
		NetPnlUsd:     cumUsd,
	}
}

// formatHourLabel creates a human-readable label for an hour offset.
// Full hours: "4h", "12h". Non-full hours: "3h 42min", "0h 30min".
func formatHourLabel(hours float64) string {
	totalMinutes := int(math.Round(hours * 60))
	h := totalMinutes / 60
	m := totalMinutes % 60
	if m == 0 {
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dh %dmin", h, m)
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
