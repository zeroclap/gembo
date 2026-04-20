package trader

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/shopspring/decimal"

	"github.com/zeroclap/gembo/internal/domain"
	"github.com/zeroclap/gembo/pkg/rediskey"
)

func (t *Trader) runFundingEventReactor(ctx context.Context) error {
	channels := []string{rediskey.UserFundingPaymentPattern()}

	pubsub := t.redis.PSubscribe(ctx, channels...)
	defer pubsub.Close()

	ch := pubsub.Channel()

	t.logger.Info("funding reactor started", "channels", channels)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg, ok := <-ch:
			if !ok {
				return nil
			}

			var event domain.ExchangeFundingPaymentEvent
			if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
				t.logger.Warn("funding reactor: unmarshal event", "error", err)
				continue
			}
			if event.UserID == "" {
				event.UserID = rediskey.UserIDFromChannel(msg.Channel)
			}

			if err := t.handleFundingPaymentEvent(ctx, &event); err != nil {
				t.logger.Error("funding reactor: handle event",
					"exchange", event.ExchangeID,
					"symbol", event.Symbol,
					"payment_id", event.PaymentID,
					"error", err,
				)
			}
		}
	}
}

func (t *Trader) handleFundingPaymentEvent(ctx context.Context, event *domain.ExchangeFundingPaymentEvent) error {
	if event.UserID == "" || event.ExchangeID == "" || event.Symbol == "" || event.Timestamp.IsZero() {
		return nil
	}

	positions, err := t.store.ListPositions(ctx, domain.PositionFilter{
		UserID: event.UserID,
		Symbol: event.Symbol,
		Status: []domain.PositionStatus{
			domain.PositionStatusOpen,
			domain.PositionStatusPartiallyOpen,
		},
	})
	if err != nil {
		return fmt.Errorf("list positions for funding event: %w", err)
	}

	for i := range positions {
		p := &positions[i]
		leg := fundingLegForExchange(p, event.ExchangeID)
		if leg == "" {
			continue
		}

		openedAt := p.CreatedAt
		if p.OpenedAt != nil {
			openedAt = *p.OpenedAt
		}
		if event.Timestamp.Before(openedAt) {
			continue
		}

		fp := &domain.FundingPayment{
			ExternalID: event.PaymentID,
			UserID:     p.UserID,
			PositionID: p.ID,
			ExchangeID: event.ExchangeID,
			Leg:        leg,
			Symbol:     event.Symbol,
			Rate:       decimal.Zero,
			Payment:    event.Amount,
			FundedAt:   event.Timestamp.UTC(),
		}

		inserted, err := t.store.CreateFundingPaymentIfNotExists(ctx, fp)
		if err != nil {
			return fmt.Errorf("create funding payment: %w", err)
		}
		if !inserted {
			continue
		}

		if p.Status == domain.PositionStatusOpen || p.Status == domain.PositionStatusPartiallyOpen {
			fundingPnL, err := t.store.SumFundingPnL(ctx, p.UserID, p.ID)
			if err == nil {
				p.TotalFundingPnL = fundingPnL
				if err := t.store.UpdatePosition(ctx, p); err != nil {
					t.logger.Warn("funding reactor: update position funding total",
						"position_id", p.ID,
						"error", err,
					)
				}
			} else {
				t.logger.Warn("funding reactor: sum funding pnl",
					"position_id", p.ID,
					"error", err,
				)
			}
		}

		t.logger.Info("funding payment recorded from collector event",
			"position_id", p.ID,
			"exchange", event.ExchangeID,
			"leg", leg,
			"symbol", event.Symbol,
			"payment", event.Amount,
			"payment_id", event.PaymentID,
			"funded_at", event.Timestamp.UTC(),
		)

		t.redis.Publish(ctx, rediskey.UserPositionUpdates(p.UserID), p.ID)
	}

	return nil
}

func fundingLegForExchange(p *domain.Position, exchangeID string) string {
	if p.LongExchangeID == exchangeID {
		return "long"
	}
	if p.ShortExchangeID == exchangeID {
		return "short"
	}
	return ""
}
