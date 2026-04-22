// Package accountdatacollector drives the private account-data pipeline
// (positions, balances, funding history) for DB-stored user credentials.
package accountdatacollector

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"golang.org/x/sync/errgroup"

	"github.com/zeroclap/gembo/internal/domain"
	"github.com/zeroclap/gembo/internal/exchanges"
	"github.com/zeroclap/gembo/internal/exchanges/base"
	"github.com/zeroclap/gembo/internal/pkg/hook"
	"github.com/zeroclap/gembo/internal/store"
	"github.com/zeroclap/gembo/pkg/rediskey"
)

// Collector drives the private-data pipeline.
type Collector struct {
	userStore store.UserStore
	store     store.Store
	redis     *goredis.Client
	emitter   hook.Emitter
	logger    *slog.Logger
}

// New creates a Collector.
func New(
	userStore store.UserStore,
	st store.Store,
	rc *goredis.Client,
	emitter hook.Emitter,
	logger *slog.Logger,
) *Collector {
	return &Collector{
		userStore: userStore,
		store:     st,
		redis:     rc,
		emitter:   emitter,
		logger:    logger,
	}
}

// Run starts all private-data exchange pipelines and blocks until ctx is
// cancelled or an unrecoverable error occurs.
func (c *Collector) Run(ctx context.Context) error {
	if c.userStore == nil {
		return fmt.Errorf("user store is required for credential-based private collector")
	}

	clients, err := c.loadCredentialExchanges(ctx)
	if err != nil {
		return fmt.Errorf("load private collector credentials: %w", err)
	}
	if len(clients) == 0 {
		return fmt.Errorf("no supported exchange credentials found")
	}

	c.logger.Info("account-data collector starting",
		"credential_mode", "db_only",
		"credential_clients", len(clients),
	)

	g, ctx := errgroup.WithContext(ctx)
	for _, client := range clients {
		client := client
		g.Go(func() error { return c.runExchange(ctx, client) })
	}

	err = g.Wait()
	c.logger.Info("account-data collector stopped", "error", err)
	return err
}

type credentialExchange struct {
	userID       string
	credentialID string
	exchangeID   string
	ex           base.PrivateExchange
}

func (c *Collector) loadCredentialExchanges(ctx context.Context) ([]credentialExchange, error) {
	creds, err := c.userStore.ListExchangeCredentials(ctx)
	if err != nil {
		return nil, fmt.Errorf("list exchange credentials: %w", err)
	}

	clients := make([]credentialExchange, 0, len(creds))
	for _, cred := range creds {
		ex, err := exchanges.NewPrivateExchangeFromCredential(c.logger, cred)
		if err != nil {
			c.logger.Warn("skip unsupported credential",
				"credential_id", cred.ID,
				"user_id", cred.UserID,
				"exchange", cred.ExchangeID,
				"error", err,
			)
			continue
		}
		clients = append(clients, credentialExchange{
			userID:       cred.UserID,
			credentialID: cred.ID,
			exchangeID:   cred.ExchangeID,
			ex:           ex,
		})
	}

	return clients, nil
}

func (c *Collector) runExchange(ctx context.Context, client credentialExchange) error {
	c.logger.Info("starting exchange (private)",
		"exchange", client.exchangeID,
		"user_id", client.userID,
		"credential_id", client.credentialID,
	)

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error { return client.ex.Run(ctx) })
	g.Go(func() error {
		return c.positionLoop(ctx, client.userID, client.credentialID, client.exchangeID, client.ex)
	})
	g.Go(func() error {
		return c.balanceLoop(ctx, client.userID, client.credentialID, client.exchangeID, client.ex)
	})
	g.Go(func() error {
		return c.fundingHistoryLoop(ctx, client.userID, client.credentialID, client.exchangeID, client.ex)
	})

	err := g.Wait()
	c.logger.Info("exchange stopped (private)",
		"exchange", client.exchangeID,
		"user_id", client.userID,
		"credential_id", client.credentialID,
		"error", err,
	)
	return err
}

