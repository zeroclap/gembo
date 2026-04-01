// Package rediskey defines the Redis key naming conventions for gembo.
// All keys follow the pattern: gembo:<exchange>:<type>:<symbol>
package rediskey

import "fmt"

const prefix = "gembo"

// Prefix is the common key prefix used by all gembo keys.
// Useful for SCAN-based bulk operations (e.g. flush).
const Prefix = prefix

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

// FundingRatePubSub returns the Pub/Sub channel name for funding rate updates on an exchange.
//
//	gembo:<exchange>:funding:updates
func FundingRatePubSub(exchangeID string) string {
	return fmt.Sprintf("%s:%s:funding:updates", prefix, exchangeID)
}

// FundingArbitragePairs returns the set key that tracks all funding arbitrage pair identifiers.
//
//	gembo:arbitrage:funding:pairs
func FundingArbitragePairs() string {
	return fmt.Sprintf("%s:arbitrage:funding:pairs", prefix)
}

// FundingArbitragePair returns the key for a single funding arbitrage opportunity.
// Exchange IDs are expected to be sorted alphabetically by the caller to ensure uniqueness.
//
//	gembo:arbitrage:funding:<symbol>|<exchangeA>|<exchangeB>
func FundingArbitragePair(symbol, exchangeA, exchangeB string) string {
	return fmt.Sprintf("%s:arbitrage:funding:%s|%s|%s", prefix, symbol, exchangeA, exchangeB)
}

// PositionPending returns the Pub/Sub channel for new position open requests.
//
//	gembo:positions:pending
func PositionPending() string {
	return fmt.Sprintf("%s:positions:pending", prefix)
}

// PositionClosing returns the Pub/Sub channel for position close requests.
//
//	gembo:positions:closing
func PositionClosing() string {
	return fmt.Sprintf("%s:positions:closing", prefix)
}

// PositionUpdates returns the Pub/Sub channel for position state changes.
//
//	gembo:positions:updates
func PositionUpdates() string {
	return fmt.Sprintf("%s:positions:updates", prefix)
}
