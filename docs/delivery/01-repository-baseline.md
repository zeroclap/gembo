# Repository baseline

[Plan index](README.md)

Assessment date: 28 October 2026. Inspected checkout: `1188c918ecd8d9373c7a713ef8fad62c7f0b78ff`. The working tree was clean before this documentation was added. GitHub metadata identifies `zeroclap/gembo` as public, with default branch `main`; auto-merge is currently disabled.

This was source and configuration inspection, not an execution audit. Application tests, container builds, exchange probes, credentials, deployed resources, branch protections, and recovery procedures were not validated. Go, Node, and npm were not available on the shell PATH used for this assessment; Docker and GitHub CLI were present. No application or exchange operation was started.

## Existing assets to preserve

| Area | Observed implementation | Implication |
|---|---|---|
| Backend | Go `1.25.0`; CCXT Go pinned at `v4.5.44` | Establish a baseline before attempting dependency upgrades |
| Main processes | `market-data-collector`, `account-data-collector`, `api`, `arbitrage-worker`, `position-manager` | Use these current names in delivery tooling |
| Data | Redis plus PostgreSQL, with 12 numbered migration pairs | Database migration and restore testing are required |
| Frontend | Next.js `16.2.1`, React `19.2.4`, lockfile committed | Add pinned Node/npm tooling; use installed Next.js guides when changing code |
| Tests | 30 tracked Go test files across domain calculations, adapters, API, config, credentials, reconciliation, and utilities | Extend these tests instead of replacing them with an agent-only test system |
| Integration utility | `backend/cmd/exchange-probe` supports market/funding checks, optional WS and private reads | Evolve into the exchange assurance entry point |
| Command-line surface | Specialist utilities `exchange-probe`, `create-user`, and `ccxt-raw-positions`; no unified product CLI found | Build a supported `gembo` client over shared service contracts; existing tools do not establish full feature coverage |
| Containers | Development Compose with per-exchange public collectors, PostgreSQL, Redis, and backend services | Useful starting point, but not a production definition |
| Private data | Credential encryption helper and account-scoped data code exist | Qualification should verify fail-closed configuration and tenant separation |
| Shutdown/recovery | Context cancellation, retry utilities, startup position reconciliation exist | Test failure paths and recovery semantics rather than assuming them |

Evidence: [Go dependencies](../../backend/go.mod), [Makefile](../../backend/Makefile), [entrypoints](../../backend/cmd), [frontend dependencies](../../frontend/package.json), [migrations](../../backend/internal/store/postgres/migrations), [probe](../../backend/cmd/exchange-probe/main.go).

## Concrete discrepancies to turn into work

