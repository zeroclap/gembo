# Product Requirements Document: Gembo — Crypto Exchange Monitoring Platform

**Version:** 0.1.0  
**Date:** 2026-03-20  
**Status:** Draft

---

## 1. Overview

Gembo is a high-performance, extensible crypto currency monitoring platform built in Go. It serves as the foundational data layer for a monorepo that will host multiple applications — visualisation frontends, trading bots, alerting systems, and analytics tools — all consuming a unified, real-time stream of exchange data.

### 1.1 Goals

- Ingest market data (markets, tickers, funding rates) from multiple crypto exchanges simultaneously.
- Normalize all data into unified domain models regardless of exchange-specific formats.
- Store data in Redis for low-latency access by downstream consumers.
- Expose data via a REST/HTTP API with WebSocket support for real-time ticker streaming.
- Emit event hooks (e.g. BullMQ-compatible queue messages) on data updates for reactive consumers.
- Make adding a new exchange a minimal, well-defined task via CCXT.

### 1.2 Non-Goals (v0.1)

- Order execution / trading.
- Historical data storage (persistent database, time-series).
- User authentication / authorization.
- Billing or multi-tenancy.

---

## 2. Architecture Overview

```
┌─────────────────────────────────────────────────────────┐
│                    cmd/                                  │
│   ┌──────────────┐   ┌──────────────┐                   │
│   │  collector/  │   │    api/      │  ← separate bins  │
│   │  main.go     │   │   main.go    │                   │
│   └──────┬───────┘   └──────┬───────┘                   │
└──────────┼─────────────────┼────────────────────────────┘
           │                 │
           ▼                 ▼
┌─────────────────────────────────────────────────────────┐
│                  internal/                               │
│                                                          │
│  exchange/          domain/         store/               │
│  ┌────────────┐    ┌───────────┐   ┌──────────────────┐ │
│  │ Exchange   │    │ Market    │   │ Redis store       │ │
│  │ interface  │───▶│ Ticker    │──▶│ get/set/pub       │ │
│  │ (CCXT)     │    │ Funding   │   └──────────────────┘ │
│  └─────┬──────┘    │   Rate    │                         │
│        │           └───────────┘   hook/                 │
│  exchanges/                        ┌──────────────────┐  │
│  ┌──────────┐                      │ EventEmitter      │ │
│  │ bybit/   │                      │ BullMQ publisher  │ │
│  │ okx/     │                      └──────────────────┘ │
│  └──────────┘                                            │
│                                    api/                  │
│                                    ┌──────────────────┐  │
│                                    │ HTTP handlers     │ │
│                                    │ /markets          │ │
│                                    │ /funding-rates    │ │
│                                    │ WS /tickers       │ │
│                                    └──────────────────┘ │
└─────────────────────────────────────────────────────────┘
```

### 2.1 Monorepo Structure

