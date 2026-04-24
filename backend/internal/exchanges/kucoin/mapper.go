package kucoin

import (
	"time"

	ccxt "github.com/ccxt/ccxt/go/v4"

	"github.com/zeroclap/gembo/internal/domain"
	"github.com/zeroclap/gembo/internal/exchanges/base/ccxtutil"
)

func mapMarket(m ccxt.MarketInterface, updatedAt time.Time) domain.Market {
	return ccxtutil.MapMarket(m, exchangeID, updatedAt)
}

func mapTicker(t ccxt.Ticker, updatedAt time.Time) domain.Ticker {
	return ccxtutil.MapTicker(t, exchangeID, updatedAt)
}

func mapFundingRate(fr ccxt.FundingRate, updatedAt time.Time) domain.FundingRate {
	return ccxtutil.MapFundingRate(fr, exchangeID, updatedAt)
}
