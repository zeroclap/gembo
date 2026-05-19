// Package redis implements the store.Store interface using Redis.
package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/zeroclap/gembo/internal/domain"
	"github.com/zeroclap/gembo/pkg/rediskey"
)

const tickerTTL = 60 * time.Second

// Store implements store.Store using Redis.
type Store struct {
	client *goredis.Client
}

// New creates a new Redis-backed Store.
func New(client *goredis.Client) *Store {
	return &Store{client: client}
}

// --- Markets ---

// SetMarket stores a Market and adds its symbol to the exchange's markets set.
func (s *Store) SetMarket(ctx context.Context, m domain.Market) error {
	data, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshal market: %w", err)
	}

	pipe := s.client.TxPipeline()
	pipe.Set(ctx, rediskey.Market(m.ExchangeID, m.Symbol), data, 0)
	pipe.SAdd(ctx, rediskey.Markets(m.ExchangeID), m.Symbol)

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("store market %s/%s: %w", m.ExchangeID, m.Symbol, err)
	}
	return nil
}

// GetMarket retrieves a Market by exchange and symbol.
func (s *Store) GetMarket(ctx context.Context, exchangeID, symbol string) (*domain.Market, error) {
	data, err := s.client.Get(ctx, rediskey.Market(exchangeID, symbol)).Bytes()
	if err == goredis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get market %s/%s: %w", exchangeID, symbol, err)
	}

	var m domain.Market
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("unmarshal market %s/%s: %w", exchangeID, symbol, err)
	}
	return &m, nil
}

// ListMarkets returns all markets for an exchange.
func (s *Store) ListMarkets(ctx context.Context, exchangeID string) ([]domain.Market, error) {
	symbols, err := s.client.SMembers(ctx, rediskey.Markets(exchangeID)).Result()
	if err != nil {
		return nil, fmt.Errorf("list market symbols for %s: %w", exchangeID, err)
	}
	if len(symbols) == 0 {
		return nil, nil
	}

	keys := make([]string, len(symbols))
	for i, sym := range symbols {
		keys[i] = rediskey.Market(exchangeID, sym)
	}

	vals, err := s.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, fmt.Errorf("mget markets for %s: %w", exchangeID, err)
	}

	markets := make([]domain.Market, 0, len(vals))
	for i, v := range vals {
		if v == nil {
			continue
		}
		raw, ok := v.(string)
		if !ok {
			continue
		}
		var m domain.Market
		if err := json.Unmarshal([]byte(raw), &m); err != nil {
			return nil, fmt.Errorf("unmarshal market %s/%s: %w", exchangeID, symbols[i], err)
		}
		markets = append(markets, m)
	}
	return markets, nil
}

// --- Tickers ---

// SetTicker stores a Ticker with a 60 s TTL, adds the symbol to the exchange's
// tickers set, and publishes a Pub/Sub notification.
func (s *Store) SetTicker(ctx context.Context, t domain.Ticker) error {
	data, err := json.Marshal(t)
	if err != nil {
		return fmt.Errorf("marshal ticker: %w", err)
	}

	pipe := s.client.TxPipeline()
	pipe.Set(ctx, rediskey.Ticker(t.ExchangeID, t.Symbol), data, tickerTTL)
	pipe.SAdd(ctx, rediskey.Tickers(t.ExchangeID), t.Symbol)
	pipe.Publish(ctx, rediskey.TickerPubSub(t.ExchangeID), data)

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("store ticker %s/%s: %w", t.ExchangeID, t.Symbol, err)
	}
	return nil
}

// GetTicker retrieves a Ticker by exchange and symbol.
func (s *Store) GetTicker(ctx context.Context, exchangeID, symbol string) (*domain.Ticker, error) {
	data, err := s.client.Get(ctx, rediskey.Ticker(exchangeID, symbol)).Bytes()
	if err == goredis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get ticker %s/%s: %w", exchangeID, symbol, err)
	}

	var t domain.Ticker
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, fmt.Errorf("unmarshal ticker %s/%s: %w", exchangeID, symbol, err)
	}
	return &t, nil
}