```
gembo/
├── cmd/
│   ├── collector/          # Data ingestion binary
│   │   └── main.go
│   └── api/                # HTTP API binary
│       └── main.go
├── internal/
│   ├── domain/             # Unified domain models (pure structs, no deps)
│   │   ├── market.go
│   │   ├── ticker.go
│   │   └── funding_rate.go
│   ├── exchange/           # Exchange abstraction
│   │   ├── exchange.go     # Exchange interface
│   │   ├── registry.go     # Exchange registry
│   │   └── exchanges/
│   │       ├── bybit/
│   │       │   ├── client.go   # CCXT bybit instance setup
│   │       │   └── mapper.go   # ccxt types → domain types
│   │       └── okx/
│   │           ├── client.go   # CCXT okx instance setup
│   │           └── mapper.go   # ccxt types → domain types
│   ├── store/              # Redis read/write/pubsub
│   │   ├── store.go        # Store interface
│   │   └── redis/
│   │       └── redis.go
│   ├── hook/               # Event emission
│   │   ├── emitter.go      # Emitter interface
│   │   └── bullmq/
│   │       └── bullmq.go   # Redis-list BullMQ v5 publisher
│   ├── collector/          # Orchestrates exchange → store pipeline
│   │   └── collector.go
│   └── api/                # HTTP handlers
│       ├── handler.go
│       ├── markets.go
│       ├── tickers_ws.go   # WebSocket ticker hub
│       └── funding_rates.go
├── pkg/                    # Code safe to import by any app in the monorepo
│   └── rediskey/           # Shared key-naming conventions
│       └── keys.go
├── apps/                   # Future consumer applications
│   ├── frontend/           # Visualization app (non-Go)
│   └── tradingbot/         # Trading bot (Go)
├── docs/
│   └── high-level.prd.md
├── configs/
│   └── config.example.yaml
├── deployments/
│   ├── docker-compose.yml
│   └── Dockerfile
├── scripts/
│   └── dev.sh
├── go.work                 # Go workspace (multi-module)
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

---

## 3. Domain Models

All domain models live in `internal/domain/`. They are plain Go structs with no external dependencies.

### 3.1 Market

```go
// Market represents a tradable instrument on an exchange.
type Market struct {
    ExchangeID string     `json:"exchange_id"` // e.g. "bybit"
    Symbol     string     `json:"symbol"`      // normalized, e.g. "TON/USDT"
    BaseAsset  string     `json:"base_asset"`  // e.g. "TON"
    QuoteAsset string     `json:"quote_asset"` // e.g. "USDT"
    MarketType MarketType `json:"market_type"` // spot | perpetual | futures
    Active     bool       `json:"active"`
    UpdatedAt  time.Time  `json:"updated_at"`
}

type MarketType string

const (
    MarketTypeSpot      MarketType = "spot"
    MarketTypePerpetual MarketType = "perpetual"
    MarketTypeFutures   MarketType = "futures"
)
```

**Note on symbols:** CCXT uses the unified `/`-separated format (e.g. `BTC/USDT`, `BTC/USDT:USDT` for perpetuals). Domain models preserve this format.

### 3.2 Ticker

```go
// Ticker is a real-time price snapshot for a market.
type Ticker struct {
    ExchangeID string          `json:"exchange_id"`
    Symbol     string          `json:"symbol"`
    Price      decimal.Decimal `json:"price"`      // last traded price
    Bid        decimal.Decimal `json:"bid"`
    Ask        decimal.Decimal `json:"ask"`
    High24h    decimal.Decimal `json:"high_24h"`
    Low24h     decimal.Decimal `json:"low_24h"`
    Volume24h  decimal.Decimal `json:"volume_24h"`
    Change24h  decimal.Decimal `json:"change_24h"` // percentage
    UpdatedAt  time.Time       `json:"updated_at"`
}
```

### 3.3 FundingRate

```go
// FundingRate is the periodic funding payment for a perpetual contract.
type FundingRate struct {
    ExchangeID      string          `json:"exchange_id"`
    Symbol          string          `json:"symbol"`
    Rate            decimal.Decimal `json:"rate"`             // e.g. -0.04 means -4%
    NextFundingTime time.Time       `json:"next_funding_time"`
    IntervalHours   int             `json:"interval_hours"`   // typically 4 or 8
    UpdatedAt       time.Time       `json:"updated_at"`
}
```

**Note:** `decimal.Decimal` maps to `github.com/shopspring/decimal` for precision arithmetic — the only domain-level external dependency justified by correctness requirements. All `decimal.Decimal` fields are serialized as **strings** in JSON API responses to avoid floating-point precision loss.

---

## 4. Exchange Interface & CCXT Integration

Every exchange adapter implements a single interface defined in `internal/exchange/exchange.go`. All adapters use [CCXT for Go](https://github.com/ccxt/ccxt) (`github.com/ccxt/ccxt/go/v4`) as the underlying data collection library, eliminating the need for custom WebSocket or REST transport code per exchange.

```go
// Exchange is the contract every exchange adapter must satisfy.
type Exchange interface {
    // ID returns the unique identifier, e.g. "bybit".
    ID() string

    // Run establishes underlying connections via CCXT and drives the
    // adapter's internal event loops. It blocks until ctx is cancelled.
    Run(ctx context.Context) error

    // Markets returns the current list of available markets.
    // Implemented via CCXT REST: exchange.LoadMarkets() / FetchMarkets().
    Markets(ctx context.Context) ([]domain.Market, error)

    // Tickers subscribes to real-time ticker updates via CCXT WebSocket.
    // Implemented via CCXT Pro: exchange.WatchTickers(symbols).
    Tickers(ctx context.Context) (<-chan domain.Ticker, error)

    // FundingRates returns current funding rates via periodic REST polling.
    // Implemented via CCXT REST: exchange.FetchFundingRates(symbols).
    FundingRates(ctx context.Context) (<-chan domain.FundingRate, error)
}
```

### 4.1 CCXT Transport Strategy

| Data Type | CCXT Method | Transport | Update Mechanism |
|-----------|-------------|-----------|-----------------|
| Markets | `LoadMarkets()` / `FetchMarkets()` | REST | Polled every 60 s |
| Tickers | `WatchTickers(symbols)` | WebSocket (CCXT Pro) | Real-time push |
| Funding Rates | `FetchFundingRates(symbols)` | REST | Polled every 60 s |

CCXT Pro (`github.com/ccxt/ccxt/go/v4/pro`) is used for WebSocket ticker streaming. CCXT REST (`github.com/ccxt/ccxt/go/v4`) is used for markets and funding rates.

```go
// Example: CCXT Pro WebSocket ticker subscription (inside adapter)
import ccxtpro "github.com/ccxt/ccxt/go/v4/pro/bybit"

