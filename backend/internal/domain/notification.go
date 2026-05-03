package domain

import (
	"encoding/json"
	"time"

	"github.com/shopspring/decimal"
)

// ChannelType identifies the delivery mechanism for a notification channel.
type ChannelType string

const (
	ChannelTypeTelegram ChannelType = "telegram"
)

// TelegramConfig holds credentials for a Telegram bot channel.
type TelegramConfig struct {
	BotToken string `json:"bot_token"`
	ChatID   string `json:"chat_id"`
}

// NotificationChannel represents a user-configured delivery endpoint
// (e.g. a Telegram bot) that alerts can be sent through.
type NotificationChannel struct {
	ID        string          `json:"id"`
	UserID    string          `json:"user_id"`
	Type      ChannelType     `json:"type"`
	Name      string          `json:"name"`
	Config    json.RawMessage `json:"config"` // type-specific JSON
	Enabled   bool            `json:"enabled"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// AlertRuleType identifies what condition triggers an alert.
type AlertRuleType string

const (
	// AlertFundingAPR fires when a funding arbitrage APR exceeds a threshold.
	AlertFundingAPR AlertRuleType = "funding_apr_threshold"
	// AlertSpreadThreshold fires when a spread arbitrage spread % exceeds a threshold.
	AlertSpreadThreshold AlertRuleType = "spread_threshold"
	// AlertPositionOpened fires when a position transitions to open.
	AlertPositionOpened AlertRuleType = "position_opened"
	// AlertPositionClosed fires when a position transitions to closed.
	AlertPositionClosed AlertRuleType = "position_closed"
	// AlertFundingPayment fires when a funding payment is received.
	AlertFundingPayment AlertRuleType = "funding_payment"
	// AlertMarginWarning fires when margin health ratio drops below a threshold.
	AlertMarginWarning AlertRuleType = "margin_warning"
)

// AlertRule represents a user-defined condition that triggers a notification.
type AlertRule struct {
	ID              string          `json:"id"`
	UserID          string          `json:"user_id"`
	ChannelID       string          `json:"channel_id"`
	Type            AlertRuleType   `json:"type"`
	Params          json.RawMessage `json:"params"` // type-specific JSON
	Enabled         bool            `json:"enabled"`
	CooldownMinutes int             `json:"cooldown_minutes"`
	LastTriggeredAt *time.Time      `json:"last_triggered_at,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

// FundingAPRParams are the parameters for an AlertFundingAPR rule.
type FundingAPRParams struct {
	// MinAPR is the minimum annualised percentage to trigger (e.g. "50" = 50% APR).
	MinAPR decimal.Decimal `json:"min_apr"`
	// Symbol filters to a specific symbol; empty means any symbol.
	Symbol string `json:"symbol,omitempty"`
	// LongExchangeID filters to a specific long-leg exchange; empty means any.
	LongExchangeID string `json:"long_exchange_id,omitempty"`
	// ShortExchangeID filters to a specific short-leg exchange; empty means any.
	ShortExchangeID string `json:"short_exchange_id,omitempty"`
}

// SpreadThresholdParams are the parameters for an AlertSpreadThreshold rule.
type SpreadThresholdParams struct {
	// MinSpreadPct is the minimum spread percentage to trigger (e.g. "0.1" = 0.1%).
	MinSpreadPct decimal.Decimal `json:"min_spread_pct"`
	// Symbol filters to a specific symbol; empty means any symbol.
	Symbol string `json:"symbol,omitempty"`
}

// MarginWarningParams are the parameters for an AlertMarginWarning rule.
type MarginWarningParams struct {
	// MaxMarginRatio is the margin ratio below which an alert fires (e.g. "0.2" = 20%).
	MaxMarginRatio decimal.Decimal `json:"max_margin_ratio"`
}

// PositionEventParams are shared parameters for position-related alert rules
// (AlertPositionOpened, AlertPositionClosed, AlertFundingPayment).
// All fields are optional filters.
type PositionEventParams struct {
	// Symbol filters to a specific trading symbol; empty means any.
	Symbol string `json:"symbol,omitempty"`
}
