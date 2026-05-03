package arbitrageworker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"

	"github.com/zeroclap/gembo/internal/domain"
	"github.com/zeroclap/gembo/internal/notification"
	"github.com/zeroclap/gembo/internal/notification/telegram"
	"github.com/zeroclap/gembo/internal/store"
)

// AlertWorker evaluates alert rules against freshly computed arbitrage data
// and dispatches notifications through the configured channels.
//
// It subscribes to Redis Pub/Sub arbitrage update events so evaluation
// happens right after the calculator publishes new opportunities.
type AlertWorker struct {
	notifStore store.NotificationStore
	redisStore store.Store
	redis      *goredis.Client
	logger     *slog.Logger
}

// NewAlertWorker creates an AlertWorker.
func NewAlertWorker(
	ns store.NotificationStore,
	rs store.Store,
	rc *goredis.Client,
	logger *slog.Logger,
) *AlertWorker {
	return &AlertWorker{
		notifStore: ns,
		redisStore: rs,
		redis:      rc,
		logger:     logger,
	}
}

// Run subscribes to arbitrage updates and evaluates alert rules.
// Blocks until ctx is cancelled.
func (w *AlertWorker) Run(ctx context.Context) error {
	pubsub := w.redis.Subscribe(ctx,
		"gembo:arbitrage:funding:updates",
		"gembo:arbitrage:spread:updates",
	)
	defer pubsub.Close()

	ch := pubsub.Channel()

	const debounce = 3 * time.Second
	timer := time.NewTimer(debounce)
	timer.Stop()
	pending := false

	for {
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()

		case _, ok := <-ch:
			if !ok {
				return nil
			}
			if !pending {
				timer.Reset(debounce)
				pending = true
			}

		case <-timer.C:
			pending = false
			w.evaluate(ctx)
		}
	}
}

func (w *AlertWorker) evaluate(ctx context.Context) {
	rules, err := w.notifStore.ListAllRules(ctx)
	if err != nil {
		w.logger.Warn("alert worker list rules", "error", err)
		return
	}

	// Group enabled rules that are past cooldown.
	now := time.Now()
	active := rules[:0]
	for _, r := range rules {
		if !r.Enabled {
			continue
		}
		if r.LastTriggeredAt != nil {
			cooldown := time.Duration(r.CooldownMinutes) * time.Minute
			if now.Sub(*r.LastTriggeredAt) < cooldown {
				continue
			}
		}
		active = append(active, r)
	}
	if len(active) == 0 {
		return
	}

	// Load channels for all referenced channel IDs.
	channelMap := map[string]*domain.NotificationChannel{}
	for _, r := range active {
		if _, ok := channelMap[r.ChannelID]; ok {
			continue
		}
		ch, err := w.notifStore.GetChannel(ctx, r.UserID, r.ChannelID)
		if err != nil || ch == nil || !ch.Enabled {
			continue
		}
		channelMap[r.ChannelID] = ch
	}

	// Evaluate market-data alert types.
	for _, r := range active {
		ch, ok := channelMap[r.ChannelID]
		if !ok {
			continue
		}
		var msg *notification.Message
		var evalErr error

		switch r.Type {
		case domain.AlertFundingAPR:
			msg, evalErr = w.evalFundingAPR(ctx, r)
		case domain.AlertSpreadThreshold:
			msg, evalErr = w.evalSpreadThreshold(ctx, r)
		// Position-based rules are triggered by the position manager process;
		// here we only handle market-data rules.
		default:
			continue
		}

		if evalErr != nil {
			w.logger.Warn("alert eval", "rule", r.ID, "type", r.Type, "error", evalErr)
			continue
		}
		if msg == nil {
			continue
		}

		sender, err := buildNotifChannel(ch)
		if err != nil {
			w.logger.Warn("alert build channel", "channel", ch.ID, "error", err)
			continue
		}

		if err := sender.Send(ctx, *msg); err != nil {
			w.logger.Warn("alert send", "channel", ch.ID, "error", err)
			continue
		}

		if err := w.notifStore.TouchRuleTriggered(ctx, r.ID); err != nil {
			w.logger.Warn("alert touch triggered", "rule", r.ID, "error", err)
		}
		w.logger.Info("alert fired", "rule", r.ID, "type", r.Type)
	}
}

