package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/zeroclap/gembo/internal/domain"
)

// ListChannels returns all notification channels for the given user.
func (s *Store) ListChannels(ctx context.Context, userID string) ([]domain.NotificationChannel, error) {
	const q = `
		SELECT id, user_id, type, name, config, enabled, created_at, updated_at
		FROM notification_channels
		WHERE user_id = $1
		ORDER BY created_at ASC`
	rows, err := s.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("list notification channels: %w", err)
	}
	defer rows.Close()

	channels := make([]domain.NotificationChannel, 0)
	for rows.Next() {
		var ch domain.NotificationChannel
		var configRaw []byte
		if err := rows.Scan(
			&ch.ID, &ch.UserID, &ch.Type, &ch.Name,
			&configRaw, &ch.Enabled, &ch.CreatedAt, &ch.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan notification channel: %w", err)
		}
		ch.Config = json.RawMessage(configRaw)
		channels = append(channels, ch)
	}
	return channels, nil
}

// GetChannel returns a channel by ID scoped to the given user, or nil if not found.
func (s *Store) GetChannel(ctx context.Context, userID, channelID string) (*domain.NotificationChannel, error) {
	const q = `
		SELECT id, user_id, type, name, config, enabled, created_at, updated_at
		FROM notification_channels
		WHERE user_id = $1 AND id = $2`
	row := s.pool.QueryRow(ctx, q, userID, channelID)

	var ch domain.NotificationChannel
	var configRaw []byte
	if err := row.Scan(
		&ch.ID, &ch.UserID, &ch.Type, &ch.Name,
		&configRaw, &ch.Enabled, &ch.CreatedAt, &ch.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get notification channel: %w", err)
	}
	ch.Config = json.RawMessage(configRaw)
	return &ch, nil
}

// CreateChannel inserts a new notification channel and populates generated fields.
func (s *Store) CreateChannel(ctx context.Context, ch *domain.NotificationChannel) error {
	const q = `
		INSERT INTO notification_channels (user_id, type, name, config, enabled)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at`
	row := s.pool.QueryRow(ctx, q, ch.UserID, ch.Type, ch.Name, []byte(ch.Config), ch.Enabled)
	if err := row.Scan(&ch.ID, &ch.CreatedAt, &ch.UpdatedAt); err != nil {
		return fmt.Errorf("create notification channel: %w", err)
	}
	return nil
}

// UpdateChannel updates an existing channel and returns true when found.
func (s *Store) UpdateChannel(ctx context.Context, ch *domain.NotificationChannel) (bool, error) {
	const q = `
		UPDATE notification_channels
		SET type = $3, name = $4, config = $5, enabled = $6, updated_at = now()
		WHERE user_id = $1 AND id = $2`
	result, err := s.pool.Exec(ctx, q, ch.UserID, ch.ID, ch.Type, ch.Name, []byte(ch.Config), ch.Enabled)
	if err != nil {
		return false, fmt.Errorf("update notification channel: %w", err)
	}
	return result.RowsAffected() > 0, nil
}

// DeleteChannel removes a channel (and cascade-deletes its alert rules) and
// returns true when deleted.
func (s *Store) DeleteChannel(ctx context.Context, userID, channelID string) (bool, error) {
	const q = `DELETE FROM notification_channels WHERE user_id = $1 AND id = $2`
	result, err := s.pool.Exec(ctx, q, userID, channelID)
	if err != nil {
		return false, fmt.Errorf("delete notification channel: %w", err)
	}
	return result.RowsAffected() > 0, nil
}

// ListRules returns all alert rules for the given user.
func (s *Store) ListRules(ctx context.Context, userID string) ([]domain.AlertRule, error) {
	const q = `
		SELECT id, user_id, channel_id, type, params, enabled, cooldown_minutes,
		       last_triggered_at, created_at, updated_at
		FROM alert_rules
		WHERE user_id = $1
		ORDER BY created_at ASC`
	return s.scanRules(ctx, q, userID)
}

