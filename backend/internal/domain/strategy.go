package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

// Strategy defines an automated arbitrage strategy that the strategy runner
// evaluates against live opportunities and executes positions on behalf of a user.
type Strategy struct {
	ID      string `json:"id"`
	UserID  string `json:"user_id"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`

	// ArbitrageType controls which opportunity stream this strategy targets.
	ArbitrageType ArbitrageType `json:"arbitrage_type"`

	// --- Criteria filters ----------------------------------------------------

	// MinAPR is the minimum annualised percentage required to trigger entry.
	// Only applicable when ArbitrageType == "funding". Zero means no minimum.
	MinAPR decimal.Decimal `json:"min_apr"`

	// MinNetSpreadPct is the minimum net spread percentage required to trigger
	// entry. Only applicable when ArbitrageType == "spread". Zero means no minimum.
	MinNetSpreadPct decimal.Decimal `json:"min_net_spread_pct"`

	// SymbolWhitelist restricts entry to specific symbols (e.g. "BTC/USDT:USDT").
	// Empty slice means any symbol is accepted.
	SymbolWhitelist []string `json:"symbol_whitelist"`

	// ExchangeWhitelist restricts entry to opportunities where both legs are
	// on exchanges present in this list. Empty slice means any exchange is accepted.
	ExchangeWhitelist []string `json:"exchange_whitelist"`

	// --- Position parameters -------------------------------------------------

	// Credentials maps exchange_id → credential_id for this strategy.
	// The runner looks up the relevant credentials when an opportunity matches.
	// At least one credential per expected exchange leg must be present.
	Credentials map[string]string `json:"credentials"`

	// SizeUSD is the nominal USD position size per entry.
	SizeUSD decimal.Decimal `json:"size_usd"`

	// Leverage is the per-position leverage to apply (1 = no leverage).
	Leverage int `json:"leverage"`

	// MaxConcurrentPositions is the maximum number of simultaneously open
	// positions that this strategy is allowed to hold. Zero is treated as 1.
	MaxConcurrentPositions int `json:"max_concurrent_positions"`

	// --- Spread arbitrage parameters (used when ArbitrageType == "spread") ---

	// TargetSpreadPct is the spread % at which the spread position is closed
	// (convergence target). Zero means no automatic target close.
	TargetSpreadPct decimal.Decimal `json:"target_spread_pct"`

	// StopLossSpreadPct is the spread % at which the position is closed to
	// limit losses. Zero means no stop-loss.
	StopLossSpreadPct decimal.Decimal `json:"stop_loss_spread_pct"`

	// MaxHoldDuration is the maximum time a spread position may be held before
	// it is force-closed, in seconds. Zero means no time limit.
	MaxHoldDuration int64 `json:"max_hold_duration"`

	// Timestamps.
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// StrategyFilter controls which strategies are returned by list queries.
type StrategyFilter struct {
	UserID string
}
