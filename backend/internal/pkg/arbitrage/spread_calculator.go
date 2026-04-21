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

// SpreadCalculator recomputes cross-exchange spread arbitrage opportunities
// from ticker data. It finds symbols traded on 2+ exchanges where the bid
// on one exchange exceeds the ask on another, indicating a price spread.
type SpreadCalculator struct {
	store  store.Store
	logger *slog.Logger
}

// NewSpreadCalculator creates a new spread arbitrage calculator.
func NewSpreadCalculator(st store.Store, logger *slog.Logger) *SpreadCalculator {
	return &SpreadCalculator{
		store:  st,
		logger: logger,
	}
}

// Recompute loads all tickers and markets from the store, finds symbols on 2+
// exchanges, computes the price spread and fee-adjusted net spread, and writes
// results back to the store.
func (c *SpreadCalculator) Recompute(ctx context.Context, exchangeIDs []string) error {
	now := time.Now().UTC()

	// Load tickers and markets per exchange.
	data := make(map[string]*exchangeData, len(exchangeIDs))
	for _, eid := range exchangeIDs {
		ed := &exchangeData{
			funding: make(map[string]domain.FundingRate),
			tickers: make(map[string]domain.Ticker),
			markets: make(map[string]domain.Market),
		}

		tickers, err := c.store.ListTickers(ctx, eid)
		if err != nil {
			c.logger.Warn("spread: list tickers", "exchange", eid, "error", err)
			continue
		}
		for _, t := range tickers {
			ed.tickers[t.Symbol] = t
		}

		markets, err := c.store.ListMarkets(ctx, eid)
		if err != nil {
			c.logger.Warn("spread: list markets", "exchange", eid, "error", err)
			continue
		}
		for _, m := range markets {
			ed.markets[m.Symbol] = m
		}

		data[eid] = ed
	}

	// Collect all symbols that appear on at least 2 exchanges (by ticker).
	symbolExchanges := make(map[string][]string)
	for eid, ed := range data {
		for sym := range ed.tickers {
			symbolExchanges[sym] = append(symbolExchanges[sym], eid)
		}
	}

	for sym, eids := range symbolExchanges {
		if len(eids) < 2 {
			continue
		}
		sort.Strings(eids)

		for i := 0; i < len(eids); i++ {
			for j := i + 1; j < len(eids); j++ {
				exA, exB := eids[i], eids[j]
				arb := c.computeSpreadPair(sym, exA, exB, data[exA], data[exB], now)
				if arb == nil {
					continue
				}
				if err := c.store.SetSpreadArbitrage(ctx, *arb); err != nil {
					c.logger.Warn("spread: store arbitrage",
						"symbol", sym, "error", err)
				}
			}
		}
	}

	return nil
}

// computeSpreadPair computes the spread opportunity between two exchanges for a
// symbol. The buy side is the exchange with the lower ask, the sell side has the
// higher bid. Returns nil if data is insufficient or prices are invalid.
func (c *SpreadCalculator) computeSpreadPair(
	symbol, exA, exB string,
	dataA, dataB *exchangeData,
	now time.Time,
) *domain.SpreadArbitrage {
	tA, okA := dataA.tickers[symbol]
	tB, okB := dataB.tickers[symbol]
	if !okA || !okB {
		return nil
	}

	// Need valid bid/ask on both sides.
	if tA.Ask.IsZero() || tA.Bid.IsZero() || tB.Ask.IsZero() || tB.Bid.IsZero() {
		return nil
	}

	// Determine buy/sell direction: buy where ask is lower, sell where bid is higher.
	// Direction 1: buy on A (ask_A), sell on B (bid_B).
	// Direction 2: buy on B (ask_B), sell on A (bid_A).
	// Pick the direction with the larger positive spread.
	spreadAB := tB.Bid.Sub(tA.Ask) // buy A, sell B
	spreadBA := tA.Bid.Sub(tB.Ask) // buy B, sell A

	var buyExID, sellExID string
	var buyAsk, sellBid decimal.Decimal
	var buyVolume, sellVolume decimal.Decimal
	var buyData, sellData *exchangeData

	if spreadAB.GreaterThanOrEqual(spreadBA) {
		buyExID, sellExID = exA, exB
		buyAsk, sellBid = tA.Ask, tB.Bid
		buyVolume, sellVolume = tA.Volume24h, tB.Volume24h
		buyData, sellData = dataA, dataB
	} else {
		buyExID, sellExID = exB, exA
		buyAsk, sellBid = tB.Ask, tA.Bid
		buyVolume, sellVolume = tB.Volume24h, tA.Volume24h
		buyData, sellData = dataB, dataA
	}

	spreadAbs := sellBid.Sub(buyAsk)
	midPrice := buyAsk.Add(sellBid).Div(dec2)
	if midPrice.IsZero() {
		return nil
	}

	spreadPct := spreadAbs.Div(midPrice).Mul(dec100)

	// Fee calculation: need market fees from both sides.
	buyFees, okBF := getMarketFees(buyData, symbol)
	sellFees, okSF := getMarketFees(sellData, symbol)
	if !okBF || !okSF {
		return nil
	}

	// Round-trip fees: taker on buy + taker on sell for open, same for close.
	buyTakerPct := buyFees.Taker.Mul(dec100)
	sellTakerPct := sellFees.Taker.Mul(dec100)
	openFeesPct := buyTakerPct.Add(sellTakerPct)
	closeFeesPct := buyTakerPct.Add(sellTakerPct)
	roundTripFeesPct := openFeesPct.Add(closeFeesPct)

	netSpreadPct := spreadPct.Sub(roundTripFeesPct)

	return &domain.SpreadArbitrage{
		Symbol:         symbol,
		BuyExchangeID:  buyExID,
		SellExchangeID: sellExID,
		BuyAsk:         buyAsk,
		SellBid:        sellBid,
		MidPrice:       midPrice,
		SpreadPct:      spreadPct,
		SpreadAbs:      spreadAbs,
		BuyVolume24h:   buyVolume,
		SellVolume24h:  sellVolume,
		Fees: domain.SpreadArbitrageFees{
			BuyTakerFeePct:   buyTakerPct,
			SellTakerFeePct:  sellTakerPct,
			OpenFeesPct:      openFeesPct,
			CloseFeesPct:     closeFeesPct,
			RoundTripFeesPct: roundTripFeesPct,
		},
		NetSpreadPct: netSpreadPct,
		UpdatedAt:    now,
	}
}
