package gate

import (
	"time"

	ccxt "github.com/ccxt/ccxt/go/v4"

	"github.com/zeroclap/gembo/internal/domain"
	"github.com/zeroclap/gembo/internal/exchange/ccxtutil"
)

// mapMarket converts a CCXT MarketInterface to a domain.Market.
func mapMarket(m ccxt.MarketInterface, updatedAt time.Time) domain.Market {
	return ccxtutil.MapMarket(m, exchangeID, updatedAt)
}

// mapTicker converts a CCXT Ticker to a domain.Ticker.
func mapTicker(t ccxt.Ticker, updatedAt time.Time) domain.Ticker {
	return ccxtutil.MapTicker(t, exchangeID, updatedAt)
}

// mapFundingRate converts a CCXT FundingRate to a domain.FundingRate.
func mapFundingRate(fr ccxt.FundingRate, updatedAt time.Time) domain.FundingRate {
	return ccxtutil.MapFundingRate(fr, exchangeID, updatedAt)
}

// mapBidAsk converts a CCXT Ticker from WatchBidsAsks into a partial
// domain.Ticker containing only the bid/ask fields and symbol metadata.
func mapBidAsk(t ccxt.Ticker, updatedAt time.Time) domain.Ticker {
	return ccxtutil.MapBidAsk(t, exchangeID, updatedAt)
}

// applyTickerUpdate merges a full ticker update into in-memory state.
func applyTickerUpdate(state map[string]*domain.Ticker, t domain.Ticker) *domain.Ticker {
	return ccxtutil.ApplyTickerUpdate(state, t)
}

// applyBidAskUpdate merges a bid/ask update into in-memory state.
func applyBidAskUpdate(state map[string]*domain.Ticker, t domain.Ticker) *domain.Ticker {
	return ccxtutil.ApplyBidAskUpdate(state, t)
}