// ListTickers returns all tickers for an exchange.
func (s *Store) ListTickers(ctx context.Context, exchangeID string) ([]domain.Ticker, error) {
	symbols, err := s.client.SMembers(ctx, rediskey.Tickers(exchangeID)).Result()
	if err != nil {
		return nil, fmt.Errorf("list ticker symbols for %s: %w", exchangeID, err)
	}
	if len(symbols) == 0 {
		return nil, nil
	}

	keys := make([]string, len(symbols))
	for i, sym := range symbols {
		keys[i] = rediskey.Ticker(exchangeID, sym)
	}

	vals, err := s.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, fmt.Errorf("mget tickers for %s: %w", exchangeID, err)
	}

	tickers := make([]domain.Ticker, 0, len(vals))
	for i, v := range vals {
		if v == nil {
			// Ticker expired (TTL), remove stale symbol from the set.
			s.client.SRem(ctx, rediskey.Tickers(exchangeID), symbols[i])
			continue
		}
		raw, ok := v.(string)
		if !ok {
			continue
		}
		var t domain.Ticker
		if err := json.Unmarshal([]byte(raw), &t); err != nil {
			return nil, fmt.Errorf("unmarshal ticker %s/%s: %w", exchangeID, symbols[i], err)
		}
		tickers = append(tickers, t)
	}
	return tickers, nil
}

// --- Funding Rates ---

// SetFundingRate stores a FundingRate and adds its symbol to the exchange's funding set.
func (s *Store) SetFundingRate(ctx context.Context, fr domain.FundingRate) error {
	data, err := json.Marshal(fr)
	if err != nil {
		return fmt.Errorf("marshal funding rate: %w", err)
	}

	pipe := s.client.TxPipeline()
	pipe.Set(ctx, rediskey.FundingRate(fr.ExchangeID, fr.Symbol), data, 0)
	pipe.SAdd(ctx, rediskey.FundingRates(fr.ExchangeID), fr.Symbol)
	pipe.Publish(ctx, rediskey.FundingRatePubSub(fr.ExchangeID), data)

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("store funding rate %s/%s: %w", fr.ExchangeID, fr.Symbol, err)
	}
	return nil
}

// GetFundingRate retrieves a FundingRate by exchange and symbol.
func (s *Store) GetFundingRate(ctx context.Context, exchangeID, symbol string) (*domain.FundingRate, error) {
	data, err := s.client.Get(ctx, rediskey.FundingRate(exchangeID, symbol)).Bytes()
	if err == goredis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get funding rate %s/%s: %w", exchangeID, symbol, err)
	}

	var fr domain.FundingRate
	if err := json.Unmarshal(data, &fr); err != nil {
		return nil, fmt.Errorf("unmarshal funding rate %s/%s: %w", exchangeID, symbol, err)
	}
	return &fr, nil
}

// ListFundingRates returns all funding rates for an exchange.
func (s *Store) ListFundingRates(ctx context.Context, exchangeID string) ([]domain.FundingRate, error) {
	symbols, err := s.client.SMembers(ctx, rediskey.FundingRates(exchangeID)).Result()
	if err != nil {
		return nil, fmt.Errorf("list funding rate symbols for %s: %w", exchangeID, err)
	}
	if len(symbols) == 0 {
		return nil, nil
	}

	keys := make([]string, len(symbols))
	for i, sym := range symbols {
		keys[i] = rediskey.FundingRate(exchangeID, sym)
	}

	vals, err := s.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, fmt.Errorf("mget funding rates for %s: %w", exchangeID, err)
	}

	rates := make([]domain.FundingRate, 0, len(vals))
	for i, v := range vals {
		if v == nil {
			continue
		}
		raw, ok := v.(string)
		if !ok {
			continue
		}
		var fr domain.FundingRate
		if err := json.Unmarshal([]byte(raw), &fr); err != nil {
			return nil, fmt.Errorf("unmarshal funding rate %s/%s: %w", exchangeID, symbols[i], err)
		}
		rates = append(rates, fr)
	}
	return rates, nil
}

// --- Balances ---

