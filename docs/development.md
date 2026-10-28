# Repository and development guide

D01 source reconciliation and D02 verification tooling, 28 October 2026. This guide describes existing interfaces. The [delivery plan](delivery/README.md) defines future work; the unified `gembo` CLI does not exist yet. Root `make bootstrap`, `make test-verification` and `make baseline` now provide the [isolated D02 baseline](verification.md). Other proposed verification/CLI targets remain planned. See the [D01 audit](reports/d01-repository-instructions.md) and [D02 handoff](reports/d02-reproducible-baseline.md) for checks and limitations.

## Process and package map

All backend paths below are relative to `backend/`. Service entrypoints are `cmd/<name>/main.go`, implementations are `internal/service/<name>/`, and Make outputs are `bin/<name>`.

| Process | Dependencies and responsibility |
|---|---|
| `market-data-collector` | Public exchange APIs → normalized markets/tickers/funding → Redis; optional BullMQ hooks. No account keys. Starts pprof on `:6060` by default. |
| `account-data-collector` | PostgreSQL credentials → private exchange positions/balances/funding → Redis snapshots and Pub/Sub updates. Requires database URL; starts pprof on `:6061`. |
| `api` | Redis plus optional PostgreSQL; REST `/v1/*`, authenticated arbitrage WS `/v1/arbitrage/ws`, login/account/position/workspace operations. PostgreSQL is needed for normal user authentication. |
| `arbitrage-worker` | Redis ticker/funding Pub/Sub → funding/spread calculations → Redis opportunity state and updates. Requires enabled exchanges in configuration. |
| `position-manager` | Redis signals, PostgreSQL, and per-user exchange credentials; order execution, monitoring, funding, reconciliation. Can affect real accounts. |

`internal/domain/` defines models. `internal/exchanges/base/` defines `Exchange`, `PrivateExchange`, and registries; `base/ccxtutil/` holds shared mapper, retry, and transport helpers. `internal/exchanges/<exchange>/` holds adapters; `factory.go` wires them. `internal/pkg/` holds arbitrage/recommendation/simulation, hooks, logging, and credential encryption. `internal/store/` defines interfaces and Redis/PostgreSQL implementations. `pkg/rediskey/` defines market, arbitrage, position, and user-scoped key/channel names.

PostgreSQL migrations are embedded from `internal/store/postgres/migrations/`: 12 numbered up/down pairs. API (when DB configured), account collector, position manager, and `create-user` currently run migrations on startup. A controlled release migration job is future work.

Public factory entries: Binance, Bybit, OKX, Gate, HTX, Aster. Private factory entries: Binance, Bybit, OKX (requires passphrase). KuCoin exists as an adapter but has no factory entry. These are wiring observations, not qualification claims.

Frontend routes under `frontend/app/` include login, dashboard, arbitrage, account, and transfers. Components, hooks, `lib/api.ts`, and stores implement the client. Some displayed functionality is mocked; complete capability classification belongs to C01. HTTP `/api/:path*` rewrites to `http://localhost:8080/:path*`. The WS hook connects directly to `/v1/arbitrage/ws` on browser hostname port 8080 unless overridden. Browser WS auth/subscription defects remain D07/C06 work; a running UI is not evidence that streaming works.

## Existing commands

Run backend commands from `backend/`. `make build` builds only the five services, not the utilities. The root Makefile is for isolated verification; it does not start application processes.

| Commands | Source behavior |
|---|---|
| `make build` | Builds all five service targets below |
| `make market-data-collector`, `make account-data-collector`, `make api`, `make arbitrage-worker`, `make position-manager` | `go build -trimpath -ldflags="-s -w"` of matching `cmd/` directory into `bin/` |
| `make run-market-data-collector`, `make run-account-data-collector`, `make run-api`, `make run-arbitrage-worker`, `make run-position-manager` | Builds matching service, then executes its binary |
| `make test` | `go test ./...`; includes `cmd/exchange-probe` and package-local tests; optional `TEST_FLAGS` supplies JSON/fresh-run flags for baseline evidence |
| `make vet` | `go vet ./...` |
| `make lint` | `golangci-lint run ./...`; no tracked custom lint configuration |
| `make tidy` | `go mod tidy`; modifies dependency files |
| `make clean` | Removes `bin/` |
| `make docker-build` | Attempts five image builds; four requested runtime targets are missing from the Dockerfile, which also builds removed command paths. Blocked pending D04. |
| `make docker-up` | `docker compose -f deployments/docker-compose.yml up -d --wait`; full development stack, including execution process |
| `make docker-down` | `docker compose -f deployments/docker-compose.yml down`; named volumes retained |

For a single actual package/test, use `go test -run TestSortedExchangeIDs ./internal/store/redis`. A subtest can be selected with `go test -run TestPositionKeys/PositionPending ./pkg/rediskey`; use `-v` to see selected cases. This test needs no Redis server. API WS tests currently connect to `localhost:6379`, DB 15, and skip when unavailable. Do not attach them to shared Redis. Disposable integration isolation and mandatory dependency enforcement belong to D05.

Specialist utilities are invoked from `backend/` with `go run ./cmd/<utility>`; these compile source and are not an installed product CLI:

| Utility | Interface and effects |
|---|---|
| `exchange-probe` | `-config`, `-exchanges`, `-symbols`, `-timeout`, `-market-sample`, `-funding-sample`, `-ws`, `-private`. Makes real exchange calls; defaults to Bybit/OKX/Binance/Gate public checks. Private checks use config/env fields, not stored user credentials. HTX/Aster are not wired in this utility. Missing-required-symbol semantics need D12 repair. |
| `create-user` | `-email` and `-password` required; connects to PostgreSQL, runs migrations, inserts a user. Password currently travels in process arguments; use synthetic local accounts only until protected input is implemented. |
| `ccxt-raw-positions` | `-config`, `-symbol`; queries Bybit and OKX directly using config/env credentials and prints raw private positions. Not a public-monitoring startup prerequisite; output is private evidence. |

