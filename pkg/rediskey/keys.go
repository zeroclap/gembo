// Package rediskey defines the Redis key naming conventions for gembo.
// All keys follow the pattern: gembo:<exchange>:<type>:<symbol>
package rediskey

import "fmt"

const prefix = "gembo"

// Market returns the hash key for a single market.
//
//	gembo:<exchange>:market:<symbol>
func Market(exchangeID, symbol string) string {
	return fmt.Sprintf("%s:%s:market:%s", prefix, exchangeID, symbol)
}

// Markets returns the set key that holds all symbols for an exchange's markets.
//
//	gembo:<exchange>:markets
func Markets(exchangeID string) string {
	return fmt.Sprintf("%s:%s:markets", prefix, exchangeID)
}

// Ticker returns the hash key for a single ticker.
//
//	gembo:<exchange>:ticker:<symbol>
func Ticker(exchangeID, symbol string) string {
	return fmt.Sprintf("%s:%s:ticker:%s", prefix, exchangeID, symbol)
}

// Tickers returns the set key that holds all symbols for an exchange's tickers.
//
//	gembo:<exchange>:tickers
func Tickers(exchangeID string) string {
	return fmt.Sprintf("%s:%s:tickers", prefix, exchangeID)
}

// TickerPubSub returns the Pub/Sub channel name for ticker updates on an exchange.
//
//	gembo:<exchange>:ticker:updates
func TickerPubSub(exchangeID string) string {
	return fmt.Sprintf("%s:%s:ticker:updates", prefix, exchangeID)
}

// FundingRate returns the hash key for a single funding rate.
//
//	gembo:<exchange>:funding:<symbol>
func FundingRate(exchangeID, symbol string) string {
	return fmt.Sprintf("%s:%s:funding:%s", prefix, exchangeID, symbol)
}

// FundingRates returns the set key that holds all symbols with funding rates.
//
//	gembo:<exchange>:funding
func FundingRates(exchangeID string) string {
	return fmt.Sprintf("%s:%s:funding", prefix, exchangeID)
}
