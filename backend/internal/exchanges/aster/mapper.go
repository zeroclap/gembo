package aster

import (
	"time"

	ccxt "github.com/ccxt/ccxt/go/v4"

	"github.com/zeroclap/gembo/internal/domain"
	"github.com/zeroclap/gembo/internal/exchanges/base/ccxtutil"
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