exchange := ccxtpro.New()
exchange.EnableRateLimit = true
defer exchange.Close()

for {
    select {
    case <-ctx.Done():
        return ctx.Err()
    default:
    }

    tickers, err := exchange.WatchTickers(symbols, nil)
    if err != nil {
        return fmt.Errorf("watch tickers: %w", err)
    }
    // map tickers → domain.Ticker and send to channel
}
```

```go
// Example: CCXT REST funding rate polling (inside adapter)
import ccxt "github.com/ccxt/ccxt/go/v4/bybit"

exchange := ccxt.New()
exchange.EnableRateLimit = true

rates, err := exchange.FetchFundingRates(symbols, nil)
if err != nil {
    return fmt.Errorf("fetch funding rates: %w", err)
}
// map rates → domain.FundingRate
```

### 4.2 Adding a New Exchange

To add a new exchange:

1. Create `internal/exchange/exchanges/<name>/` directory.
2. Create `client.go` that initializes the CCXT exchange instance (REST + Pro).
3. Create `mapper.go` that converts CCXT types → domain types.
4. Implement the `Exchange` interface using CCXT calls.
5. Register the exchange in `internal/exchange/registry.go`.

No changes required elsewhere — the collector and API discover exchanges via the registry.

### 4.3 CCXT Error Handling

CCXT provides typed errors that map cleanly to retry strategies:

| CCXT Error Type | Retry? | Handling |
|-----------------|--------|---------|
| `NetworkError` | Yes | Exponential backoff |
| `RateLimitExceeded` | Yes | Respect `exchange.RateLimit` delay |
| `ExchangeNotAvailable` | Yes | Exponential backoff |
| `ExchangeError` | No | Log and skip |

---

## 5. Store Layer

`internal/store/store.go` defines the storage interface.

```go
type Store interface {
    // Markets
    SetMarket(ctx context.Context, m domain.Market) error
    GetMarket(ctx context.Context, exchangeID, symbol string) (*domain.Market, error)
    ListMarkets(ctx context.Context, exchangeID string) ([]domain.Market, error)

    // Tickers
    SetTicker(ctx context.Context, t domain.Ticker) error
    GetTicker(ctx context.Context, exchangeID, symbol string) (*domain.Ticker, error)

    // Funding Rates
    SetFundingRate(ctx context.Context, fr domain.FundingRate) error
    GetFundingRate(ctx context.Context, exchangeID, symbol string) (*domain.FundingRate, error)
    ListFundingRates(ctx context.Context, exchangeID string) ([]domain.FundingRate, error)
}
```

### 5.1 Redis Implementation

`internal/store/redis/redis.go` implements `Store` using:

- **Hash** for structured fields per key.
- **Set** for index keys (e.g. all symbols for an exchange).
- **Pub/Sub** channel for change notification (used internally by the hook layer and WebSocket ticker hub).

**Key naming convention** (in `pkg/rediskey`):

```
gembo:<exchange>:market:<symbol>        → Hash (Market fields)
gembo:<exchange>:markets                → Set  (list of symbols)
gembo:<exchange>:ticker:<symbol>        → Hash (Ticker fields)
gembo:<exchange>:funding:<symbol>       → Hash (FundingRate fields)
gembo:<exchange>:funding               → Set  (list of symbols with funding)
```

**TTL:** Ticker keys expire after 60 s (refreshed on each update) to avoid stale data if an exchange disconnects.

---

## 6. Hook / Event Layer

`internal/hook/emitter.go` defines:

```go
// Event represents a data update notification.
type Event struct {
    Type     EventType       `json:"type"`     // market_updated | ticker_updated | funding_updated
    Exchange string          `json:"exchange"`
    Symbol   string          `json:"symbol"`
    Payload  json.RawMessage `json:"payload"`  // pre-encoded domain model
}

