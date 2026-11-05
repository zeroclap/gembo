# Gembo

Operator tool for cross-exchange crypto perpetual-futures arbitrage (funding-rate and price-spread). Ingests market data from exchanges via CCXT, normalizes it, stores it in Redis, and serves it over REST/WebSocket.

> [!WARNING]
> **Not production-ready.** Baseline (2026-11-04, [#1](https://github.com/zeroclap/gembo/issues/1)): arbitrage output is not yet trustworthy, there is no CI, and `position-manager` can place **real market orders** without dry-run, kill switch, or notional caps. Do not run it with real exchange credentials.

**➡️ Direction and current priorities: [ROADMAP.md](ROADMAP.md)** (status: proposed, awaiting owner approval in #1). Work is tracked in [GitHub issues](https://github.com/zeroclap/gembo/issues) and [milestones](https://github.com/zeroclap/gembo/milestones).

## Monorepo structure

```
gembo/
├── backend/            Go services
│   ├── cmd/            Entrypoints (one main.go each, see below)
│   ├── internal/       Domain, exchange adapters, services, store, API
│   ├── configs/        config.yaml (no secrets)
│   ├── deployments/    Dockerfile & docker-compose
│   ├── docs/           PRD (v0.1, superseded as direction by ROADMAP.md)
│   └── Makefile
└── frontend/           Next.js dashboard (frozen: CLI-first, see ROADMAP.md)
```

## Binaries (`backend/cmd/`)

| Binary | Purpose | Credentials |
|---|---|---|
| `market-data-collector` | Public markets, tickers, funding rates → Redis | none |
| `arbitrage-worker` | Computes spread and funding opportunities from Redis | none |
| `api` | REST + WebSocket API (JWT auth; Postgres optional for read-only data) | API secret |
| `exchange-probe` | Live check of public exchange endpoints | none |
| `account-data-collector` | Positions, balances, funding payments | exchange keys (from DB) |
| `position-manager` | **Places real orders** | exchange keys (from DB) |
| `create-user` | Creates an API user in Postgres | DB |
| `ccxt-raw-positions` | Prints raw authenticated positions | exchange keys |

Exchanges configured in `backend/configs/config.yaml`: Bybit, OKX, Binance, Gate, HTX, Aster. A KuCoin adapter exists but is not registered.

## Getting started

Prerequisites: Go 1.25+, Redis 7+, Docker (optional).

```bash
cd backend
cp .env.example .env
export AUTH_JWT_SECRET=$(openssl rand -hex 32)   # never rely on the built-in default
make build                         # market-data-collector, account-data-collector, api, arbitrage-worker, position-manager
make run-market-data-collector     # public data only, no credentials
make run-arbitrage-worker
make run-api
go run ./cmd/exchange-probe -exchanges bybit -symbols BTC/USDT:USDT -ws
```

Bybit and Binance block US IP addresses.

### Make targets (`backend/`)

| Target | Description |
|---|---|
| `make build` | Build the five service binaries into `bin/` |
| `make <binary>` / `make run-<binary>` | Build / build and run one of `market-data-collector`, `account-data-collector`, `api`, `arbitrage-worker`, `position-manager` |
| `make test` / `vet` / `lint` / `tidy` | Tests, `go vet`, golangci-lint, `go mod tidy` |
| `make clean` | Remove build artifacts |
| `make docker-build` / `docker-up` / `docker-down` | Docker images and compose stack |

