package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/zeroclap/gembo/internal/domain"
	"github.com/zeroclap/gembo/pkg/rediskey"
)

const snapshotRetention = time.Hour

// AppendTickerSnapshot adds a snapshot to the exchange's sorted set keyed by
// unix timestamp, trims entries older than 1h, and publishes to the snapshot
// pub/sub channel — all in a single pipeline.
func (s *Store) AppendTickerSnapshot(ctx context.Context, snap domain.TickerSnapshot) error {
	data, err := json.Marshal(snap)
	if err != nil {
		return fmt.Errorf("marshal ticker snapshot: %w", err)
	}

	key := rediskey.TickerSnapshotKey(snap.ExchangeID, snap.Symbol)
	cutoff := fmt.Sprintf("%d", time.Now().Add(-snapshotRetention).Unix())

	pipe := s.client.TxPipeline()
	pipe.ZAdd(ctx, key, goredis.Z{Score: float64(snap.TS), Member: string(data)})
	pipe.ZRemRangeByScore(ctx, key, "0", cutoff)
	pipe.Publish(ctx, rediskey.TickerSnapshotPubSub(), string(data))

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("append ticker snapshot %s/%s: %w", snap.ExchangeID, snap.Symbol, err)
	}
	return nil
}

// GetTickerSnapshots returns all snapshots for an exchange+symbol within
// [from, to] unix seconds (inclusive), ordered by timestamp ascending.
func (s *Store) GetTickerSnapshots(ctx context.Context, exchangeID, symbol string, from, to int64) ([]domain.TickerSnapshot, error) {
	key := rediskey.TickerSnapshotKey(exchangeID, symbol)

	results, err := s.client.ZRangeByScore(ctx, key, &goredis.ZRangeBy{
		Min: fmt.Sprintf("%d", from),
		Max: fmt.Sprintf("%d", to),
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("get ticker snapshots %s/%s: %w", exchangeID, symbol, err)
	}

	snaps := make([]domain.TickerSnapshot, 0, len(results))
	for _, raw := range results {
		var snap domain.TickerSnapshot
		if err := json.Unmarshal([]byte(raw), &snap); err != nil {
			continue
		}
		snaps = append(snaps, snap)
	}
	return snaps, nil
}
