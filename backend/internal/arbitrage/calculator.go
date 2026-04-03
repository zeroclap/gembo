// Package arbitrage implements the funding rate arbitrage calculator.
// It compares funding rates across exchanges to find cross-exchange
// long+short arbitrage opportunities.
package arbitrage

import (
	"context"
	"log/slog"
	"sort"
	"time"

	"github.com/shopspring/decimal"
	"github.com/zeroclap/gembo/internal/domain"
	"github.com/zeroclap/gembo/internal/store"
)

var (
	dec8   = decimal.NewFromInt(8)
	dec3   = decimal.NewFromInt(3)
	dec100 = decimal.NewFromInt(100)
	dec365 = decimal.NewFromInt(365)
	dec2   = decimal.NewFromInt(2)
)

// Calculator recomputes arbitrage opportunities from funding rates and tickers.
type Calculator struct {
	store  store.Store
	logger *slog.Logger
}

// NewCalculator creates a new arbitrage calculator.
func NewCalculator(st store.Store, logger *slog.Logger) *Calculator {
	return &Calculator{
		store:  st,
		logger: logger,
	}
}

// exchangeData holds the funding rates, tickers, and markets for one exchange, keyed by symbol.
type exchangeData struct {
	funding map[string]domain.FundingRate
	tickers map[string]domain.Ticker
	markets map[string]domain.Market
}

// Recompute loads all funding rates and tickers from the store, finds cross-exchange
// pairs, computes arbitrage metrics, and writes results back to the store.
func (c *Calculator) Recompute(ctx context.Context, exchangeIDs []string) error {
	now := time.Now().UTC()

	// Load all data per exchange.
	data := make(map[string]*exchangeData, len(exchangeIDs))
	for _, eid := range exchangeIDs {
		ed := &exchangeData{
			funding: make(map[string]domain.FundingRate),
			tickers: make(map[string]domain.Ticker),
			markets: make(map[string]domain.Market),
		}

		rates, err := c.store.ListFundingRates(ctx, eid)
		if err != nil {
			c.logger.Warn("list funding rates", "exchange", eid, "error", err)
			continue
		}
		for _, fr := range rates {
			ed.funding[fr.Symbol] = fr
		}

		tickers, err := c.store.ListTickers(ctx, eid)
		if err != nil {
			c.logger.Warn("list tickers", "exchange", eid, "error", err)
			continue
		}
		for _, t := range tickers {
			ed.tickers[t.Symbol] = t
		}

		markets, err := c.store.ListMarkets(ctx, eid)
		if err != nil {
			c.logger.Warn("list markets", "exchange", eid, "error", err)
		}
		for _, m := range markets {
			ed.markets[m.Symbol] = m
		}

		data[eid] = ed
	}

	// Collect all symbols that appear on at least 2 exchanges.
	symbolExchanges := make(map[string][]string)
	for eid, ed := range data {
		for sym := range ed.funding {
			symbolExchanges[sym] = append(symbolExchanges[sym], eid)
		}
	}

	for sym, eids := range symbolExchanges {
		if len(eids) < 2 {
			continue
		}
		// Sort exchange IDs to ensure deterministic pairing.
		sort.Strings(eids)

		// For each pair of exchanges, compute arbitrage.
		for i := 0; i < len(eids); i++ {
			for j := i + 1; j < len(eids); j++ {
				exA, exB := eids[i], eids[j]
				arb := c.computePair(sym, exA, exB, data[exA], data[exB], now)
				if arb == nil {
					continue
				}
				if err := c.store.SetFundingArbitrage(ctx, *arb); err != nil {
					c.logger.Warn("store arbitrage", "symbol", sym, "error", err)
				}
			}
		}
	}

	return nil
}