// SetBalance stores a balance and adds its currency to the exchange's balances set.
func (s *Store) SetBalance(ctx context.Context, b domain.Balance) error {
	data, err := json.Marshal(b)
	if err != nil {
		return fmt.Errorf("marshal balance: %w", err)
	}

	pipe := s.client.TxPipeline()
	pipe.Set(ctx, rediskey.UserBalance(b.UserID, b.ExchangeID, b.Currency), data, 0)
	pipe.SAdd(ctx, rediskey.UserBalances(b.UserID, b.ExchangeID), b.Currency)
	pipe.Publish(ctx, rediskey.UserBalancePubSub(b.UserID, b.ExchangeID), data)

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("store balance %s/%s: %w", b.ExchangeID, b.Currency, err)
	}
	return nil
}

// ListBalances returns all balances for an exchange.
func (s *Store) ListBalances(ctx context.Context, userID, exchangeID string) ([]domain.Balance, error) {
	currencies, err := s.client.SMembers(ctx, rediskey.UserBalances(userID, exchangeID)).Result()
	if err != nil {
		return nil, fmt.Errorf("list balance currencies for %s: %w", exchangeID, err)
	}
	if len(currencies) == 0 {
		return nil, nil
	}

	keys := make([]string, len(currencies))
	for i, currency := range currencies {
		keys[i] = rediskey.UserBalance(userID, exchangeID, currency)
	}

	vals, err := s.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, fmt.Errorf("mget balances for %s: %w", exchangeID, err)
	}

	balances := make([]domain.Balance, 0, len(vals))
	for i, v := range vals {
		if v == nil {
			continue
		}
		raw, ok := v.(string)
		if !ok {
			continue
		}
		var b domain.Balance
		if err := json.Unmarshal([]byte(raw), &b); err != nil {
			return nil, fmt.Errorf("unmarshal balance %s/%s: %w", exchangeID, currencies[i], err)
		}
		balances = append(balances, b)
	}
	return balances, nil
}

// --- Arbitrage (unified) ---

// sortedExchangeIDs returns the two exchange IDs in alphabetical order.
// This ensures a canonical key regardless of which exchange is long vs short.
func sortedExchangeIDs(a, b string) (string, string) {
	if a <= b {
		return a, b
	}
	return b, a
}

// ttlForArbitrageType returns the TTL for an arbitrage entry based on strategy.
// Spread opportunities expire faster because price spreads are more volatile.
func ttlForArbitrageType(t domain.ArbitrageType) time.Duration {
	switch t {
	case domain.ArbitrageTypeSpread:
		return 30 * time.Second
	default:
		return 120 * time.Second
	}
}

// SetArbitrage stores an Arbitrage opportunity and tracks it in the unified pairs set.
// Exchange IDs in the key are always sorted alphabetically so that a direction change
// overwrites the same key rather than creating a duplicate.
func (s *Store) SetArbitrage(ctx context.Context, a domain.Arbitrage) error {
	data, err := json.Marshal(a)
	if err != nil {
		return fmt.Errorf("marshal arbitrage: %w", err)
	}

	exA, exB := sortedExchangeIDs(a.ExchangeA, a.ExchangeB)
	key := rediskey.ArbitragePair(string(a.Type), a.Symbol, exA, exB)
	member := a.Symbol + "|" + string(a.Type) + "|" + exA + "|" + exB

	// Clean up any stale contra-pair member that used the opposite ordering.
	if a.ExchangeA != exA {
		staleMember := a.Symbol + "|" + string(a.Type) + "|" + a.ExchangeA + "|" + a.ExchangeB
		staleKey := rediskey.ArbitragePair(string(a.Type), a.Symbol, a.ExchangeA, a.ExchangeB)
		s.client.SRem(ctx, rediskey.ArbitragePairs(), staleMember)
		s.client.Del(ctx, staleKey)
	}

	ttl := ttlForArbitrageType(a.Type)
	pipe := s.client.TxPipeline()
	pipe.Set(ctx, key, data, ttl)
	pipe.SAdd(ctx, rediskey.ArbitragePairs(), member)
	pipe.Publish(ctx, rediskey.ArbitragePubSub(), data)

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("store arbitrage %s %s %s|%s: %w", a.Type, a.Symbol, exA, exB, err)
	}
	return nil
}

