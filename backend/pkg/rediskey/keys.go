// Package rediskey defines the Redis key naming conventions for gembo.
// All keys follow the pattern: gembo:<exchange>:<type>:<symbol>
package rediskey

import (
	"fmt"
	"strings"
)

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

// Balance returns the key for a single balance by exchange and currency.
//
//	gembo:<exchange>:balance:<currency>
func Balance(exchangeID, currency string) string {
	return fmt.Sprintf("%s:%s:balance:%s", prefix, exchangeID, currency)
}

// UserBalance returns the key for a single balance by user, exchange and currency.
//
//	gembo:users:<user_id>:<exchange>:balance:<currency>
func UserBalance(userID, exchangeID, currency string) string {
	return fmt.Sprintf("%s:users:%s:%s:balance:%s", prefix, userID, exchangeID, currency)
}

// Balances returns the set key that tracks all currencies with balances.
//
//	gembo:<exchange>:balances
func Balances(exchangeID string) string {
	return fmt.Sprintf("%s:%s:balances", prefix, exchangeID)
}

// UserBalances returns the set key that tracks all currencies for a user+exchange.
//
//	gembo:users:<user_id>:<exchange>:balances
func UserBalances(userID, exchangeID string) string {
	return fmt.Sprintf("%s:users:%s:%s:balances", prefix, userID, exchangeID)
}

// BalancePubSub returns the Pub/Sub channel name for balance updates on an exchange.
//
//	gembo:<exchange>:balance:updates
func BalancePubSub(exchangeID string) string {
	return fmt.Sprintf("%s:%s:balance:updates", prefix, exchangeID)
}

// UserBalancePubSub returns the Pub/Sub channel for user-scoped balance updates.
//
//	gembo:users:<user_id>:<exchange>:balance:updates
func UserBalancePubSub(userID, exchangeID string) string {
	return fmt.Sprintf("%s:users:%s:%s:balance:updates", prefix, userID, exchangeID)
}

// ArbitragePairs returns the set key that tracks all arbitrage pair identifiers
// across all strategy types.
//
//	gembo:arbitrage:pairs
func ArbitragePairs() string {
	return fmt.Sprintf("%s:arbitrage:pairs", prefix)
}

// ArbitragePair returns the key for a single arbitrage opportunity.
// Exchange IDs are expected to be sorted alphabetically by the caller.
// The arbType is embedded in the pipe-delimited suffix so all arbitrage
// keys share the single namespace gembo:arbitrage:.
//
//	gembo:arbitrage:<symbol>|<type>|<exchangeA>|<exchangeB>
func ArbitragePair(arbType, symbol, exchangeA, exchangeB string) string {
	return fmt.Sprintf("%s:arbitrage:%s|%s|%s|%s", prefix, symbol, arbType, exchangeA, exchangeB)
}

// ArbitragePubSub returns the unified Pub/Sub channel for all arbitrage updates.
//
//	gembo:arbitrage:updates
func ArbitragePubSub() string {
	return fmt.Sprintf("%s:arbitrage:updates", prefix)
}

// PositionPending returns the Pub/Sub channel for new position open requests.
//
//	gembo:positions:pending
func PositionPending() string {
	return fmt.Sprintf("%s:positions:pending", prefix)
}

// UserPositionPending returns user-scoped position open request channel.
//
//	gembo:users:<user_id>:positions:pending
func UserPositionPending(userID string) string {
	return fmt.Sprintf("%s:users:%s:positions:pending", prefix, userID)
}

// PositionClosing returns the Pub/Sub channel for position close requests.
//
//	gembo:positions:closing
func PositionClosing() string {
	return fmt.Sprintf("%s:positions:closing", prefix)
}

// UserPositionClosing returns user-scoped position close request channel.
//
//	gembo:users:<user_id>:positions:closing
func UserPositionClosing(userID string) string {
	return fmt.Sprintf("%s:users:%s:positions:closing", prefix, userID)
}

// PositionUpdates returns the Pub/Sub channel for position state changes.
//
//	gembo:positions:updates
func PositionUpdates() string {
	return fmt.Sprintf("%s:positions:updates", prefix)
}

// UserPositionUpdates returns user-scoped position updates channel.
//
//	gembo:users:<user_id>:positions:updates
func UserPositionUpdates(userID string) string {
	return fmt.Sprintf("%s:users:%s:positions:updates", prefix, userID)
}

// PositionStatePubSub returns the Pub/Sub channel for exchange position state
// events published by the collector. Keyed per exchange so subscribers can
// filter by exchange.
//
//	gembo:<exchange>:position:updates
func PositionStatePubSub(exchangeID string) string {
	return fmt.Sprintf("%s:%s:position:updates", prefix, exchangeID)
}

// UserPositionStatePubSub returns user-scoped exchange position updates channel.
//
//	gembo:users:<user_id>:<exchange>:position:updates
func UserPositionStatePubSub(userID, exchangeID string) string {
	return fmt.Sprintf("%s:users:%s:%s:position:updates", prefix, userID, exchangeID)
}

// UserPositionPendingPattern matches all user-scoped pending channels.
func UserPositionPendingPattern() string {
	return fmt.Sprintf("%s:users:*:positions:pending", prefix)
}

// UserPositionClosingPattern matches all user-scoped closing channels.
func UserPositionClosingPattern() string {
	return fmt.Sprintf("%s:users:*:positions:closing", prefix)
}

// UserPositionStatePattern matches all user-scoped position state channels.
func UserPositionStatePattern() string {
	return fmt.Sprintf("%s:users:*:*:position:updates", prefix)
}

// UserFundingPaymentPubSub returns user-scoped funding payment event channel.
//
//	gembo:users:<user_id>:<exchange>:funding:payments:updates
func UserFundingPaymentPubSub(userID, exchangeID string) string {
	return fmt.Sprintf("%s:users:%s:%s:funding:payments:updates", prefix, userID, exchangeID)
}

// UserFundingPaymentPattern matches all user-scoped funding payment channels.
func UserFundingPaymentPattern() string {
	return fmt.Sprintf("%s:users:*:*:funding:payments:updates", prefix)
}

// UserIDFromChannel extracts user_id from a user-scoped channel.
func UserIDFromChannel(channel string) string {
	parts := strings.Split(channel, ":")
	if len(parts) < 4 || parts[0] != prefix || parts[1] != "users" {
		return ""
	}
	return parts[2]
}