| Finding | Evidence | Required response |
|---|---|---|
| Documentation describes older binaries, directories, and a much smaller application | [Root README](../../README.md), [AGENTS.md](../../AGENTS.md), actual `cmd/` and `internal/service/` trees | Reconcile instructions and commands before unattended implementation |
| Dockerfile builds removed entrypoint paths and exposes targets inconsistent with the current Makefile | [Dockerfile](../../backend/deployments/Dockerfile), [Makefile](../../backend/Makefile) | Repair all build targets; require clean-checkout container builds |
| Dockerfile copies `configs/config.yaml`; that file is tracked, despite the supplied convention describing it as ignored | Dockerfile and `git ls-files` | Audit values and history privately, separate safe defaults from secrets, and exclude credentials from image context; this assessment did not establish that secrets are present |
| Compose uses source mounts, `go run`, a shared `.env`, development database credentials, public host ports, and Redis without persistence | [Compose](../../backend/deployments/docker-compose.yml) | Keep a development profile; create a production deployment definition with explicit service secrets and ports |
| No tracked GitHub workflows or infrastructure-as-code were found | Tracked-file inventory | Introduce CI and provisioning as first-class artifacts; remote settings still need inspection |
| No frontend test framework or browser test configuration found | Frontend package scripts and file inventory | Add browser integration tests and focused unit/component coverage |
| Application features are primarily exposed through HTTP handlers and frontend clients, without a complete CLI | [Routes](../../backend/internal/service/api/routes.go), [frontend API client](../../frontend/lib/api.ts), entrypoint inventory | Inventory every capability and map it to a tested CLI workflow; prioritize CLI E2E |
| Some visible functionality is mocked, rather than backed by a real feature | [Transfers](../../frontend/components/transfers/transfers-page.tsx), [dashboard](../../frontend/app/dashboard/page.tsx), [P&L component](../../frontend/components/dashboard/pnl-section.tsx) | Track mock/planned status explicitly; do not present fabricated data as real CLI feature coverage |
| Redis-dependent WS tests skip if `localhost:6379` is unavailable and use DB 15 | [WS tests](../../backend/internal/service/api/websocket_test.go) | Provision disposable Redis, inject its address, fail required integration suites on missing dependencies, and report skips |
| Browser WS connection does not send the bearer header required by the API | [Frontend hook](../../frontend/hooks/use-arbitrage-websocket.ts), [routes](../../backend/internal/service/api/routes.go), [auth middleware](../../backend/internal/service/api/middleware.go) | Design browser-compatible authentication and reproduce the flow in a real browser; a Go client setting headers is insufficient evidence |
| Browser subscription sends `long_exchange`/`short_exchange`, while server expects `exchange_a`/`exchange_b` | Frontend hook and [WS implementation](../../backend/internal/service/api/websocket.go) | Version and test the shared message contract, including an actual delivered update |
| Frontend HTTP rewrite assumes `localhost:8080`; WS defaults to browser hostname port 8080 | [Next config](../../frontend/next.config.ts), frontend WS hook | Introduce production routing/TLS and test deployed browser URLs |
| Health endpoint always returns success | [Market/API handlers](../../backend/internal/service/api/markets.go) | Separate process health, dependency readiness, and data freshness |
| JWT has a static development default; credential storage passes plaintext through when cipher is absent | [Config](../../backend/internal/config/config.go), [Postgres user store](../../backend/internal/store/postgres/user.go) | Production startup must reject missing/invalid required secrets; test configuration enforcement |
| API offers a global flush route behind ordinary authentication; WS origin check accepts every origin | Routes and WS implementation | Remove/disable production flush or restrict through explicit administration; enforce origin policy |
| Service worker caches successful API responses and serves them after network failure | [Service worker](../../frontend/public/sw.js) | Test cross-user leakage, logout cleanup, private-data exclusion, and stale-data presentation before private operation |
| Multiple processes run migrations on startup | [API](../../backend/cmd/api/main.go), account collector and position-manager entrypoints | Move production migrations to one controlled release job; application identities should not need DDL authority |
| Startup reconciliation logs failure and continues | [Position manager](../../backend/internal/service/position-manager/trader.go) | Require successful reconciliation before new live execution; investigate existing recovery tests |

These are inspection findings and hypotheses for qualification, not a complete security or correctness audit. Fixes should be focused PRs with reproducible acceptance cases.

## Exchange scope actually wired today

| Exchange | Public factory | Private factory | Existing probe |
|---|---|---|---|
| Binance | Yes | Yes | Public and private reads |
| Bybit | Yes | Yes | Public and private reads |
| OKX | Yes | Yes; passphrase required | Public and private reads |
| Gate | Yes | No | Public only |
| HTX | Yes | No | Not wired |
| Aster | Yes | No | Not wired |
| KuCoin | Adapter directory exists | Not wired | Not wired |

Source: [exchange factory](../../backend/internal/exchanges/factory.go) and probe. “Wired” means source construction exists, not that the integration works. The probe currently reports missing requested markets in its detail while returning success for that check; required-symbol behavior needs an explicit assertion.

Do not assume every adapter supports production, testnet, demo, REST, WS, private reads, and writes equally. No verified environment-selection contract was found in the inspected public constructors/configuration. Add and test one before secret-bearing sandbox execution.

## First evidence bundle

The first implementation should produce a dated baseline report containing:

1. Commit, OS/architecture, Go/Node/npm versions, dependency lock hashes, and sanitized environment profile.
2. Clean dependency installation, all binary builds, frontend production build, tests, lint, vet, and container build outcomes.
3. Explicit passed/failed/skipped/not-run counts, dependency availability, timing, and peak build memory.
4. A capability-to-CLI gap inventory and the first planned headless auth → HTTP data → authenticated stream journey with synthetic data; record the lack of a product CLI honestly until implemented.
5. Existing test inventory mapped to critical boundaries; missing coverage is visible.
6. Failures converted to bounded work packages with reproduction and ownership.
7. No live API success claims until a separate live probe has actually run from the intended region.

Retain failing results. A green summary obtained by suppressing failures, silently skipping integrations, or replacing real checks with mocks is not an acceptable baseline.

The CLI-first requirement changes implementation priority, not these observations. Browser mismatches remain defects in the exposed UI, but browser parity is no longer the primary application acceptance path. See the [CLI contract](06-cli-contract.md) for complete feature inventory and parity rules.