// evalFundingAPR checks whether any funding arbitrage opportunity exceeds the
// rule's minimum APR threshold.
func (w *AlertWorker) evalFundingAPR(ctx context.Context, rule domain.AlertRule) (*notification.Message, error) {
	var params domain.FundingAPRParams
	if err := json.Unmarshal(rule.Params, &params); err != nil {
		return nil, fmt.Errorf("unmarshal funding apr params: %w", err)
	}
	if params.MinAPR.IsZero() {
		return nil, nil
	}

	opps, err := w.redisStore.ListFundingArbitrage(ctx)
	if err != nil {
		return nil, fmt.Errorf("list funding arbitrage: %w", err)
	}

	for _, opp := range opps {
		if params.Symbol != "" && opp.Symbol != params.Symbol {
			continue
		}
		if params.LongExchangeID != "" && opp.Long.ExchangeID != params.LongExchangeID {
			continue
		}
		if params.ShortExchangeID != "" && opp.Short.ExchangeID != params.ShortExchangeID {
			continue
		}
		if opp.APR.LessThan(params.MinAPR) {
			continue
		}

		msg := notification.Message{
			Title: fmt.Sprintf("Funding APR Alert: %s", opp.Symbol),
			Body: fmt.Sprintf(
				"APR: %s%%\nLong: %s | Short: %s\nNet Rate 8h: %s%%\nOpen Spread: %s%%",
				formatPct(opp.APR),
				opp.Long.ExchangeID, opp.Short.ExchangeID,
				formatPct(opp.NetRate8h),
				formatPct(opp.OpenSpread),
			),
		}
		return &msg, nil
	}
	return nil, nil
}

// evalSpreadThreshold checks whether any spread arbitrage opportunity exceeds
// the rule's minimum spread percentage threshold.
func (w *AlertWorker) evalSpreadThreshold(ctx context.Context, rule domain.AlertRule) (*notification.Message, error) {
	var params domain.SpreadThresholdParams
	if err := json.Unmarshal(rule.Params, &params); err != nil {
		return nil, fmt.Errorf("unmarshal spread threshold params: %w", err)
	}
	if params.MinSpreadPct.IsZero() {
		return nil, nil
	}

	opps, err := w.redisStore.ListSpreadArbitrage(ctx)
	if err != nil {
		return nil, fmt.Errorf("list spread arbitrage: %w", err)
	}

	for _, opp := range opps {
		if params.Symbol != "" && opp.Symbol != params.Symbol {
			continue
		}
		if opp.NetSpreadPct.LessThan(params.MinSpreadPct) {
			continue
		}

		msg := notification.Message{
			Title: fmt.Sprintf("Spread Alert: %s", opp.Symbol),
			Body: fmt.Sprintf(
				"Net Spread: %s%%\nBuy: %s | Sell: %s",
				formatPct(opp.NetSpreadPct),
				opp.BuyExchangeID, opp.SellExchangeID,
			),
		}
		return &msg, nil
	}
	return nil, nil
}

func formatPct(d decimal.Decimal) string {
	return d.StringFixed(4)
}

// buildNotifChannel constructs a notification.Channel from a stored NotificationChannel.
func buildNotifChannel(ch *domain.NotificationChannel) (notification.Channel, error) {
	switch ch.Type {
	case domain.ChannelTypeTelegram:
		var cfg domain.TelegramConfig
		if err := json.Unmarshal(ch.Config, &cfg); err != nil {
			return nil, fmt.Errorf("invalid telegram config: %w", err)
		}
		if cfg.BotToken == "" || cfg.ChatID == "" {
			return nil, fmt.Errorf("telegram channel requires bot_token and chat_id")
		}
		return telegram.New(cfg.BotToken, cfg.ChatID), nil
	default:
		return &notification.NoopChannel{}, nil
	}
}
