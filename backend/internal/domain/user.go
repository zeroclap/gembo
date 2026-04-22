package domain

import "time"

// User represents an authenticated application user.
type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ArbitrageWorkspace is a saved pair + exchange selection belonging to a user.
type ArbitrageWorkspace struct {
	ID            string `json:"id"`
	Symbol        string `json:"symbol"`
	LongExchange  string `json:"long_exchange"`
	ShortExchange string `json:"short_exchange"`
}
