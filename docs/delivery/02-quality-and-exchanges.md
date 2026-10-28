# Quality and exchange assurance

[Plan index](README.md)

## Establish three different kinds of confidence

Deterministic tests establish that a change preserves defined behavior. Live probes establish that an external API still behaves as expected. Deployed end-to-end tests establish that the CLI, application services, routing, credentials, and stores work together. Browser E2E additionally verifies features exposed in the UI. None substitutes for the others.

The CLI is the primary application acceptance surface. Follow the [CLI contract](06-cli-contract.md): full capability coverage, machine schemas, noninteractive behavior, and client/server compatibility are release requirements. UI coverage may be a subset; missing UI support is not a failed CLI feature.

Start by retaining the pinned CCXT version and measuring the baseline. An upgrade may be necessary, but first determine whether a failure comes from application code, library behavior, configuration, an exchange change, or the execution environment. Compare a proposed library upgrade on a separate branch using the same suite.

## Test layers and gates

All durations below are starting execution budgets, not measured performance guarantees. Tune after collecting baseline timing. Required jobs must report when they cannot execute.

| Layer | What runs | Trigger and environment | Gate |
|---|---|---|---|
| Static/build | Go/CLI format, build, vet, pinned golangci-lint; frontend lint/type/build when affected or shipped; lockfile and secret checks | Every relevant code PR, disposable runner | Required; no credentials |
| Unit/property | Existing Go tests; mapper edge cases; domain invariants; focused frontend logic tests; bounded Go fuzz cases where useful | Every relevant PR | Required; normally under 5 minutes after cache warmup |
| Component integration | Actual Redis/Postgres, migrations, store queries, tenant isolation, REST auth and WS contracts | Every backend/contract PR; disposable services | Required; under 10 minutes initially; missing DB is failure |
| CLI contract/conformance | Built executable; input/output schemas, exit status, stdout/stderr, auth, pagination, non-TTY, cancellation, version skew, capability mapping | CLI/API/feature PRs and every release candidate | Required; no new supported feature without complete CLI coverage |
| Deterministic system E2E | Real CLI and services, controlled exchange simulator/replay, real stores; no frontend required | Relevant PRs and every release candidate | Primary application gate; initially under 15 minutes |
| Browser E2E | Production frontend build against real services and controlled data for exposed UI features | UI/shared-contract changes and releases shipping the UI | Required for shipped UI behavior; does not require UI parity with CLI |
| Public API contract | Real public REST/WS through the exact adapter/library under test | Scheduled, manually dispatched, and candidate qualification | Bounded per exchange; distinguishes broken behavior from unavailable evidence |
| Private read contract | Dedicated read-only test account balances/positions and private WS lifecycle | Trusted scheduled workflow; candidate qualification when affected | Protected credentials; never arbitrary PR execution |
| Exchange testnet/demo | Create/query/cancel orders and reconcile states where supported | Reviewed execution/integration candidate | Required for relevant capabilities; sandbox-only keys/endpoints |
| Staging deployment | Installed CLI auth/query/watch, HTTPS API/WS, migrations, freshness, logs, resource limits, restarts; browser smoke if UI deployed | Every release candidate | Required before production promotion |
| Production smoke | Synthetic CLI auth/read/watch, deployment and CLI identity, data freshness, dependencies; targeted UI smoke if deployed | After deployment plus continuous external checks | Automatic halt/rollback under documented conditions; no synthetic live orders |
| Resilience/recovery | Dependency outage, restart, delayed/duplicate messages, restore, split ownership, interrupted deployment | Nightly subset; full suite before execution enablement and after material infrastructure changes | Required for the applicable operating mode |

Every code change needs appropriate behavioral evidence. For simple documentation changes, link/schema validation is sufficient. Do not create tests that merely restate implementation. Existing uncovered areas are tracked separately so broad “coverage improvement” does not hide a specific failure.

## Reproducible harness

Create a versioned development/test image or tool manifest with Go, Node/npm, lint tools, browsers, and database versions. A fresh checkout must run without a developer's home directory, existing Redis, `.env`, or API keys. Pin the actual compatible toolchain, not just a major version; inspect the installed Next.js 16 guides before frontend implementation.

Add proposed root entry points such as `make verify`, `make integration`, `make e2e-cli`, `make e2e-ui`, and `make probe-public`; `make e2e` selects the release's required suites. They may delegate to backend targets and frontend scripts. Document working directory, prerequisites, environment profile, exit codes, artifact locations, and cleanup. Names become a stable contract once merged. Keep these engineering commands distinct from the installed `gembo` application CLI.