// GetArbitrage retrieves a single arbitrage opportunity by type, symbol and exchange pair.
// Exchange IDs are sorted alphabetically to match the canonical key.
// Returns nil if not found.
func (s *Store) GetArbitrage(ctx context.Context, arbType domain.ArbitrageType, symbol, exchangeA, exchangeB string) (*domain.Arbitrage, error) {
	exA, exB := sortedExchangeIDs(exchangeA, exchangeB)
	key := rediskey.ArbitragePair(string(arbType), symbol, exA, exB)

	data, err := s.client.Get(ctx, key).Bytes()
	if err == goredis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get arbitrage %s|%s|%s|%s: %w", arbType, symbol, exA, exB, err)
	}

	var a domain.Arbitrage
	if err := json.Unmarshal(data, &a); err != nil {
		return nil, fmt.Errorf("unmarshal arbitrage %s|%s|%s|%s: %w", arbType, symbol, exA, exB, err)
	}
	return &a, nil
}

// ListArbitrage returns all stored arbitrage opportunities across all strategy types.
func (s *Store) ListArbitrage(ctx context.Context) ([]domain.Arbitrage, error) {
	members, err := s.client.SMembers(ctx, rediskey.ArbitragePairs()).Result()
	if err != nil {
		return nil, fmt.Errorf("list arbitrage pairs: %w", err)
	}
	if len(members) == 0 {
		return nil, nil
	}

	keys := make([]string, len(members))
	for i, m := range members {
		// m is "symbol|type|exchangeA|exchangeB"
		parts := strings.SplitN(m, "|", 4)
		if len(parts) != 4 {
			continue
		}
		keys[i] = rediskey.ArbitragePair(parts[1], parts[0], parts[2], parts[3])
	}

	vals, err := s.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, fmt.Errorf("mget arbitrage: %w", err)
	}

	arbs := make([]domain.Arbitrage, 0, len(vals))
	for i, v := range vals {
		if v == nil {
			// Expired, clean up stale member.
			s.client.SRem(ctx, rediskey.ArbitragePairs(), members[i])
			continue
		}
		raw, ok := v.(string)
		if !ok {
			continue
		}
		var a domain.Arbitrage
		if err := json.Unmarshal([]byte(raw), &a); err != nil {
			return nil, fmt.Errorf("unmarshal arbitrage %s: %w", members[i], err)
		}
		arbs = append(arbs, a)
	}
	return arbs, nil
}

// RemoveArbitrage removes a specific arbitrage pair by type, symbol and exchanges.
// Exchange IDs are sorted alphabetically to match the canonical key format.
func (s *Store) RemoveArbitrage(ctx context.Context, arbType domain.ArbitrageType, symbol, exchangeA, exchangeB string) error {
	exA, exB := sortedExchangeIDs(exchangeA, exchangeB)
	key := rediskey.ArbitragePair(string(arbType), symbol, exA, exB)
	member := symbol + "|" + string(arbType) + "|" + exA + "|" + exB

	pipe := s.client.TxPipeline()
	pipe.Del(ctx, key)
	pipe.SRem(ctx, rediskey.ArbitragePairs(), member)

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("remove arbitrage %s|%s|%s|%s: %w", arbType, symbol, exA, exB, err)
	}
	return nil
}

// --- Admin ---

// FlushAll deletes all gembo-prefixed keys from Redis using SCAN to avoid
// blocking the server. It returns the total number of keys deleted.
func (s *Store) FlushAll(ctx context.Context) (int64, error) {
	var deleted int64
	pattern := rediskey.Prefix + ":*"
	iter := s.client.Scan(ctx, 0, pattern, 100).Iterator()

	for iter.Next(ctx) {
		if err := s.client.Del(ctx, iter.Val()).Err(); err != nil {
			return deleted, fmt.Errorf("delete key %s: %w", iter.Val(), err)
		}
		deleted++
	}
	if err := iter.Err(); err != nil {
		return deleted, fmt.Errorf("scan keys %s: %w", pattern, err)
	}
	return deleted, nil
}
