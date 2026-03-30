# AGENTS.md

## Project overview

Gembo is a high-performance cryptocurrency exchange monitoring platform. It is a monorepo with two main components:

- **Backend** (`backend/`): Go 1.25, pure `net/http` (no framework). Ingests real-time market data from multiple crypto exchanges via CCXT, normalizes it into unified domain models, stores it in Redis, and serves it via REST/WebSocket APIs. Two independent binaries: `collector` (data ingestion) and `api` (HTTP server).
- **Frontend** (`frontend/`): TypeScript, Next.js 16 (App Router), React 19, Tailwind v4, shadcn/ui. Dashboards for visualizing tickers and exchange data.

Architecture: collector and API are separate processes communicating through Redis, enabling independent scaling.

## Setup

### Prerequisites

- Go 1.25+
- Node.js 20+
- Redis 7+
- Docker & Docker Compose (optional)

### Backend

```bash
cd backend
cp configs/config.example.yaml configs/config.yaml  # edit as needed
make build        # builds both collector and api binaries
make run-collector  # build + run the collector
make run-api        # build + run the api server
```

### Frontend

```bash
cd frontend
npm install
npm run dev       # dev server at localhost:3000
```

### Docker (full stack)

```bash
cd backend
make docker-up    # Redis + collector + API via docker-compose
make docker-down  # tear down
```

## Build & test commands

### Backend (`backend/`)

| Command | Purpose |
|---|---|
| `make build` | Build both `collector` and `api` binaries |
| `make collector` | Build collector only |
| `make api` | Build API only |
| `make test` | Run all Go tests (`go test ./...`) |
| `make lint` | Run `golangci-lint run ./...` |
| `make vet` | Run `go vet ./...` |
| `make tidy` | Run `go mod tidy` |
| `make clean` | Remove build artifacts |

### Frontend (`frontend/`)

| Command | Purpose |
|---|---|
| `npm run dev` | Start dev server |
| `npm run build` | Production build |
| `npm run lint` | ESLint |

## Code style

### Go (backend)

- **Standard project layout**: `cmd/` for entrypoints, `internal/` for private packages, `pkg/` for shared utilities.
- **Naming**: snake_case for filenames, PascalCase for exported identifiers, camelCase for local variables.
- **Error handling**: Always wrap errors with context using `fmt.Errorf("context: %w", err)`. Never discard errors silently.
- **Interfaces**: Define at the point of use. Accept interfaces, return concrete structs.
- **Functional options**: Use the `With*` pattern for configurable constructors (see `exchange/exchanges/bybit/options.go`).
- **Concurrency**: `errgroup` for goroutine lifecycle, buffered channels for data pipelines, context-based cancellation throughout.
- **Logging**: `log/slog` with structured fields (`exchange`, `symbol`, `error`). JSON format by default.
- **HTTP**: Pure `net/http` with `http.ServeMux` (Go 1.22+ method-based routing). No web framework.
- **JSON envelope**: All REST responses use `{ data, error, ts }` shape.
- **Financial precision**: `shopspring/decimal` for all price/volume fields, serialized as strings in JSON.
- **Redis keys**: Namespaced via `pkg/rediskey` with `gembo:<exchange>:<type>:<symbol>` convention.
- **Minimal dependencies**: Only add a dependency when reimplementing would be non-trivial.

### TypeScript/React (frontend)

- **File naming**: kebab-case for files (`app-header.tsx`), PascalCase for components.
- **`"use client"`** directive where client-side interactivity is needed.
- **Path aliases**: `@/*` maps to project root.
- **Styling**: Tailwind v4 utility classes with semantic color tokens. Use `cn()` from `lib/utils` for conditional classes.
- **UI components**: shadcn/ui (`radix-mira` style, `lucide-react` icons).
- **Data fetching**: TanStack React Query with typed API client functions in `lib/api.ts`.
- **Strict TypeScript**: `strict: true` in tsconfig.

## Architecture notes

- **Plugin architecture for exchanges**: Adding a new exchange requires only creating an adapter directory under `internal/exchange/exchanges/` and registering it in the registry. No changes elsewhere.
- **Domain models** (`internal/domain/`) have zero external dependencies except `shopspring/decimal`.
- **Store and hook interfaces** (`internal/store/`, `internal/hook/`) are abstracted. Redis and BullMQ are the current implementations.
- **Configuration** is YAML-based with env var overrides (12-factor). See `configs/config.example.yaml`.
- The frontend proxies `/api/*` requests to the backend at `localhost:8080` via Next.js rewrites.

## Testing

- Run `make test` from `backend/` before submitting changes.
- Run `make lint` and `make vet` to check for issues.
- Run `npm run lint` from `frontend/` for TypeScript/React linting.
- Add or update tests for any code you change, even if not explicitly asked.
- All tests and lints must pass before a PR is mergeable.

## PR & commit conventions

- Commit messages should be concise and describe the "why", not just the "what".
- Keep PRs focused on a single concern.
- Run all lints and tests before committing.

## Security

- Never commit secrets, API keys, or credentials. Use environment variables or `configs/config.yaml` (gitignored).
- Exchange API credentials are configured via env vars, not checked into source.

## Nested AGENTS.md

The `frontend/` directory has its own `AGENTS.md` with Next.js-specific instructions. That file takes precedence when working within the frontend.