// computePair computes the arbitrage opportunity between two exchanges for a symbol.
// Returns nil if data is insufficient.
func (c *Calculator) computePair(
	symbol, exA, exB string,
	dataA, dataB *exchangeData,
	now time.Time,
) *domain.FundingArbitrage {
	frA, okA := dataA.funding[symbol]
	frB, okB := dataB.funding[symbol]
	if !okA || !okB {
		return nil
	}

	tA, okTA := dataA.tickers[symbol]
	tB, okTB := dataB.tickers[symbol]
	if !okTA || !okTB {
		return nil
	}

	// Normalize both rates to 8h equivalent.
	rate8hA := normalizeRate8h(frA.Rate, frA.IntervalHours)
	rate8hB := normalizeRate8h(frB.Rate, frB.IntervalHours)

	// Strategy: long on the exchange with the lower 8h rate, short on the higher.
	// Net = short_rate_8h - long_rate_8h (positive = profitable).
	var longEx, shortEx string
	var longFR, shortFR domain.FundingRate
	var longTicker, shortTicker domain.Ticker
	var longRate8h, shortRate8h decimal.Decimal
	var longData, shortData *exchangeData

	if rate8hA.LessThanOrEqual(rate8hB) {
		longEx, shortEx = exA, exB
		longFR, shortFR = frA, frB
		longTicker, shortTicker = tA, tB
		longRate8h, shortRate8h = rate8hA, rate8hB
		longData, shortData = dataA, dataB
	} else {
		longEx, shortEx = exB, exA
		longFR, shortFR = frB, frA
		longTicker, shortTicker = tB, tA
		longRate8h, shortRate8h = rate8hB, rate8hA
		longData, shortData = dataB, dataA
	}

	netRate8h := shortRate8h.Sub(longRate8h)

	// APR = net_rate_8h * 3 payments/day * 365 days * 100 for percentage.
	apr := netRate8h.Mul(dec3).Mul(dec365).Mul(dec100)

	// Open spread = (short_bid - long_ask) / midpoint * 100.
	midpoint := shortTicker.Bid.Add(longTicker.Ask).Div(dec2)
	var openSpread decimal.Decimal
	if !midpoint.IsZero() {
		openSpread = shortTicker.Bid.Sub(longTicker.Ask).Div(midpoint).Mul(dec100)
	}

	// Fees: use real per-market fees from the store.
	// Fees are stored as ratios; convert to percentage for ArbitrageFees output.
	longFees, okLF := getMarketFees(longData, symbol)
	shortFees, okSF := getMarketFees(shortData, symbol)
	if !okLF || !okSF {
		return nil
	}

	openFeesPct := longFees.Taker.Add(shortFees.Taker).Mul(dec100)
	closeFeesPct := longFees.Taker.Add(shortFees.Taker).Mul(dec100)
	roundTripFeesPct := openFeesPct.Add(closeFeesPct)

	// NetAPRAfterFees: subtract annualized cost of one round-trip per year.
	// Simplification: assume you hold position for 1 year, pay fees once.
	netAPRAfterFees := apr.Sub(roundTripFeesPct)

	// Build legs.
	longMarket := longData.markets[symbol]
	shortMarket := shortData.markets[symbol]
	longLeg := buildLeg(longEx, longFR, longTicker, longRate8h, longFees, longMarket)
	shortLeg := buildLeg(shortEx, shortFR, shortTicker, shortRate8h, shortFees, shortMarket)

	// Next payment info: which leg pays next?
	nextPayment := computeNextPayment(longLeg, shortLeg, now)

	// Full loop info.
	fullLoop := computeFullLoop(longLeg, shortLeg)

	// Ensure exchangeA < exchangeB alphabetically in the stored key.
	// The Long/Short assignment is independent of key ordering.
	return &domain.FundingArbitrage{
		Symbol:     symbol,
		Long:       longLeg,
		Short:      shortLeg,
		NetRate8h:  netRate8h,
		APR:        apr,
		OpenSpread: openSpread,
		Fees: domain.ArbitrageFees{
			OpenFeesPct:      openFeesPct,
			CloseFeesPct:     closeFeesPct,
			RoundTripFeesPct: roundTripFeesPct,
			NetAPRAfterFees:  netAPRAfterFees,
		},
		NextPayment: nextPayment,
		FullLoop:    fullLoop,
		UpdatedAt:   now,
	}
}

// normalizeRate8h normalizes a funding rate to an 8-hour equivalent.
// If the interval is 4h, the rate covers half the period, so multiply by 2.
// If 8h, use as-is. General formula: rate * (8 / interval).
func normalizeRate8h(rate decimal.Decimal, intervalHours int) decimal.Decimal {
	if intervalHours <= 0 {
		intervalHours = 8
	}
	return rate.Mul(dec8).Div(decimal.NewFromInt(int64(intervalHours)))
}

func buildLeg(
	exchangeID string,
	fr domain.FundingRate,
	ticker domain.Ticker,
	rate8h decimal.Decimal,
	fees marketFees,
	market domain.Market,
) domain.ArbitrageLeg {
	return domain.ArbitrageLeg{
		ExchangeID:      exchangeID,
		FundingRate:     fr.Rate,
		IntervalHours:   fr.IntervalHours,
		Rate8h:          rate8h,
		NextFundingAt:   fr.NextFundingTime,
		NextFundingRate: fr.NextFundingRate,
		Price:           ticker.Price,
		Bid:             ticker.Bid,
		Ask:             ticker.Ask,
		MarkPrice:       fr.MarkPrice,
		IndexPrice:      fr.IndexPrice,
		Volume24h:       ticker.Volume24h,
		MakerFeePct:     fees.Maker.Mul(dec100),
		TakerFeePct:     fees.Taker.Mul(dec100),
		ContractSize:    market.ContractSize,
	}
}