Run frontend commands from `frontend/`, using the committed npm lockfile:

| Command | Source behavior |
|---|---|
| `npm ci` | Installs locked dependencies; pinned Node/npm are provided by the D02 toolchain |
| `npm run dev` | `next dev`, port 3000 by default |
| `npm run build` | `next build` |
| `npm run start` | `next start`; requires production build |
| `npm run lint` | ESLint with Next core-web-vitals/TypeScript presets |

No frontend test or separate typecheck script exists. There is no frontend Dockerfile or browser test configuration. Consult `frontend/AGENTS.md` and installed Next.js guides before frontend code changes.

## Configuration inventory

Sources: [loader](../backend/internal/config/config.go), [tracked YAML](../backend/configs/config.yaml), [environment example](../backend/.env.example), and service entrypoints. No private `.env` file is needed for this inventory.

All eight backend entrypoints call `godotenv.Load()` for `.env` in their working directory; existing environment values take precedence. `CONFIG_PATH` selects YAML (default `configs/config.yaml`); the two exchange utilities also accept `-config`. The loader applies built-in defaults, YAML, then nonempty environment overrides. An absent YAML file is allowed. Relative paths depend on working directory. Some invalid numeric/duration overrides silently retain prior values; this is existing behavior, not a validation guarantee.

| YAML fields | Environment overrides / notes |
|---|---|
| `server.host`, `server.port`, `server.read_timeout`, `server.write_timeout` | `SERVER_HOST`, `SERVER_PORT`; timeouts YAML-only. Built-in address `0.0.0.0:8080`. |
| `redis.addr`, `redis.password`, `redis.db` | `REDIS_ADDR`, `REDIS_PASSWORD`, `REDIS_DB`; built-in `localhost:6379`, DB 0. |
| `database.url` | `DATABASE_URL`; no built-in URL. Required by account collector, position manager, and create-user. API connects/migrates only when nonempty. |
| `auth.jwt_secret`, `auth.token_ttl` | `AUTH_JWT_SECRET`, `AUTH_TOKEN_TTL`; built-in static development signing secret and 24h TTL. Override the secret; production fail-closed enforcement is D08. |
| `security.credentials_encryption_key_base64` | `CREDENTIALS_ENCRYPTION_KEY_BASE64`; base64 32-byte key. Cipher is enabled only when configured; absence does not currently fail startup. |
| `exchanges[].id`, `.enabled`, `.markets_poll_interval`, `.funding_poll_interval` | `EXCHANGES` is an exact enabled-ID filter; unlisted YAML exchanges are disabled, missing IDs added with 60s poll intervals. Intervals otherwise YAML-only. |
| `exchanges[].api_key`, `.secret`, `.passphrase` | `<ID>_API_KEY`, `<ID>_API_SECRET`, `<ID>_API_PASSPHRASE` with uppercase ID. Legacy fields consumed by specialist utilities. Private runtime services load per-user credentials from PostgreSQL; public factory deliberately omits credentials. |
| `market_filter.quote_assets`, `.settle_assets`, `.market_types` | `MARKET_QUOTE_ASSETS`, `MARKET_SETTLE_ASSETS`, `MARKET_TYPES`; comma-separated. Empty filters mean unrestricted. Tracked YAML selects USDT perpetual markets. |
| `hooks.bullmq.enabled`, `.queue_prefix` | `BULLMQ_ENABLED`, `BULLMQ_QUEUE_PREFIX`; built-in disabled, prefix `gembo`. |
| `trader.leverage`, `.margin_mode`, `.funding_check_interval` | `TRADER_LEVERAGE`, `TRADER_MARGIN_MODE`, `TRADER_FUNDING_CHECK_INTERVAL`; legacy YAML/env names remain despite process rename. Built-in margin mode `cross`, tracked YAML `isolated`. |
| `log.level`, `log.format` | `LOG_LEVEL`, `LOG_FORMAT`; built-in info/JSON. |
| No YAML field | `PPROF_ADDR`: collector debug server address; defaults `:6060` public / `:6061` account. Keep debug access local/private. |
| Frontend only | `NEXT_PUBLIC_WS_HOST`: public WS host override, otherwise browser hostname plus `:8080`; protocol follows page HTTP/HTTPS. HTTP rewrite is hardcoded in `next.config.ts`. |

`backend/configs/config.yaml` and `.env.example` are tracked; `.env` is ignored. Never write credentials into tracked defaults. Runtime user credential encryption depends on separately retained key material. This reconciliation is not a secrets/history audit.

## Development Compose

The definition uses Redis 7, PostgreSQL 17, RedisInsight, a Go dependency warmup container, six separate public collectors, account collector, API, arbitrage worker, and position manager. Backend services mount source and use `go run`; there is no frontend service. Backend containers share `backend/.env`; explicit Compose `environment` values override matching env-file entries.

Compose publishes Redis 6379, PostgreSQL 5432, RedisInsight 5540, API 8080, and Gate collector pprof 6060. PostgreSQL uses development credentials and persistent `pgdata`; Redis persistence is disabled. Go module/build caches and RedisInsight have named volumes. This is a development stack, not an isolated test harness or production deployment. Use dedicated local dependencies for development; do not run the complete stack as the baseline verification step.
