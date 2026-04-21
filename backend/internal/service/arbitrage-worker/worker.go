// Package arbitrageworker subscribes to Redis Pub/Sub events (ticker and
// funding rate updates) and recomputes arbitrage opportunities on change.
package arbitrageworker

import (
	"context"
	"log/slog"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/zeroclap/gembo/internal/pkg/arbitrage"
)

const debounceInterval = 2 * time.Second

// Worker recomputes funding and spread arbitrage whenever market data changes.
type Worker struct {
	calc        *arbitrage.Calculator
	spreadCalc  *arbitrage.SpreadCalculator
	exchangeIDs []string
	redis       *goredis.Client
	logger      *slog.Logger
}

// New creates a Worker.
func New(
	rc *goredis.Client,
	calc *arbitrage.Calculator,
	spreadCalc *arbitrage.SpreadCalculator,
	exchangeIDs []string,
	logger *slog.Logger,
) *Worker {
	return &Worker{
		calc:        calc,
		spreadCalc:  spreadCalc,
		exchangeIDs: exchangeIDs,
		redis:       rc,
		logger:      logger,
	}
}

// Run subscribes to Redis Pub/Sub channels and triggers recomputation on
// debounced events. Blocks until ctx is cancelled.
func (w *Worker) Run(ctx context.Context) error {
	pubsub := w.redis.PSubscribe(ctx,
		"gembo:*:ticker:updates",
		"gembo:*:funding:updates",
	)
	defer pubsub.Close()

	ch := pubsub.Channel()

	timer := time.NewTimer(debounceInterval)
	timer.Stop()

	pending := false

	for {
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()

		case msg, ok := <-ch:
			if !ok {
				return nil
			}
			w.logger.Debug("pubsub event", "channel", msg.Channel)
			if !pending {
				timer.Reset(debounceInterval)
				pending = true
			}

		case <-timer.C:
			pending = false
			start := time.Now()
			if err := w.calc.Recompute(ctx, w.exchangeIDs); err != nil {
				w.logger.Warn("funding recompute failed", "error", err)
			}
			if err := w.spreadCalc.Recompute(ctx, w.exchangeIDs); err != nil {
				w.logger.Warn("spread recompute failed", "error", err)
			}
			w.logger.Debug("recompute done", "duration", time.Since(start))
		}
	}
}
