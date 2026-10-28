# AGENTS.md

## Project overview

Gembo is a cryptocurrency exchange monitoring platform. Monorepo with two components:

- **Backend** (`backend/`): Go 1.25, pure `net/http`. Ingests real-time market data from crypto exchanges via CCXT, normalizes into domain models, stores market data in Redis and identity/credential/position data in PostgreSQL, serves via REST/WebSocket. Binaries: `market-data-collector` (public market data), `account-data-collector` (authenticated account data), `api` (HTTP server), `arbitrage-worker` (background jobs), `position-manager` (position management).
- **Frontend** (`frontend/`): TypeScript, Next.js 16 (App Router), React 19, Tailwind v4, shadcn/ui v4. Dashboards for tickers and exchange data.

Architecture: collectors, API, arbitrage worker, and position manager are separate processes communicating through Redis and PostgreSQL. The two collectors can run independently — `market-data-collector` requires no API keys and fetches markets, tickers, and funding rates; `account-data-collector` requires PostgreSQL with stored per-user exchange credentials and watches private account data. The position manager can execute real exchange orders; starting it is not part of public monitoring setup.

## Build & test commands

### Backend (run from `backend/`)

| Command | Purpose |
|---|---|
| `make build` | Build all binaries (`market-data-collector`, `account-data-collector`, `api`, `arbitrage-worker`, `position-manager`) |
| `make market-data-collector` | Build public collector only |
| `make account-data-collector` | Build private collector only |
| `make api` | Build API only |
| `make arbitrage-worker` | Build worker only |
| `make position-manager` | Build position manager only |
| `make test` | Run all Go tests (`go test ./...`) |
| `make lint` | Run `golangci-lint run ./...` (default config, no `.golangci.yml`) |
| `make vet` | Run `go vet ./...` |
| `make tidy` | Run `go mod tidy` |
| `make run-market-data-collector` | Build + run public collector |
| `make run-account-data-collector` | Build + run private collector |
| `make run-api` | Build + run API server |
| `make run-arbitrage-worker` | Build + run worker |
| `make run-position-manager` | Build + run position manager |
| `make docker-up` / `docker-down` | Development stack via Docker Compose; includes account collector and execution process |

**Running a single test:**
```bash
go test -run TestSortedExchangeIDs ./internal/store/redis
go test -v -run TestSortedExchangeIDs ./internal/store/redis  # verbose
go test -run TestPositionKeys/PositionPending ./pkg/rediskey   # subtests
```

### Frontend (run from `frontend/`)

| Command | Purpose |
|---|---|
| `npm run dev` | Dev server at localhost:3000 |
| `npm run build` | Production build |
| `npm run lint` | ESLint (flat config, core-web-vitals + typescript presets) |

No test framework is configured in the frontend. See [repository guide](docs/development.md) for configuration, dependency limitations, utility commands, and the complete command audit. Docker packaging remains broken pending D04; a Make target existing is not proof that it builds.

## Code style — Go (backend)

### Project layout

- `cmd/` — entrypoints (`api/`, `market-data-collector/`, `account-data-collector/`, `arbitrage-worker/`, `position-manager/`) plus `exchange-probe/`, `create-user/`, and `ccxt-raw-positions/`, each with a single `main.go`
- `internal/` — private packages: `service/` (five service implementations), `config/`, `domain/`, `exchanges/` (adapters and `base/` contracts), `pkg/` (arbitrage, hooks, logging, security), and `store/` (Redis/PostgreSQL)
- `pkg/` — shared utilities (e.g., `rediskey`)
- Module path: `github.com/zeroclap/gembo`

### Imports (strict ordering)

Group imports separated by blank lines: (1) stdlib, (2) third-party, (3) internal (`github.com/zeroclap/gembo/...`).

```go
import (
    "context"
    "fmt"
    "log/slog"

    "github.com/ccxt/ccxt/go/v4"

    "github.com/zeroclap/gembo/internal/domain"
)
```

### Formatting & naming

- Run `gofmt` / `goimports`. No custom formatter config.
- **Filenames**: `snake_case.go` (e.g., `funding_rate.go`, `market.go`)
- **Exported identifiers**: `PascalCase`; **Local variables**: `camelCase`
- **JSON tags**: `snake_case` (e.g., `json:"exchange_id"`, `json:"volume_24h"`)
- **Constants**: grouped at the top of the file, unexported `camelCase` for package-private

### Error handling

