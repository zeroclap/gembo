# AGENTS.md

## Project overview

Gembo is a cryptocurrency exchange monitoring platform. Monorepo with two components:

- **Backend** (`backend/`): Go 1.25, pure `net/http`. Ingests real-time market data from crypto exchanges via CCXT, normalizes into domain models, stores in Redis, serves via REST/WebSocket. Binaries (`backend/cmd/`): `market-data-collector` (public market data), `account-data-collector` (authenticated account data), `api` (HTTP server), `arbitrage-worker` (background jobs), `position-manager` (position management), plus the operator CLI tools `exchange-probe`, `create-user`, and `ccxt-raw-positions`.
- **Frontend** (`frontend/`): TypeScript, Next.js 16 (App Router), React 19, Tailwind v4, shadcn/ui v4. Dashboards for tickers and exchange data.

Architecture: collectors, API, worker, and position manager are separate processes communicating through Redis (and Postgres for account data, the API, and the position manager). The two collectors can run independently — `market-data-collector` requires no API keys and fetches markets, tickers, and funding rates; `account-data-collector` requires exchange credentials (read from the `exchange_credentials` table, DB-only) and watches positions via WebSocket.

> **Status: this repo is pre-production and under active reconciliation; see [ROADMAP.md](ROADMAP.md).** Nothing here authorizes execution, production deployment, or private-account access beyond what the owner has approved issue by issue.

### Owner operating constraints (binding, from ROADMAP.md)

1. **Real money.** Production is isolated: a separate GCP project and separate credentials. Agents plan; the owner applies.
2. **API drift is expected.** Every change to exchange or CCXT code ships with an integration test for each API call and an e2e scenario run against the real public APIs.
3. There is no sandbox or staging environment.
4. **CLI-first.** UI work is frozen until the owner explicitly unfreezes it.
5. Code documentation lives next to the code. Reports and other non-product artifacts go in GitHub issues.
6. **Latency matters, within reason.** Measure first. Do not accept large infrastructure or complexity for marginal gains.
7. Tooling: Go, GitHub Issues/Projects/Actions, Copilot CLI agents, GCP + Terraform, and free-tier Postgres (Supabase or similar) during beta.

## Build & test commands

### Backend (run from `backend/`)

| Command | Purpose |
|---|---|
| `make build` | Build the 5 service binaries (`market-data-collector`, `account-data-collector`, `api`, `arbitrage-worker`, `position-manager`) |
| `make market-data-collector` | Build the public market data collector only |
| `make account-data-collector` | Build the account data collector only |
| `make api` | Build API only |
| `make arbitrage-worker` | Build arbitrage worker only |
| `make position-manager` | Build position manager only |
| `make test` | Run all Go tests (`go test ./...`) |
| `make lint` | Run `golangci-lint run ./...` (default config, no `.golangci.yml`) |
| `make vet` | Run `go vet ./...` |
| `make tidy` | Run `go mod tidy` |
| `make run-market-data-collector` | Build + run the market data collector |
| `make run-account-data-collector` | Build + run the account data collector |
| `make run-api` | Build + run API server |
| `make run-arbitrage-worker` | Build + run arbitrage worker |
| `make run-position-manager` | Build + run position manager |
| `make docker-build` | Build a Docker image per service binary from `deployments/Dockerfile` |
| `make docker-up` / `docker-down` | Full stack via docker-compose |

The CLI tools `exchange-probe`, `create-user`, and `ccxt-raw-positions` are operator utilities run directly with `go run ./cmd/<name>`; they have no `make` targets and no Docker images (they are not long-running services).

**Running a single test:**
```bash
go test -run TestFunctionName ./internal/path/to/package/...
go test -v -run TestFunctionName ./internal/path/to/package/...  # verbose
go test -run TestSuite/TestCase ./internal/path/to/package/...   # subtests
```

### Frontend (run from `frontend/`)

| Command | Purpose |
|---|---|
| `npm run dev` | Dev server at localhost:3000 |
| `npm run build` | Production build |
| `npm run lint` | ESLint (flat config, core-web-vitals + typescript presets) |

No test framework is configured in the frontend.

## Code style — Go (backend)

### Project layout

- `cmd/` — entrypoints, each with a single `main.go`: `market-data-collector/`, `account-data-collector/`, `api/`, `arbitrage-worker/`, `position-manager/`, plus the CLI tools `exchange-probe/`, `create-user/`, `ccxt-raw-positions/`
- `internal/` — private packages: `config/`, `domain/`, `exchanges/` (adapters, e.g. `exchanges/bybit/`, `exchanges/base/`), `service/` (per-binary wiring, e.g. `service/api/`), `store/` (Redis + Postgres), `pkg/` (`pkg/hook/`, `pkg/arbitrage/`)
- `pkg/` (top-level) — shared utilities (e.g., `rediskey`)
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
- Exchange interface (`internal/exchanges/base/exchange.go`): `ID()`, `Run()`, `Markets()`, `Tickers()`, `FundingRates()`
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

Create adapter under `internal/exchanges/<name>/` with `client.go` (implements `base.Exchange`), `mapper.go` (CCXT to domain), `options.go` (functional options). Register it in the `switch` statements in `internal/exchanges/factory.go` (`BuildPublicRegistry` and, for private/credentialed access, `NewPrivateExchangeFromCredential`).

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
- `configs/config.yaml` is committed and holds non-secret defaults only. Exchange API credentials are **DB-only**, stored in the `exchange_credentials` table (encrypted at rest via `security.credentials_encryption_key_base64` / `CREDENTIALS_ENCRYPTION_KEY_BASE64`) — never in `.env` or `config.yaml`. See `backend/.env.example`.
- Use `joho/godotenv` for `.env` loading in development

## PR & commit conventions

- Commit messages: concise, describe the "why" not the "what"
- Keep PRs focused on a single concern
- Run all lints and tests before committing
