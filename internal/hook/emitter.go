// Package hook defines the event emission interface for gembo.
package hook

import (
	"context"
	"encoding/json"
)

// EventType identifies the kind of data update.
type EventType string

const (
	EventMarketUpdated  EventType = "market_updated"
	EventTickerUpdated  EventType = "ticker_updated"
	EventFundingUpdated EventType = "funding_rate_updated"
)

// Event represents a data update notification.
type Event struct {
	Type     EventType       `json:"type"`
	Exchange string          `json:"exchange"`
	Symbol   string          `json:"symbol"`
	Payload  json.RawMessage `json:"payload"` // pre-encoded domain model
}

// Emitter publishes events to downstream consumers.
type Emitter interface {
	Emit(ctx context.Context, e Event) error
}

// NoopEmitter is an Emitter that discards all events. Useful when hooks are
// disabled.
type NoopEmitter struct{}

func (NoopEmitter) Emit(_ context.Context, _ Event) error { return nil }