// marketFees holds maker/taker fee ratios for a specific market (e.g. 0.00055 = 0.055%).
type marketFees struct {
	Maker decimal.Decimal
	Taker decimal.Decimal
}

// getMarketFees returns the taker/maker fees for a specific symbol on an exchange.
// Returns false if the market is not found or fees are zero (exchange did not report them).
func getMarketFees(ed *exchangeData, symbol string) (marketFees, bool) {
	m, ok := ed.markets[symbol]
	if !ok {
		return marketFees{}, false
	}
	if m.MakerFee.IsZero() && m.TakerFee.IsZero() {
		return marketFees{}, false
	}
	return marketFees{Maker: m.MakerFee, Taker: m.TakerFee}, true
}

// computeNextPayment determines which leg has the next funding payment and
// what the P&L impact is for the arbitrageur.
func computeNextPayment(long, short domain.ArbitrageLeg, now time.Time) domain.NextPaymentInfo {
	longNext := long.NextFundingAt
	shortNext := short.NextFundingAt

	// Pick the earlier of the two.
	var info domain.NextPaymentInfo
	if !longNext.IsZero() && (shortNext.IsZero() || longNext.Before(shortNext)) {
		// Long leg pays next.
		// As a long holder: if rate > 0, you pay (negative P&L); if rate < 0, you receive.
		paymentPct := long.FundingRate.Neg() // long pays the opposite of rate sign
		info = domain.NextPaymentInfo{
			ExchangeID:    long.ExchangeID,
			Side:          "long",
			Rate:          long.FundingRate,
			PaymentPct:    paymentPct,
			At:            longNext,
			TimeUntilSecs: max(0, int64(longNext.Sub(now).Seconds())),
		}
	} else {
		// Short leg pays next.
		// As a short holder: if rate > 0, you receive; if rate < 0, you pay.
		paymentPct := short.FundingRate // short receives the rate as-is
		info = domain.NextPaymentInfo{
			ExchangeID:    short.ExchangeID,
			Side:          "short",
			Rate:          short.FundingRate,
			PaymentPct:    paymentPct,
			At:            shortNext,
			TimeUntilSecs: max(0, int64(shortNext.Sub(now).Seconds())),
		}
	}

	return info
}

// computeFullLoop calculates the full funding cycle (LCM of intervals) and
// generates the ordered payment schedule with cumulative P&L.
func computeFullLoop(long, short domain.ArbitrageLeg) domain.FullLoopInfo {
	longInt := long.IntervalHours
	shortInt := short.IntervalHours
	if longInt <= 0 {
		longInt = 8
	}
	if shortInt <= 0 {
		shortInt = 8
	}

	loopHours := lcm(longInt, shortInt)

	// Build payment schedule.
	type paymentEvent struct {
		hour       int
		exchangeID string
		side       string
		rate       decimal.Decimal
	}

	var events []paymentEvent

	// Long leg payments: every longInt hours.
	for h := longInt; h <= loopHours; h += longInt {
		// Long holder: payment_pct = -rate (positive rate means you pay).
		events = append(events, paymentEvent{
			hour:       h,
			exchangeID: long.ExchangeID,
			side:       "long",
			rate:       long.FundingRate,
		})
	}

	// Short leg payments: every shortInt hours.
	for h := shortInt; h <= loopHours; h += shortInt {
		// Short holder: payment_pct = +rate (positive rate means you receive).
		events = append(events, paymentEvent{
			hour:       h,
			exchangeID: short.ExchangeID,
			side:       "short",
			rate:       short.FundingRate,
		})
	}

	// Sort by hour, then by exchange ID for determinism.
	sort.Slice(events, func(i, j int) bool {
		if events[i].hour != events[j].hour {
			return events[i].hour < events[j].hour
		}
		return events[i].exchangeID < events[j].exchangeID
	})

	cumulative := decimal.Zero
	payments := make([]domain.LoopPayment, len(events))
	for i, ev := range events {
		var pct decimal.Decimal
		if ev.side == "long" {
			pct = ev.rate.Neg() // long holder pays opposite of rate
		} else {
			pct = ev.rate // short holder receives rate
		}
		cumulative = cumulative.Add(pct)
		payments[i] = domain.LoopPayment{
			ExchangeID: ev.exchangeID,
			Side:       ev.side,
			Rate:       ev.rate,
			PaymentPct: pct,
			AtHour:     ev.hour,
			Cumulative: cumulative,
		}
	}

	return domain.FullLoopInfo{
		DurationHours: loopHours,
		Payments:      payments,
		NetPctPerLoop: cumulative,
	}
}

// gcd returns the greatest common divisor of a and b.
func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

// lcm returns the least common multiple of a and b.
func lcm(a, b int) int {
	if a == 0 || b == 0 {
		return 0
	}
	return (a / gcd(a, b)) * b
}
