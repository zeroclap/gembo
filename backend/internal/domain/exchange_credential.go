package domain

import "time"

// ExchangeCredential stores API credentials for a specific user and exchange.
type ExchangeCredential struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	ExchangeID string    `json:"exchange_id"`
	Title      string    `json:"title"`
	APIKey     string    `json:"api_key"`
	APISecret  string    `json:"api_secret"`
	Passphrase string    `json:"passphrase,omitempty"`
	IsDefault  bool      `json:"is_default"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