// ListAllRules returns all alert rules across all users.
func (s *Store) ListAllRules(ctx context.Context) ([]domain.AlertRule, error) {
	const q = `
		SELECT id, user_id, channel_id, type, params, enabled, cooldown_minutes,
		       last_triggered_at, created_at, updated_at
		FROM alert_rules
		ORDER BY user_id, created_at ASC`
	return s.scanRules(ctx, q)
}

func (s *Store) scanRules(ctx context.Context, query string, args ...interface{}) ([]domain.AlertRule, error) {
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query alert rules: %w", err)
	}
	defer rows.Close()

	rules := make([]domain.AlertRule, 0)
	for rows.Next() {
		var r domain.AlertRule
		var paramsRaw []byte
		if err := rows.Scan(
			&r.ID, &r.UserID, &r.ChannelID, &r.Type, &paramsRaw,
			&r.Enabled, &r.CooldownMinutes, &r.LastTriggeredAt,
			&r.CreatedAt, &r.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan alert rule: %w", err)
		}
		r.Params = json.RawMessage(paramsRaw)
		rules = append(rules, r)
	}
	return rules, nil
}

// GetRule returns a rule by ID scoped to the given user, or nil if not found.
func (s *Store) GetRule(ctx context.Context, userID, ruleID string) (*domain.AlertRule, error) {
	const q = `
		SELECT id, user_id, channel_id, type, params, enabled, cooldown_minutes,
		       last_triggered_at, created_at, updated_at
		FROM alert_rules
		WHERE user_id = $1 AND id = $2`
	row := s.pool.QueryRow(ctx, q, userID, ruleID)

	var r domain.AlertRule
	var paramsRaw []byte
	if err := row.Scan(
		&r.ID, &r.UserID, &r.ChannelID, &r.Type, &paramsRaw,
		&r.Enabled, &r.CooldownMinutes, &r.LastTriggeredAt,
		&r.CreatedAt, &r.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get alert rule: %w", err)
	}
	r.Params = json.RawMessage(paramsRaw)
	return &r, nil
}

// CreateRule inserts a new alert rule and populates generated fields.
func (s *Store) CreateRule(ctx context.Context, rule *domain.AlertRule) error {
	const q = `
		INSERT INTO alert_rules (user_id, channel_id, type, params, enabled, cooldown_minutes)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at`
	row := s.pool.QueryRow(ctx, q,
		rule.UserID, rule.ChannelID, rule.Type, []byte(rule.Params),
		rule.Enabled, rule.CooldownMinutes,
	)
	if err := row.Scan(&rule.ID, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
		return fmt.Errorf("create alert rule: %w", err)
	}
	return nil
}

// UpdateRule updates an existing rule and returns true when found.
func (s *Store) UpdateRule(ctx context.Context, rule *domain.AlertRule) (bool, error) {
	const q = `
		UPDATE alert_rules
		SET channel_id = $3, type = $4, params = $5, enabled = $6,
		    cooldown_minutes = $7, updated_at = now()
		WHERE user_id = $1 AND id = $2`
	result, err := s.pool.Exec(ctx, q,
		rule.UserID, rule.ID, rule.ChannelID, rule.Type, []byte(rule.Params),
		rule.Enabled, rule.CooldownMinutes,
	)
	if err != nil {
		return false, fmt.Errorf("update alert rule: %w", err)
	}
	return result.RowsAffected() > 0, nil
}

// DeleteRule removes a rule by ID for a user and returns true when deleted.
func (s *Store) DeleteRule(ctx context.Context, userID, ruleID string) (bool, error) {
	const q = `DELETE FROM alert_rules WHERE user_id = $1 AND id = $2`
	result, err := s.pool.Exec(ctx, q, userID, ruleID)
	if err != nil {
		return false, fmt.Errorf("delete alert rule: %w", err)
	}
	return result.RowsAffected() > 0, nil
}

// TouchRuleTriggered updates last_triggered_at to now for the given rule.
func (s *Store) TouchRuleTriggered(ctx context.Context, ruleID string) error {
	const q = `UPDATE alert_rules SET last_triggered_at = now() WHERE id = $1`
	if _, err := s.pool.Exec(ctx, q, ruleID); err != nil {
		return fmt.Errorf("touch rule triggered: %w", err)
	}
	return nil
}
