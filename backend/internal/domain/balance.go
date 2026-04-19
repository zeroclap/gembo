package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

// Balance is an account balance snapshot for a single currency.
type Balance struct {
	UserID       string          `json:"user_id"`
	CredentialID string          `json:"credential_id"`
	ExchangeID   string          `json:"exchange_id"`
	Currency     string          `json:"currency"`
	Free         decimal.Decimal `json:"free"`
	Used         decimal.Decimal `json:"used"`
	Total        decimal.Decimal `json:"total"`
	UpdatedAt    time.Time       `json:"updated_at"`
}
