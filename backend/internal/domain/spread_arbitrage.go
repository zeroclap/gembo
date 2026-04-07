package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

// ArbitrageType distinguishes between different arbitrage strategies.
type ArbitrageType string

const (
	ArbitrageTypeFunding ArbitrageType = "funding"
	ArbitrageTypeSpread  ArbitrageType = "spread"
)

// SpreadArbitrage represents a cross-exchange price spread opportunity.
// The strategy: buy on the cheaper exchange and sell on the more expensive one,
// then wait for the spread to converge (prices to equalize).
type SpreadArbitrage struct {
	Symbol         string `json:"symbol"`
	BuyExchangeID  string `json:"buy_exchange_id"`  // exchange with lower ask
	SellExchangeID string `json:"sell_exchange_id"` // exchange with higher bid

	BuyAsk  decimal.Decimal `json:"buy_ask"`  // ask price on buy exchange
	SellBid decimal.Decimal `json:"sell_bid"` // bid price on sell exchange

	MidPrice  decimal.Decimal `json:"mid_price"`  // (buy_ask + sell_bid) / 2
	SpreadPct decimal.Decimal `json:"spread_pct"` // (sell_bid - buy_ask) / mid_price * 100
	SpreadAbs decimal.Decimal `json:"spread_abs"` // sell_bid - buy_ask

	BuyVolume24h  decimal.Decimal `json:"buy_volume_24h"`
	SellVolume24h decimal.Decimal `json:"sell_volume_24h"`

	Fees         SpreadArbitrageFees `json:"fees"`
	NetSpreadPct decimal.Decimal     `json:"net_spread_pct"` // spread_pct - round_trip_fees_pct

	UpdatedAt time.Time `json:"updated_at"`
}

// SpreadArbitrageFees contains fee estimates for opening and closing a spread
// arbitrage position (taker fees on both legs, each direction).
type SpreadArbitrageFees struct {
	BuyTakerFeePct   decimal.Decimal `json:"buy_taker_fee_pct"`   // taker fee on buy exchange (%)
	SellTakerFeePct  decimal.Decimal `json:"sell_taker_fee_pct"`  // taker fee on sell exchange (%)
	OpenFeesPct      decimal.Decimal `json:"open_fees_pct"`       // buy taker + sell taker (%)
	CloseFeesPct     decimal.Decimal `json:"close_fees_pct"`      // same (close is reverse trades)
	RoundTripFeesPct decimal.Decimal `json:"round_trip_fees_pct"` // open + close (%)
}

// SpreadSimulationRequest contains parameters for simulating a spread arbitrage trade.
type SpreadSimulationRequest struct {
	Symbol          string          `json:"symbol"`
	BuyExchangeID   string          `json:"buy_exchange_id"`
	SellExchangeID  string          `json:"sell_exchange_id"`
	PositionSize    decimal.Decimal `json:"position_size"`     // notional value in USD
	TargetSpreadPct decimal.Decimal `json:"target_spread_pct"` // expected exit spread (%)
	SlippagePct     decimal.Decimal `json:"slippage_pct"`      // slippage per side (%)
}

// SpreadSimulationResult contains the computed P&L simulation for a spread
// arbitrage opportunity.
type SpreadSimulationResult struct {
	PositionSize decimal.Decimal `json:"position_size"` // notional per side in USD

	EntrySpreadPct  decimal.Decimal `json:"entry_spread_pct"`  // current spread at entry
	TargetSpreadPct decimal.Decimal `json:"target_spread_pct"` // target spread at exit

	SpreadCapturePct decimal.Decimal `json:"spread_capture_pct"` // entry - target spread (%)
	SpreadCaptureUsd decimal.Decimal `json:"spread_capture_usd"` // spread capture in USD

	EntryCost decimal.Decimal `json:"entry_cost"` // $ cost to open (fees + slippage)
	ExitCost  decimal.Decimal `json:"exit_cost"`  // $ cost to close (fees + slippage)
	TotalCost decimal.Decimal `json:"total_cost"` // entry + exit

	GrossPnl  decimal.Decimal `json:"gross_pnl"`   // spread capture in USD
	NetPnl    decimal.Decimal `json:"net_pnl"`     // gross - total cost
	NetPnlPct decimal.Decimal `json:"net_pnl_pct"` // net P&L as % of position size

	BreakevenSpreadPct decimal.Decimal `json:"breakeven_spread_pct"` // spread at which P&L = 0 after fees
}
