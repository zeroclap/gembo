package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"golang.org/x/sync/errgroup"

	"github.com/zeroclap/gembo/internal/domain"
	"github.com/zeroclap/gembo/internal/exchange"
	bybitex "github.com/zeroclap/gembo/internal/exchange/exchanges/bybit"
	okxex "github.com/zeroclap/gembo/internal/exchange/exchanges/okx"
	"github.com/zeroclap/gembo/internal/hook"
	"github.com/zeroclap/gembo/internal/store"
	"github.com/zeroclap/gembo/pkg/rediskey"
)

const balancePollInterval = 15 * time.Second

// PrivateCollector drives the private-data pipeline (positions, orders,
// balances) for DB-stored user credentials.
type PrivateCollector struct {
	userStore store.UserStore
	store     store.Store
	redis     *goredis.Client
	emitter   hook.Emitter
	logger    *slog.Logger
}

type credentialExchange struct {
	userID       string
	credentialID string
	exchangeID   string
	ex           exchange.Exchange
	watcher      exchange.PositionWatcher
	trading      exchange.TradingExchange
}

// NewPrivate creates a PrivateCollector.
func NewPrivate(
	userStore store.UserStore,
	st store.Store,
	rc *goredis.Client,
	emitter hook.Emitter,
	logger *slog.Logger,
) *PrivateCollector {
	return &PrivateCollector{
		userStore: userStore,
		store:     st,
		redis:     rc,
		emitter:   emitter,
		logger:    logger,
	}
}

// Run starts all private-data exchange pipelines and blocks until ctx is
// cancelled or an unrecoverable error occurs.
func (c *PrivateCollector) Run(ctx context.Context) error {
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

	c.logger.Info("private collector starting",
		"credential_mode", "db_only",
		"credential_clients", len(clients),
	)

	g, ctx := errgroup.WithContext(ctx)
	for _, client := range clients {
		client := client
		g.Go(func() error { return c.runExchange(ctx, client) })
	}

	err = g.Wait()
	c.logger.Info("private collector stopped", "error", err)
	return err
}

func (c *PrivateCollector) loadCredentialExchanges(ctx context.Context) ([]credentialExchange, error) {
	creds, err := c.userStore.ListExchangeCredentials(ctx)
	if err != nil {
		return nil, fmt.Errorf("list exchange credentials: %w", err)
	}

	clients := make([]credentialExchange, 0, len(creds))
	for _, cred := range creds {
		ex, err := newPrivateExchangeFromCredential(c.logger, cred)
		if err != nil {
			c.logger.Warn("skip unsupported credential",
				"credential_id", cred.ID,
				"user_id", cred.UserID,
				"exchange", cred.ExchangeID,
				"error", err,
			)
			continue
		}

		client := credentialExchange{
			userID:       cred.UserID,
			credentialID: cred.ID,
			exchangeID:   cred.ExchangeID,
			ex:           ex,
		}
		if pw, ok := ex.(exchange.PositionWatcher); ok {
			client.watcher = pw
		}
		if tx, ok := ex.(exchange.TradingExchange); ok {
			client.trading = tx
		}
		clients = append(clients, client)
	}

	return clients, nil
}

// runExchange starts all private-data loops for a single credentialed exchange.
func (c *PrivateCollector) runExchange(ctx context.Context, client credentialExchange) error {
	c.logger.Info("starting exchange (private)",
		"exchange", client.exchangeID,
		"user_id", client.userID,
		"credential_id", client.credentialID,
	)

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error { return client.ex.Run(ctx) })

	if client.watcher != nil {
		g.Go(func() error {
			return c.positionLoop(ctx, client.userID, client.credentialID, client.exchangeID, client.watcher)
		})
	} else {
		c.logger.Warn("exchange does not support position watching, skipping",
			"exchange", client.exchangeID,
			"user_id", client.userID,
			"credential_id", client.credentialID,
		)
	}

	if client.trading != nil {
		g.Go(func() error {
			return c.balanceLoop(ctx, client.userID, client.credentialID, client.exchangeID, client.trading)
		})
	} else {
		c.logger.Warn("exchange does not support balance fetching, skipping",
			"exchange", client.exchangeID,
			"user_id", client.userID,
			"credential_id", client.credentialID,
		)
	}

	err := g.Wait()
	c.logger.Info("exchange stopped (private)",
		"exchange", client.exchangeID,
		"user_id", client.userID,
		"credential_id", client.credentialID,
		"error", err,
	)
	return err
}

// balanceLoop periodically fetches balances and stores snapshots in Redis.
func (c *PrivateCollector) balanceLoop(ctx context.Context, userID, credentialID, exchangeID string, tx exchange.TradingExchange) error {
	c.fetchAndStoreBalances(ctx, userID, credentialID, exchangeID, tx)

	ticker := time.NewTicker(balancePollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("balance loop shutting down",
				"exchange", exchangeID,
				"user_id", userID,
				"credential_id", credentialID,
			)
			return ctx.Err()
		case <-ticker.C:
			c.fetchAndStoreBalances(ctx, userID, credentialID, exchangeID, tx)
		}
	}
}

func (c *PrivateCollector) fetchAndStoreBalances(ctx context.Context, userID, credentialID, exchangeID string, tx exchange.TradingExchange) {
	balances, err := tx.FetchBalance(ctx)
	if err != nil {
		c.logger.Warn("fetch balances error",
			"exchange", exchangeID,
			"user_id", userID,
			"credential_id", credentialID,
			"error", err,
		)
		return
	}

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

// positionLoop consumes position updates from an exchange that implements
// PositionWatcher and publishes each position as an ExchangePositionEvent to
// Redis pubsub. The trader subscribes to these events.
func (c *PrivateCollector) positionLoop(ctx context.Context, userID, credentialID, exchangeID string, pw exchange.PositionWatcher) error {
	ch, err := pw.Positions(ctx)
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
					UserID:           userID,
					CredentialID:     credentialID,
					ExchangeID:       exchangeID,
					Symbol:           pos.Symbol,
					Side:             pos.Side,
					Contracts:        pos.Contracts,
					EntryPrice:       pos.EntryPrice,
					MarkPrice:        pos.MarkPrice,
					LiquidationPrice: pos.LiquidationPrice,
					UnrealizedPnL:    pos.UnrealizedPnL,
					Leverage:         pos.Leverage,
					MarginMode:       pos.MarginMode,
					Notional:         pos.Notional,
					Timestamp:        time.Now().UTC(),
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

func newPrivateExchangeFromCredential(logger *slog.Logger, cred domain.ExchangeCredential) (exchange.Exchange, error) {
	switch cred.ExchangeID {
	case "bybit":
		return bybitex.New(logger,
			bybitex.WithAPIKey(cred.APIKey),
			bybitex.WithSecret(cred.APISecret),
		), nil
	case "okx":
		if cred.Passphrase == "" {
			return nil, fmt.Errorf("okx credential %s requires passphrase", cred.ID)
		}
		return okxex.New(logger,
			okxex.WithAPIKey(cred.APIKey),
			okxex.WithSecret(cred.APISecret),
			okxex.WithPassphrase(cred.Passphrase),
		), nil
	default:
		return nil, fmt.Errorf("exchange %s is not supported for private collection", cred.ExchangeID)
	}
}