Use per-run containers, randomized host ports, a unique Compose project name, disposable database credentials, and seeded synthetic users. CI must not attach to a developer's DB 15 or flush shared Redis. Explicitly destroy resources on success, failure, cancellation, and timeout; a periodic janitor removes abandoned test resources by run ID and TTL.

Keep fixture generation explicit. Tests must not regenerate their own expected results during ordinary verification. Seed migrations and data through supported interfaces; do not bypass auth in CLI or browser tests. Required integration jobs parse test JSON/JUnit to detect unexpected skips and empty test selection.

Use Go race detection for concurrency-sensitive packages, then expand to the full backend when runner capacity supports it. CCXT compilation may be expensive: measure clean/warm build RAM and duration, cache by toolchain/platform/lock hash, and separate trusted caches from untrusted contributions. Build binaries in CI, not on the small production VM.

Playwright supports running against Linux CI with its browser dependencies installed. Pin compatible browser and package versions. [Playwright CI documentation](https://playwright.dev/docs/ci).

Use Vitest for pure frontend logic and React Testing Library for focused client-component behavior, with versions selected against the pinned Node/React environment. Keep Next.js routing, server rendering, hydration, service workers, and authenticated browser behavior in the production-build Playwright suite. Do not introduce two competing browser test frameworks. [Vitest setup](https://vitest.dev/guide/), [React Testing Library](https://testing-library.com/docs/react-testing-library/intro/).

Before the first cloud sizing decision, run a bounded synthetic load test with explicit user count, symbol count, update rate, and WS connection count. Record p95 latency, memory, CPU, dropped updates, and reconnect recovery. Add a two-hour staging soak for collector changes initially and a 24-hour sandbox/reconciliation soak before live execution qualification. Establish acceptable bounds from measurements and owner objectives; a process that stays alive while silently losing data does not pass.

## First CLI acceptance journey

Implement this as the first vertical slice, without starting Next.js:

1. Start disposable Redis/Postgres, apply migrations once, and bootstrap a synthetic principal through the controlled administrative path.
2. Run real API/worker binaries and a deterministic exchange feed; build and install the candidate `gembo` executable into a clean environment.
3. Discover local command schemas, authenticate without a browser/TTY, and inspect identity, environment, and permissions.
4. Query a known market with `--no-input --output json`; validate the schema, decimal values, freshness, stdout/stderr separation, and exit status.
5. Subscribe to the authenticated stream using NDJSON with a count/deadline bound; inject an update and assert the exact delivered change.
6. Interrupt the connection; verify reconnect, resubscription, snapshot, and explicit gap reporting, without claiming unavailable replay.
7. Run an allowed simulation or saved-workspace mutation, verify it through a separate CLI process, and exercise asynchronous operation status where relevant.
8. Expire/revoke credentials and attempt cross-user and unauthorized actions; assert stable machine errors and no side effects.
9. Close stdin, interrupt a process, break the output pipe, and exhaust a deadline; verify bounded cleanup and honest outcome reporting.
10. Disable execution and attempt a mutation requiring execution approval against the fake adapter; the server rejects it and records no exchange action.

Expand this journey to every capability through the parity registry. Test CLI mutation/idempotency/approval behavior with deterministic or sandbox adapters before live qualification. The full subprocess, parser, transport, server, store, and result path must execute; testing only an HTTP client library does not qualify the CLI.

## Browser acceptance for exposed features

Retain this additional journey when shipping the UI:

1. Start real Redis and PostgreSQL, apply migrations once, and create a synthetic user.
2. Run the real API and arbitrage worker; inject a known stream through a fake exchange at the adapter boundary.
3. Launch the production Next.js build behind the intended local reverse-proxy routes.
4. Log in through the UI. Confirm unauthenticated API calls are rejected.
5. Read a known market through the UI and compare displayed values with the seeded decimal strings.
6. Establish browser-compatible authenticated WS access; subscribe using the agreed contract.
7. Emit a new observation, then verify that the browser changes to the new value. An open socket alone is not success.
8. Drop the socket, reconnect, resubscribe, and verify snapshot plus subsequent update behavior.
9. Expire/revoke the session, log out, then log in as a different user; verify protected data and cache separation.
10. Interrupt the API/network and verify stale/offline state is visible without presenting old data as current.

Choose the WS auth design explicitly: a same-origin secure cookie with CSRF/origin controls, or a short-lived single-use WS ticket obtained using existing bearer auth. Prefer a ticket initially if preserving the existing HTTP auth API minimizes change. Specify expiry, replay prevention, user binding, and logging redaction. Never put a long-lived bearer token into a URL. Do not “fix” WS by removing authentication.

Later cases cover account credentials, error states, pagination/filters, multi-user authorization, a disabled-execution UI, and service-worker updates across deployments. Most browser tests should use controlled data; keep a small separately labelled live smoke suite.

A CLI-only deployment may omit the frontend while these UI defects are resolved. Once the UI is exposed, its auth, cache, and supported journeys are required checks. The CLI must never depend on a browser workaround for authentication, streaming, or application state.

## Exchange capability registry

Create a machine-readable registry, for example `tests/exchanges/capabilities.yaml`, reviewed as source. Each record contains:

| Field group | Required information |
|---|---|
| Identity | Exchange ID, CCXT class, product/market type, environment (`production`, `testnet`, or `demo`) |
| Capabilities | Markets, ticker REST, ticker WS, funding, balances, positions, private WS, order create/query/cancel; supported/unsupported/unverified with reason |
| Contract | Required fields, optional fields, decimal/unit semantics, timestamps, symbol and contract-size conventions |
| Connectivity | Official endpoint references, tested host allowlist, required credential kind, region/egress profile, clock requirements |
| Test inputs | Small representative instrument set, account mode, minimum order constraints, reset/cleanup procedure |
| Rate budget | Per-method weights where known, connection/subscription budget, shared account/IP constraints, maximum requests and duration |
| Evidence | Last successful check, commit and CCXT version, runner region, result URI, expiry, responsible role |
| Exceptions | Unsupported behavior, temporary quarantine with owner and expiry, and effect on release eligibility |

The registry initially covers the six wired public adapters and three private adapters from the baseline. KuCoin remains explicitly unqualified until wiring and tests are intentional. Add HTX and Aster to the existing probe instead of creating separate ad hoc scripts.

## Validate meaning, not only response shape

Required checks include:

- Requested required symbols actually exist and belong to the intended market type; a missing required symbol fails its check.
- Symbols preserve spot/perpetual, settlement currency, linear/inverse, multiplier, and contract-size meaning.
- Decimal fields parse without silent float-based rounding or conversion of absence into valid zero.
- Funding zero is valid; missing funding is distinct. Timestamp units and funding interval are interpreted explicitly.
- Tickers distinguish last/mark/index/bid/ask; partial WS updates do not erase valid fields from prior observations.
- Per-field validity and freshness use exchange-aware rules. Do not impose a universal “price always positive” or bid/ask invariant on every optional field.
- Out-of-order and duplicate messages do not make current state older; reconnect does not silently lose subscriptions.
- Private snapshots with zero balances or no positions can be valid. Prove endpoint/auth success separately from collection size.
- Provider errors become classified failures, not empty success responses. Preserve rate-limit/reset information and retryability.
- Cancellation closes streams and terminates retry loops within a bound; no steadily growing goroutine or subscription count during reconnect tests.

Capture sanitized fixtures at two boundaries: raw provider REST/WS payloads and the corresponding CCXT/domain representation. Directly constructing a CCXT struct tests Gembo's mapper but does not test CCXT's parser. Add transport injection or a local HTTP/WS replay endpoint where feasible, so selected fixtures traverse the real library parsing path. If a library path cannot be replayed, record the gap and cover it with a live contract probe; do not claim equivalent coverage.

Fixtures carry provider, product, environment, capture date, library version, and provenance. Review fixture changes as behavior changes. Remove tokens, signatures, user IDs, balances, and order identifiers where unnecessary; keep private evidence in restricted storage. Ordinary PR tests must never refresh fixtures from production.

## Live probe policy

Extend `exchange-probe` to emit structured JSON and JUnit plus a readable summary. It must use the same factory/options as the application, report effective mode and host without credentials, enforce timeout and request budgets, and return distinguishable outcomes:

| Outcome | Meaning | Release handling |
|---|---|---|
| `pass` | Required assertions executed successfully | Eligible evidence for the tested revision/configuration |
| `regression` | Wrong parsing, contract mismatch, panic, or reproducible application failure | Blocks affected change/capability |
| `upstream_unavailable` | Provider outage or maintenance supported by evidence | Evidence incomplete; retry boundedly, hold affected release or explicitly disable capability |
| `environment_blocked` | DNS, regional restriction, egress policy, clock, or credentials prevents testing | Fix environment; never classify as a pass |
| `unsupported` | Registry explicitly says capability is not supported | Acceptable only outside the required scope |
| `not_run` | Check never executed | Visible gap; blocks a required gate |

Start with public probes every six hours, private reads daily, and candidate probes on relevant changes. Use one selected symbol per product first and stagger exchanges. These are proposed schedules to tune against observed limits and cost. Run a small public pipeline canary continuously in staging once deployed.

Serialize probes per credential and coordinate their rate budget with other clients using the same exchange/IP. CCXT's limiter belongs to an exchange instance; multiple processes still need a shared operating budget. Configure sandbox mode before any network call where supported, and use environment-specific keys. [CCXT manual](https://github.com/ccxt/ccxt/wiki/manual).

Run public checks on isolated runners with no account keys. Run private checks through a trusted test workflow and dedicated credential-bearing harness after the candidate code has passed the required review boundary. A read-only key can still be stolen by arbitrary test code; a secret environment variable inside an untrusted PR is not safe.

For production-region checks, use a disposable probe workload with explicit egress, not the production host's shell. Never route around an exchange restriction to force a green result; choose a permitted deployment region and account arrangement.

A required release probe must identify the candidate image/commit it tested. Evidence from yesterday's `main` does not qualify a new adapter/library change. Default validity window is 24 hours for affected live capabilities, with an immediate refresh after relevant code, dependency, credential, account-mode, or endpoint changes.

## Sandbox qualification and production execution

Testnet, demo, replay, and production are separate environment types. Do not use one boolean to imply they are equivalent. Bybit documents incomplete demo API coverage and distinct public/private stream behavior. Binance documents product-specific futures testnet endpoints. Verify each selected product against current official documentation during implementation. [Bybit demo service](https://bybit-exchange.github.io/docs/v5/demo), [Binance futures API information](https://developers.binance.com/en/docs/products/derivatives-trading-usds-futures/general-info).

Before sandbox writes, add a configuration validator that rejects mixed environment keys/endpoints and an outbound host allowlist enforced independently of the model. Logs must identify the selected mode. If an environment is unsupported, report it; do not silently fall back to production.

Test execution mechanics with deterministic fault injection first: accepted request followed by timeout, duplicate delivery, partial completion, cancellation race, restart during an operation, missing Pub/Sub notification, and reconciliation failure. Confirm idempotency and durable intent before external side effects. Never blindly retry an ambiguous create request: query/reconcile its identity first.

Sandbox suites use dedicated accounts, small bounded operations, unique test IDs, and before/after inventories. Cleanup must query and cancel owned orders and resolve remaining exposure under an explicit test policy. A cleanup failure leaves the environment quarantined and creates an urgent actionable result; a timeout does not imply cleanup succeeded.

Live execution qualification is a later owner-approved experiment, not ordinary CI. Bind approval to account, exchange/product, image and configuration, instrument allowlist, maximum operation/exposure limits, duration, stop conditions, and cleanup/reconciliation procedure. Disable withdrawals for execution keys. Withdrawals and cross-account transfers require a separate future scope; they are not granted by this plan.

No unattended live-money E2E test is required to make normal CI green. Sandbox success also does not prove all production semantics. Document the remaining differences and qualify them through a narrowly approved live canary.

## Dependency maintenance

Choose one dependency PR bot, initially Dependabot for Go, npm, Docker images, and Actions. Pin tool versions and action commit SHAs. Treat CCXT upgrades as their own change stream with release-note review, fixture replay, public live comparisons, and relevant private/sandbox qualification. Do not group them with unrelated UI or infrastructure upgrades.

The dependency agent can watch official exchange changelogs and CCXT changes, compare them with the capability registry, and open a proposal. It cannot upgrade production directly or alter a failing assertion to accept new behavior without review. Record semantic differences, required adapter changes, and rollback constraints.

Add dependency/secret/image scanning. Critical actionable security findings block release; any temporary exception needs an owner, rationale, affected scope, compensating measure, and expiry. Verify license compatibility when adding dependencies. Maintain a supported-version policy after the baseline; this plan does not claim that any pinned version is currently secure or compatible.

## Evidence, flakiness, and trust

Each suite emits: run ID; commit; image digest if applicable; CLI artifact/version and protocol version; tool versions; environment/config hash; suite and case names; start/end times; passed/failed/skipped/not-run counts; classification; sanitized logs; and reproduction command. CLI tests retain sanitized input/output, stderr, exit status, operation IDs, and capability coverage. Collect Playwright traces/screenshots on failure using synthetic accounts. Raw private payloads and tokens never enter public CI artifacts.

Permit at most one automatic retry for a transient browser/live failure initially, retain the original failure, and count flaky success separately. Quarantine requires a linked issue, named owner, and maximum seven-day expiry. Critical auth, execution-environment, tenant-isolation, and rollback checks cannot be silently quarantined.

Use coverage to locate blind spots, not as the only correctness metric. Track required-journey coverage, escaped defects, flake rate, time to reproduce, time since last successful capability check, and skipped tests. Test the test system by introducing known faults in an isolated evaluation branch: wrong WS field names, missing Redis, reversed decimal units, expired credentials, and a broken image target must all produce the intended failures.