func (c *Collector) positionLoop(ctx context.Context, userID, credentialID, exchangeID string, ex base.PrivateExchange) error {
	ch, err := ex.Positions(ctx)
	if err != nil {
		return fmt.Errorf("start position watcher for %s: %w", exchangeID, err)
	}

	c.logger.Info("position loop started",
		"exchange", exchangeID,
		"user_id", userID,
		"credential_id", credentialID,
	)

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("position loop shutting down",
				"exchange", exchangeID,
				"user_id", userID,
				"credential_id", credentialID,
			)
			return ctx.Err()
		case positions, ok := <-ch:
			if !ok {
				c.logger.Info("position channel closed",
					"exchange", exchangeID,
					"user_id", userID,
					"credential_id", credentialID,
				)
				return nil
			}
			for _, pos := range positions {
				event := domain.ExchangePositionEvent{
					UserID:            userID,
					CredentialID:      credentialID,
					ExchangeID:        exchangeID,
					Symbol:            pos.Symbol,
					Side:              pos.Side,
					Contracts:         pos.Contracts,
					EntryPrice:        pos.EntryPrice,
					MarkPrice:         pos.MarkPrice,
					LiquidationPrice:  pos.LiquidationPrice,
					UnrealizedPnL:     pos.UnrealizedPnL,
					Leverage:          pos.Leverage,
					MarginMode:        pos.MarginMode,
					Notional:          pos.Notional,
					InitialMargin:     pos.InitialMargin,
					MaintenanceMargin: pos.MaintenanceMargin,
					MarginRatio:       pos.MarginRatio,
					Collateral:        pos.Collateral,
					AdlQuantile:       pos.AdlQuantile,
					Timestamp:         time.Now().UTC(),
				}
				payload, err := json.Marshal(event)
				if err != nil {
					c.logger.Error("marshal position event", "error", err)
					continue
				}
				if err := c.redis.Publish(ctx, rediskey.UserPositionStatePubSub(userID, exchangeID), payload).Err(); err != nil {
					c.logger.Warn("publish position event",
						"exchange", exchangeID,
						"user_id", userID,
						"credential_id", credentialID,
						"symbol", pos.Symbol,
						"error", err,
					)
				}
			}
		}
	}
}

func (c *Collector) balanceLoop(ctx context.Context, userID, credentialID, exchangeID string, ex base.PrivateExchange) error {
	ch, err := ex.Balances(ctx)
	if err != nil {
		return fmt.Errorf("start balance stream for %s: %w", exchangeID, err)
	}

	c.logger.Info("balance loop started",
		"exchange", exchangeID,
		"user_id", userID,
		"credential_id", credentialID,
	)

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("balance loop shutting down",
				"exchange", exchangeID,
				"user_id", userID,
				"credential_id", credentialID,
			)
			return ctx.Err()
		case balances, ok := <-ch:
			if !ok {
				return nil
			}
			c.storeBalances(ctx, userID, credentialID, exchangeID, balances)
		}
	}
}

func (c *Collector) storeBalances(ctx context.Context, userID, credentialID, exchangeID string, balances []base.Balance) {
	now := time.Now().UTC()
	for _, b := range balances {
		db := domain.Balance{
			UserID:       userID,
			CredentialID: credentialID,
			ExchangeID:   exchangeID,
			Currency:     b.Currency,
			Free:         b.Free,
			Used:         b.Used,
			Total:        b.Total,
			UpdatedAt:    now,
		}
		if err := c.store.SetBalance(ctx, db); err != nil {
			c.logger.Warn("store balance error",
				"exchange", exchangeID,
				"user_id", userID,
				"credential_id", credentialID,
				"currency", b.Currency,
				"error", err,
			)
		}
	}

	c.logger.Debug("balances updated",
		"exchange", exchangeID,
		"user_id", userID,
		"credential_id", credentialID,
		"count", len(balances),
	)
}

func (c *Collector) fundingHistoryLoop(ctx context.Context, userID, credentialID, exchangeID string, ex base.PrivateExchange) error {
	ch, err := ex.FundingHistory(ctx)
	if err != nil {
		return fmt.Errorf("start funding history stream for %s: %w", exchangeID, err)
	}

	c.logger.Info("funding history loop started",
		"exchange", exchangeID,
		"user_id", userID,
		"credential_id", credentialID,
	)

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("funding history loop shutting down",
				"exchange", exchangeID,
				"user_id", userID,
				"credential_id", credentialID,
			)
			return ctx.Err()
		case rows, ok := <-ch:
			if !ok {
				return nil
			}
			c.publishFundingHistory(ctx, userID, credentialID, exchangeID, rows)
		}
	}
}

func (c *Collector) publishFundingHistory(ctx context.Context, userID, credentialID, exchangeID string, rows []base.FundingHistoryEntry) {
	for _, row := range rows {
		if row.Timestamp.IsZero() {
			continue
		}

		event := domain.ExchangeFundingPaymentEvent{
			UserID:       userID,
			CredentialID: credentialID,
			ExchangeID:   exchangeID,
			PaymentID:    row.ID,
			Symbol:       row.Symbol,
			Amount:       row.Amount,
			Currency:     row.Currency,
			Timestamp:    row.Timestamp,
		}
		payload, err := json.Marshal(event)
		if err != nil {
			c.logger.Error("marshal funding payment event", "error", err)
			continue
		}

		if err := c.redis.Publish(ctx, rediskey.UserFundingPaymentPubSub(userID, exchangeID), payload).Err(); err != nil {
			c.logger.Warn("publish funding payment event",
				"exchange", exchangeID,
				"user_id", userID,
				"credential_id", credentialID,
				"symbol", row.Symbol,
				"payment_id", row.ID,
				"error", err,
			)
		}
	}

	if len(rows) > 0 {
		c.logger.Debug("funding history fetched",
			"exchange", exchangeID,
			"user_id", userID,
			"credential_id", credentialID,
			"rows", len(rows),
		)
	}
}
