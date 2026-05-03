package store

import (
	"context"

	"github.com/zeroclap/gembo/internal/domain"
)

// NotificationStore defines the persistence interface for notification channels
// and alert rules.
type NotificationStore interface {
	// Channels

	// ListChannels returns all notification channels for the given user.
	ListChannels(ctx context.Context, userID string) ([]domain.NotificationChannel, error)
	// GetChannel returns a channel by ID scoped to the given user, or nil if not found.
	GetChannel(ctx context.Context, userID, channelID string) (*domain.NotificationChannel, error)
	// CreateChannel inserts a new notification channel and populates generated fields.
	CreateChannel(ctx context.Context, ch *domain.NotificationChannel) error
	// UpdateChannel updates an existing channel and returns true when found.
	UpdateChannel(ctx context.Context, ch *domain.NotificationChannel) (bool, error)
	// DeleteChannel removes a channel by ID for a user and returns true when deleted.
	// Deleting a channel also deletes all alert rules that reference it.
	DeleteChannel(ctx context.Context, userID, channelID string) (bool, error)

	// Rules

	// ListRules returns all alert rules for the given user.
	ListRules(ctx context.Context, userID string) ([]domain.AlertRule, error)
	// ListAllRules returns all alert rules across all users (used by the alert worker).
	ListAllRules(ctx context.Context) ([]domain.AlertRule, error)
	// GetRule returns a rule by ID scoped to the given user, or nil if not found.
	GetRule(ctx context.Context, userID, ruleID string) (*domain.AlertRule, error)
	// CreateRule inserts a new alert rule and populates generated fields.
	CreateRule(ctx context.Context, rule *domain.AlertRule) error
	// UpdateRule updates an existing rule and returns true when found.
	UpdateRule(ctx context.Context, rule *domain.AlertRule) (bool, error)
	// DeleteRule removes a rule by ID for a user and returns true when deleted.
	DeleteRule(ctx context.Context, userID, ruleID string) (bool, error)
	// TouchRuleTriggered updates last_triggered_at to now for the given rule.
	TouchRuleTriggered(ctx context.Context, ruleID string) error
}