- Always wrap with context: `fmt.Errorf("bybit fetch markets: %w", err)`
- Never discard errors silently
- Use the pattern `prefix verb object: %w` for error messages

### Types & financial precision

- All price/volume/fee fields use `shopspring/decimal` (`decimal.Decimal`), serialized as strings in JSON
- Domain models (`internal/domain/`) depend only on stdlib + `shopspring/decimal`
- JSON envelope for REST responses: `{ data, error, ts }`

### Interfaces & constructors

- Define interfaces at the point of use — accept interfaces, return concrete structs
- Public exchange interface (`internal/exchanges/base/exchange.go`): `ID()`, `Run()`, `Markets()`, `Tickers()`, `FundingRates()`
- Constructor named `New` (not `NewClient`). Use functional options: `type Option func(*Client)`, `With*` helpers

### Concurrency patterns

- `errgroup` for goroutine lifecycle (`golang.org/x/sync/errgroup`)
- Buffered channels (size 256) for streaming pipelines (`chan domain.Ticker`)
- Context-based cancellation throughout; exponential backoff with `math.Phi` multiplier
- Structured logging via `log/slog` with fields: `"exchange"`, `"symbol"`, `"error"`, `"backoff"`

### HTTP & infrastructure

- Pure `net/http` with `http.ServeMux` (Go 1.22+ method-based routing). No framework.
- Build flags: `-trimpath -ldflags="-s -w"`
- Redis keys namespaced via `pkg/rediskey`: `gembo:<exchange>:<type>:<symbol>`

### Adding a new exchange

Create an adapter under `internal/exchanges/<name>/` with `client.go` (implements `base.Exchange`), `mapper.go` (CCXT to domain), and `options.go` (functional options). Wire it in `internal/exchanges/factory.go`; private capabilities use `base.PrivateExchange` from `base/private.go` and require separate qualification.

## Code style — TypeScript/React (frontend)

### CRITICAL: Next.js 16 breaking changes

This project uses Next.js 16 which has breaking changes from prior versions. **Always read the relevant guide in `node_modules/next/dist/docs/` before writing any code.** Do not rely on training data for Next.js APIs.

### Naming & files

- **Files**: kebab-case (`app-header.tsx`), **Components**: PascalCase (`AppHeader`)
- **`"use client"`** directive required for client-side interactivity
- `strict: true` in tsconfig — no `any` types, no implicit nulls
- Path alias: `@/*` maps to project root (e.g., `import { cn } from "@/lib/utils"`)
- `moduleResolution: "bundler"`, `jsx: "react-jsx"` (no `import React` needed)

### Styling & UI

- Tailwind v4 (CSS-native config, no `tailwind.config.js`), PostCSS via `@tailwindcss/postcss`
- `cn()` from `@/lib/utils` for conditional class merging (clsx + tailwind-merge)
- shadcn/ui v4 with radix-ui primitives and lucide-react icons

### State & data

- **Server state**: TanStack React Query (`@tanstack/react-query`) with typed API functions
- **Client state**: zustand; **Forms**: react-hook-form; **Tables**: TanStack React Table
- API proxy: Next.js rewrites `/api/*` to Go backend at `localhost:8080`

### ESLint

Flat config (`eslint.config.mjs`): `eslint-config-next/core-web-vitals` + `eslint-config-next/typescript`. No custom rules.

## Testing

- Run `make test` from `backend/` and `npm run lint` from `frontend/` before committing
- Run `make lint` and `make vet` for Go static analysis
- **Add tests for any code you change**, even if not explicitly asked
- All lints and tests must pass before a PR is mergeable

## Security

- Never commit secrets, API keys, or credentials
- `backend/configs/config.yaml` is tracked: keep it free of secrets. `.env` is ignored. Runtime private services use per-user exchange credentials stored in PostgreSQL; legacy config/env credential fields remain for utilities. See the configuration inventory in `docs/development.md`.
- Use `joho/godotenv` for `.env` loading in development

## PR & commit conventions

- Commit messages: concise, describe the "why" not the "what"
- Keep PRs focused on a single concern
- Run all lints and tests before committing

## Delivery direction

Follow [docs/delivery/README.md](docs/delivery/README.md) and its dependency-ordered backlog. The unified `gembo` CLI is planned, not implemented. New supported capabilities require CLI contracts and acceptance evidence under that plan; the UI may cover a subset. D01 documents current interfaces; D02 establishes the executable baseline.
