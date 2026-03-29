# Gembo

High-performance crypto exchange monitoring platform. Ingests real-time market data from multiple exchanges, normalizes it into unified models, and serves it via REST/WebSocket APIs.

## Monorepo Structure

```
gembo/
├── backend/          Go services (API server + data collector)
│   ├── cmd/          Entrypoints (api, collector)
│   ├── internal/     Application code (domain, exchange adapters, store, hooks)
│   ├── pkg/          Shared library code
│   ├── configs/      Configuration examples
│   ├── deployments/  Dockerfile & docker-compose
│   ├── docs/         Documentation & PRD
│   ├── Makefile
│   ├── go.mod
│   └── go.sum
└── frontend/         Frontend application (TBD)
```

## Getting Started

### Prerequisites

- Go 1.25+
- Redis 7+
- Docker & Docker Compose (optional)

### Run Locally

```bash
cd backend

# Copy and configure environment
cp .env.example .env  # edit with your API keys

# Build both binaries
make build

# Run the collector (ingests exchange data into Redis)
make run-collector

# Run the API server (serves data over HTTP/WebSocket)
make run-api
```

### Docker

```bash
cd backend

# Build and start all services (Redis + collector + API)
make docker-up

# Stop
make docker-down
```

### Available Make Targets

Run all targets from the `backend/` directory:

| Target           | Description                          |
|------------------|--------------------------------------|
| `make build`     | Build collector and API binaries     |
| `make collector` | Build collector binary only          |
| `make api`       | Build API binary only                |
| `make run-collector` | Build and run collector          |
| `make run-api`   | Build and run API server             |
| `make tidy`      | Run `go mod tidy`                    |
| `make vet`       | Run `go vet`                         |
| `make test`      | Run all tests                        |
| `make lint`      | Run golangci-lint                    |
| `make clean`     | Remove build artifacts               |
| `make docker-build` | Build Docker images               |
| `make docker-up` | Start Docker Compose stack           |
| `make docker-down` | Stop Docker Compose stack          |

## Architecture

**Two-binary design:**

- **collector** -- Connects to exchanges (Bybit, OKX) via CCXT, ingests markets/tickers/funding rates, writes to Redis, and emits BullMQ events.
- **api** -- HTTP server exposing REST endpoints and WebSocket streams for real-time ticker data.

**Supported exchanges:** Bybit, OKX (extensible via plugin architecture under `internal/exchange/exchanges/`).
