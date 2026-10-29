# Gembo

Gembo collects cryptocurrency market and account data, computes arbitrage opportunities, and exposes authenticated HTTP and WebSocket APIs with a Next.js frontend. Position execution is a separate backend process.

## Repository

| Path | Contents |
|---|---|
| `backend/cmd/` | Five service entrypoints and three specialist utilities |
| `backend/internal/service/` | API, market/account collectors, arbitrage worker, position manager |
| `backend/internal/exchanges/` | Exchange adapters, base contracts, and factory wiring |
| `backend/internal/domain/` | Domain models |
| `backend/internal/store/` | Redis and PostgreSQL stores; embedded SQL migrations |
| `backend/internal/pkg/`, `backend/pkg/` | Internal helpers and shared Redis key conventions |
| `backend/configs/` | Tracked YAML defaults; never put secrets here |
| `backend/deployments/` | Development Compose stack and legacy Dockerfile |
| `frontend/` | Next.js 16.2.1 / React 19.2.4 dashboard |
| `docs/delivery/` | CLI-first delivery plan and ordered backlog |

## Local development

The backend declares Go 1.25.0. The [D02 verification toolchain](docs/verification.md) pins Go 1.25.14, Node 22.23.4, npm 10.9.9 and golangci-lint 2.15.0 in digest-pinned Linux images. From the repository root, `make bootstrap`, `make test-verification` and `make baseline` reproduce credential-free checks with Git, Docker and Python 3.10+. Baseline results retain failures and skips. Development Compose uses Redis 7 and PostgreSQL 17 and remains optional for local infrastructure. See the [repository guide](docs/development.md) for application commands and configuration.

From `backend/`, copy `.env.example` to `.env` only if `.env` does not already exist, then configure local infrastructure and authentication. Use synthetic users and no exchange credentials for initial development. The API needs PostgreSQL for login and account/position features; without a database, those features are unavailable.

```bash
cd backend
make build
make test
make lint
make vet
```

Each service runs in its own terminal from `backend/` against configured dependencies:

| Command | Purpose |
|---|---|
| `make run-market-data-collector` | Public markets, tickers, and funding data; makes real public exchange calls |
| `make run-api` | HTTP API on port 8080 by default |
| `make run-arbitrage-worker` | Recomputes opportunities from Redis updates |
| `make run-account-data-collector` | Private account collection using credentials stored in PostgreSQL |
| `make run-position-manager` | Position execution and reconciliation using stored credentials; can place real orders |

For public monitoring, start only the public collector, API, and arbitrage worker. Select a single exchange with `EXCHANGES` for initial qualification. Running these commands is not evidence of integration qualification.

From `frontend/`:

```bash
npm ci
npm run dev
```

The development UI serves port 3000 and rewrites `/api/*` to the API at `localhost:8080`, removing `/api`. API routes start at `/v1/`. Use `npm run lint` and `npm run build` for frontend verification. There is no frontend test script yet.

## Containers and current limitations

From `backend/`, `make docker-up` starts the **entire development stack**, including all six public collectors, the account collector, and the position manager. It requires `backend/.env`, exposes development infrastructure ports, and is unsuitable for production. `make docker-down` stops the stack and retains named volumes.

`make docker-build` exists but currently references targets absent from the legacy Dockerfile; the Dockerfile also references removed entrypoint paths. Packaging repair is tracked as D04. The frontend has no Dockerfile. See the [D01 audit](docs/reports/d01-repository-instructions.md) for validation scope and remaining gaps.

## Architecture and delivery

Redis holds market snapshots, derived opportunities, and Pub/Sub updates. PostgreSQL holds users, encrypted credentials when a cipher is configured, positions, orders, funding records, and workspaces. API, account collector, position manager, and user-creation utility currently run migrations on startup.

Public factory wiring exists for Binance, Bybit, OKX, Gate, HTX, and Aster; private factory wiring exists for Binance, Bybit, and OKX. KuCoin has an adapter directory but is not wired in the factory. Wiring does not establish verified exchange support.

The [delivery plan](docs/delivery/README.md) makes a unified `gembo` CLI the primary application interface. That CLI is not implemented yet; current command-line tools are specialist utilities. Start with the [roadmap](docs/delivery/05-roadmap-and-backlog.md) and [repository guide](docs/development.md).