type EventType string

const (
    EventMarketUpdated  EventType = "market_updated"
    EventTickerUpdated  EventType = "ticker_updated"
    EventFundingUpdated EventType = "funding_rate_updated"
)

// Emitter publishes events to downstream consumers.
type Emitter interface {
    Emit(ctx context.Context, e Event) error
}
```

### 6.1 BullMQ Publisher

`internal/hook/bullmq/bullmq.go` implements `Emitter`.

BullMQ stores jobs as JSON in a Redis list following the **BullMQ v5** wire format:

```
LPUSH bull:<queueName>:wait <json-job>
```

This allows any BullMQ v5 consumer (Node.js, etc.) to pick up events without additional infrastructure.

Queue names map to event types:
- `bull:gembo.ticker:wait`
- `bull:gembo.market:wait`
- `bull:gembo.funding:wait`

---

## 7. Collector

`internal/collector/collector.go` orchestrates the pipeline:

```
Exchange.Tickers()      ──▶  Store.SetTicker()      ──▶  Emitter.Emit()
Exchange.FundingRates() ──▶  Store.SetFundingRate()  ──▶  Emitter.Emit()
Exchange.Markets()      ──▶  Store.SetMarket()       ──▶  Emitter.Emit()
```

- Runs one goroutine per exchange per data type.
- Uses `errgroup` for lifecycle management.
- Reconnects on CCXT `NetworkError` with exponential backoff (max 60 s).
- Emits events only when data actually changes (hash comparison).

---

## 8. HTTP API

`internal/api/` implements an HTTP/JSON API using only `net/http` from the standard library (no framework dependency).

### 8.1 Endpoints

```
GET  /v1/exchanges                          List registered exchanges
GET  /v1/{exchange}/markets                 List all markets (REST poll)
GET  /v1/{exchange}/markets/{symbol}        Get single market
GET  /v1/{exchange}/funding-rates           List all funding rates (REST poll)
GET  /v1/{exchange}/funding-rates/{symbol}  Get single funding rate
GET  /v1/health                             Health check

