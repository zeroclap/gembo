// Package bullmq implements hook.Emitter using BullMQ v5 wire format over Redis.
package bullmq

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/zeroclap/gembo/internal/pkg/hook"
)

// BullMQ v5 job wire format.
// See: https://docs.bullmq.io/guide/jobs
type bullJob struct {
	Name      string          `json:"name"`
	Data      json.RawMessage `json:"data"`
	Opts      jobOpts         `json:"opts"`
	Timestamp int64           `json:"timestamp"`
}

type jobOpts struct {
	Attempts         int  `json:"attempts"`
	Backoff          int  `json:"backoff,omitempty"`
	RemoveOnComplete bool `json:"removeOnComplete,omitempty"`
}

// Publisher publishes hook.Events to BullMQ-compatible Redis lists.
type Publisher struct {
	client      *goredis.Client
	queuePrefix string
}

// New creates a new BullMQ Publisher.
func New(client *goredis.Client, queuePrefix string) *Publisher {
	return &Publisher{client: client, queuePrefix: queuePrefix}
}

// Emit encodes e as a BullMQ v5 job and pushes it to the appropriate queue.
func (p *Publisher) Emit(ctx context.Context, e hook.Event) error {
	queueName := p.queueName(e.Type)
	key := fmt.Sprintf("bull:%s:wait", queueName)

	job := bullJob{
		Name:      string(e.Type),
		Data:      e.Payload,
		Timestamp: time.Now().UnixMilli(),
		Opts: jobOpts{
			Attempts:         1,
			RemoveOnComplete: true,
		},
	}

	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal bullmq job: %w", err)
	}

	if err := p.client.LPush(ctx, key, data).Err(); err != nil {
		return fmt.Errorf("lpush to %s: %w", key, err)
	}
	return nil
}

// queueName returns the BullMQ queue name for the given event type.
func (p *Publisher) queueName(t hook.EventType) string {
	switch t {
	case hook.EventTickerUpdated:
		return p.queuePrefix + ".ticker"
	case hook.EventMarketUpdated:
		return p.queuePrefix + ".market"
	case hook.EventFundingUpdated:
		return p.queuePrefix + ".funding"
	default:
		return p.queuePrefix + ".event"
	}
}
