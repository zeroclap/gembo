package arbitrageworker

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/zeroclap/gembo/internal/domain"
	"github.com/zeroclap/gembo/internal/notification"
	"github.com/zeroclap/gembo/internal/store"
	"github.com/zeroclap/gembo/pkg/rediskey"
)

// PositionAlertWorker dispatches position lifecycle alert notifications when
// positions are opened or closed. It subscribes to the user-scoped position
// updates channel that the position manager publishes to on each state change.
type PositionAlertWorker struct {
	notifStore    store.NotificationStore
	positionStore store.PositionStore
	redis         *goredis.Client
	logger        *slog.Logger
}

// NewPositionAlertWorker creates a PositionAlertWorker.
func NewPositionAlertWorker(
	ns store.NotificationStore,
	ps store.PositionStore,
	rc *goredis.Client,
	logger *slog.Logger,
) *PositionAlertWorker {
	return &PositionAlertWorker{
		notifStore:    ns,
		positionStore: ps,
		redis:         rc,
		logger:        logger,
	}
}

// Run subscribes to position update events and evaluates position-based alert
// rules. Blocks until ctx is cancelled.
func (w *PositionAlertWorker) Run(ctx context.Context) error {
	pubsub := w.redis.PSubscribe(ctx, rediskey.UserPositionUpdatesPattern())
	defer pubsub.Close()

	ch := pubsub.Channel()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg, ok := <-ch:
			if !ok {
				return nil
			}
			userID := rediskey.UserIDFromChannel(msg.Channel)
			if userID == "" {
				continue
			}
			positionID := msg.Payload
			w.handlePositionEvent(ctx, userID, positionID)
		}
	}
}

func (w *PositionAlertWorker) handlePositionEvent(ctx context.Context, userID, positionID string) {
	p, err := w.positionStore.GetPosition(ctx, userID, positionID)
	if err != nil || p == nil {
		return
	}

	// Only fire on terminal transitions.
	var ruleType domain.AlertRuleType
	switch p.Status {
	case domain.PositionStatusOpen:
		ruleType = domain.AlertPositionOpened
	case domain.PositionStatusClosed:
		ruleType = domain.AlertPositionClosed
	default:
		return
	}

	rules, err := w.notifStore.ListRules(ctx, userID)
	if err != nil {
		w.logger.Warn("position alert list rules", "user", userID, "error", err)
		return
	}

	now := time.Now()
	for _, rule := range rules {
		if rule.Type != ruleType || !rule.Enabled {
			continue
		}
		if rule.LastTriggeredAt != nil {
			cooldown := time.Duration(rule.CooldownMinutes) * time.Minute
			if now.Sub(*rule.LastTriggeredAt) < cooldown {
				continue
			}
		}

		ch, err := w.notifStore.GetChannel(ctx, userID, rule.ChannelID)
		if err != nil || ch == nil || !ch.Enabled {
			continue
		}

		msg := buildPositionMessage(ruleType, p)
		sender, err := buildNotifChannel(ch)
		if err != nil {
			w.logger.Warn("position alert build channel", "channel", ch.ID, "error", err)
			continue
		}
		if err := sender.Send(ctx, msg); err != nil {
			w.logger.Warn("position alert send", "channel", ch.ID, "error", err)
			continue
		}
		if err := w.notifStore.TouchRuleTriggered(ctx, rule.ID); err != nil {
			w.logger.Warn("position alert touch triggered", "rule", rule.ID, "error", err)
		}
		w.logger.Info("position alert fired", "rule", rule.ID, "type", ruleType, "position", p.ID)
	}
}

func buildPositionMessage(ruleType domain.AlertRuleType, p *domain.Position) notification.Message {
	switch ruleType {
	case domain.AlertPositionOpened:
		return notification.Message{
			Title: fmt.Sprintf("Position Opened: %s", p.Symbol),
			Body: fmt.Sprintf(
				"ID: %s\nLong: %s | Short: %s\nSize: %s %s\nLeverage: %dx",
				p.ID,
				p.LongExchangeID, p.ShortExchangeID,
				p.RequestedSize.StringFixed(2), string(p.SizeType),
				p.Leverage,
			),
		}
	case domain.AlertPositionClosed:
		return notification.Message{
			Title: fmt.Sprintf("Position Closed: %s", p.Symbol),
			Body: fmt.Sprintf(
				"ID: %s\nLong: %s | Short: %s\nClose reason: %s\nFunding P&L: %s",
				p.ID,
				p.LongExchangeID, p.ShortExchangeID,
				string(p.CloseReason),
				p.TotalFundingPnL.StringFixed(4),
			),
		}
	default:
		return notification.Message{
			Title: fmt.Sprintf("Position Update: %s", p.Symbol),
			Body:  fmt.Sprintf("Position %s status: %s", p.ID, string(p.Status)),
		}
	}
}