WS   /v1/{exchange}/tickers                 Real-time ticker stream (WebSocket)
WS   /v1/tickers                            Real-time ticker stream — all exchanges
```

**Ticker delivery model:**
- Clients connect via WebSocket upgrade to `/v1/{exchange}/tickers`.
- The server fans out Redis Pub/Sub ticker updates to all connected WebSocket clients.
- Markets and funding rates are available via REST polling only; no WebSocket push for those.

### 8.2 Response Format

REST responses follow a consistent envelope:

```json
{
  "data": { ... },
  "error": null,
  "ts": "2026-03-20T12:00:00Z"
}
```

Errors:

```json
{
  "data": null,
  "error": { "code": "not_found", "message": "symbol BTC/USDT not found" },
  "ts": "2026-03-20T12:00:00Z"
}
```

**JSON serialization:** All `decimal.Decimal` fields are serialized as JSON strings (e.g. `"price": "65432.10"`) to preserve precision.

WebSocket ticker messages are JSON objects matching the `domain.Ticker` structure with decimals as strings.

---

## 9. Configuration

All configuration is loaded from environment variables (12-factor) with an optional YAML config file overlay.

```yaml
# configs/config.example.yaml
server:
  host: "0.0.0.0"
  port: 8080
  read_timeout: "10s"
  write_timeout: "10s"

redis:
  addr: "localhost:6379"
  password: ""
  db: 0

exchanges:
  - id: bybit
    enabled: true
    markets_poll_interval: "60s"
    funding_poll_interval: "60s"
  - id: okx
    enabled: true
    markets_poll_interval: "60s"
    funding_poll_interval: "60s"

hooks:
  bullmq:
    enabled: true
    queue_prefix: "gembo"

log:
  level: "info"   # debug | info | warn | error
  format: "json"  # json | text
```

---

## 10. Dependencies

Strict minimum-dependency policy. Only add a dependency when the alternative is reimplementing a non-trivial, security-sensitive, or highly complex piece of functionality.

| Package | Justification |
|---------|---------------|
| `github.com/redis/go-redis/v9` | Redis client; no viable stdlib alternative |
| `github.com/shopspring/decimal` | Exact decimal arithmetic for financial data |
| `golang.org/x/sync` (errgroup) | Structured goroutine lifecycle; small, official |
| `github.com/ccxt/ccxt/go/v4` | CCXT REST client — unified exchange API, eliminates per-exchange REST code |
| `github.com/ccxt/ccxt/go/v4/pro` | CCXT Pro WebSocket client — unified WS streaming for tickers |

Everything else — HTTP server, JSON, logging, config parsing — uses the Go standard library.

---

## 11. Error Handling & Observability

- All errors wrapped with `fmt.Errorf("context: %w", err)`.
- CCXT typed errors (`NetworkError`, `ExchangeError`, etc.) mapped to retry vs. fatal handling in the collector.
- Structured JSON logging via `log/slog` (Go 1.21 stdlib).
- Log fields: `exchange`, `symbol`, `event_type`, `latency_ms`, `error`.
- Prometheus metrics endpoint at `/metrics` (optional, feature-flagged):
  - `gembo_updates_total{exchange, type}` — counter
  - `gembo_update_latency_seconds{exchange, type}` — histogram
  - `gembo_exchange_connected{exchange}` — gauge
  - `gembo_ws_clients{exchange}` — gauge (active WebSocket API clients)

---

## 12. Deployment

- **docker-compose** for local dev: Redis + collector + api.
- Each binary is a separate container with a multi-stage Dockerfile (final image: `gcr.io/distroless/static`).
- Environment-variable-only runtime config (no config file in containers).

---

## 13. Phased Delivery

| Phase | Scope |
|-------|-------|
| **P0 — Foundation** | Domain models, Exchange interface, Redis store, Registry, Config |
| **P1 — First Exchange (Bybit)** | Bybit adapter via CCXT (WS tickers + REST markets + REST funding rates) |
| **P2 — API** | REST API (markets, funding rates), WebSocket ticker endpoint, health check |
| **P3 — Second Exchange (OKX)** | OKX adapter via CCXT (validates extensibility) |
| **P4 — Events** | Hook/Emitter interface, BullMQ v5 publisher |
| **P5 — Observability** | slog structured logging, Prometheus metrics |
| **P6 — Monorepo Apps** | go.work workspace, apps/ scaffold for tradingbot / frontend |
