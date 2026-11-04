# Gembo roadmap

> **Status: PROPOSED.** Waiting for owner approval in [#1](https://github.com/zeroclap/gembo/issues/1). Nothing here authorizes implementation, deployment, private-account access, or trading.
> The evidence is in the baseline report on #1. Task details and decisions live in the linked issues, not in this file.

## Product direction (proposed)

Gembo is an operator-run tool for **cross-exchange perpetual-futures arbitrage**: funding-rate and price-spread opportunities. It is built in stages, and each stage must be trustworthy before the next one starts:

**observe one venue correctly → observe across venues correctly → read own accounts → execute with hard safety limits.**

As product direction, this replaces the monitoring-only scope of [`backend/docs/high-level.prd.md`](backend/docs/high-level.prd.md) (v0.1). The PRD remains the reference for the market-data foundation.

### Operating constraints (owner-stated)

1. **Real money.** Production is isolated: a separate GCP project and separate credentials. Agents plan; the owner applies.
2. **API drift is expected.** Every change to exchange or CCXT code ships with an integration test for each API call and an e2e scenario run against the real public APIs.
3. There is no sandbox or staging environment.
4. **CLI-first.** UI work is frozen until the owner explicitly unfreezes it. The owner reconfirmed this on 2026-11-04.
5. Code documentation lives next to the code. Reports and other non-product artifacts go in GitHub issues.
6. **Latency matters, within reason.** Measure first. Do not accept large infrastructure or complexity for marginal gains.
7. Tooling: Go, GitHub Issues/Projects/Actions, Copilot CLI agents, GCP + Terraform, and free-tier Postgres (Supabase or similar) during beta.

## Where we are (baseline, 2026-11-04)

This is a summary of the report on #1.

- **Works:** all 8 binaries build, and `go vet` and `go test -race` pass. Public market data for Bybit, Binance, Gate, and HTX flows from collector to Redis to API.
- **Broken:**
  - OKX WebSocket tickers fail (OKX is retiring port 8443; the pinned CCXT treats the notice as fatal).
  - Arbitrage output is not trustworthy: suspected instrument-identity collisions produce spreads of up to 199%, funding is mis-normalized when the interval is unknown, and quotes up to 59 s old are accepted.
  - Lint shows 45 issues, govulncheck shows 4 vulnerabilities, `go.mod` is not tidy, the Dockerfile targets binaries that don't exist, and the frontend type check fails.
- **Never exercised:** private account data, the position manager (which places real market orders), and Postgres. CI, a dry-run mode, a kill switch, and notional caps do not exist.

## Now and Next: milestone M1, first operator outcome

**[M1: Trustworthy read-only Bybit market data](https://github.com/zeroclap/gembo/milestone/1)**

**Outcome:** with one command, an operator gets a verified answer to *"is Gembo's Bybit perpetual market data (markets, top-of-book tickers, funding rates) correct and fresh right now?"*. The same check gates relevant PRs and runs on a schedule. Failures are reported as GitHub issues.

**Why Bybit:** in the baseline, Bybit had complete ticker data (0 zero quotes out of 786) and BTC/ETH funding with a known 8 h interval. It has the deepest existing adapter, including trading for later stages, and it streams over WebSocket. The downside is that it geo-blocks US CI runners, so CI needs non-US egress (#4). The alternative is Gate, which is reachable from US runners but has no private adapter and returned zero-price tickers in the baseline. **The owner decides.**

**Ordered backlog:**

| # | Issue | Depends on |
|---|---|---|
| 1 | #2 CI quality gate (build, vet, `test -race`, tidy, new-only lint; frontend lint + tsc when `frontend/**` changes) | none |
| 2 | #3 Reconcile `AGENTS.md`, Makefile, and Dockerfile with the actual binaries | #2 |
| 3 | #4 Non-US egress CI runner on GCP (Terraform, ephemeral, no credentials, no fork PRs) | #2, owner GCP project |
| 4 | #5 Bybit public API integration tests, one per CCXT call, asserting raw → domain mapping | #4 (for CI) |
| 5 | #6 Keep exchange timestamps; validate and flag stale tickers and funding | #2 |
| 6 | #7 Bybit e2e: live + deterministic replay, operator command `make e2e-public EXCHANGE=bybit` | #4, #5, #6 |
| 7 | #8 Go 1.26 toolchain and vulnerable module patches (CCXT stays pinned) | #2, #5, #7 |

**Acceptance criteria (milestone):**

- `make e2e-public EXCHANGE=bybit` exits non-zero on missing, stale, invalid, or unknown data. It requires `BTC/USDT:USDT` and `ETH/USDT:USDT`, a minimum number of valid observations in a bounded window, exchange timestamps, freshness under a measured threshold, and funding with a known interval and next-funding time. A negative test with a stale feed proves that it fails.
- The replay e2e runs on every code PR. The live e2e and integration tests run on PRs that touch exchange, collector, or store code or `go.mod`, and on a schedule. A scheduled failure opens or updates an issue.
- Nothing in this path uses exchange or account credentials. The e2e uses an ephemeral random API secret.

**Non-goals:** other exchanges, cross-exchange arbitrage, private or account data, trading, UI, production deployment, and a CLI redesign.

## Then: correct cross-exchange observation ([#9](https://github.com/zeroclap/gembo/issues/9))

**Outcome:** funding and spread opportunities between Bybit and one or two more venues are **correct**: instrument identity is verified, funding intervals are normalized correctly, contract sizes and fees are applied, and invalid or stale quotes are excluded. They are also **timely**: exchange-to-opportunity latency is measured against an agreed budget.

**Scope:** CCXT upgrade tested on all enabled venues (fixes OKX WS), instrument-identity checks, funding normalization, venue data-quality fixes, a measured arbitrage-recompute latency improvement, probe coverage, and per-venue integration and e2e suites.

**Acceptance:** no opportunity is produced from an unverified pair or from an unknown funding interval. Latency percentiles are published, with a budget that the owner agrees to.

**Non-goals:** account data, execution, and UI.

**Dependencies:** M1 is done, and the owner approves this as the goal and picks the venues. Child issues will be filed then, using evidence from M1.

## Later: each item needs its own owner decision ([#10](https://github.com/zeroclap/gembo/issues/10))

**Scope:**

- **Production isolation on GCP:** project, Terraform, Workload Identity Federation, Secret Manager, beta Postgres, observability, runbook.
- **Private read-only account data:** read-only keys only.
- **Execution:** needs dry-run, kill switch, notional caps, client order IDs, a refusal to start with unsafe auth configuration, and reconciled e2e evidence. The position manager exists but stays disabled until then.
- **Single `gembo` operator CLI.**
- **UI unfreeze.**
- **PRD replacement.**

**Acceptance:** a decision issue is recorded for each item before any work on it starts.

**Non-goals:** anything not approved.

**Dependencies:** #9 for anything that touches opportunities or execution.

## Forecast: M1 only

| | |
|---|---|
| **Scope** | 7 issues (#2–#8) |
| **Engineering effort** | P50 8 / P90 15 engineer-days, AI-assisted. Breakdown: CI 0.5–1.5, drift 0.5–1, egress 1.5–3, integration tests 1.5–3, timestamps 1–2.5, e2e 2–3, toolchain 0.5–1 |
| **Owner and external wait** | GCP project and billing, a Terraform apply, the exchange choice, and review of about 7 PRs. Not estimable by us; typically the largest share of calendar time |
| **Calendar** | P50 3 weeks / P90 6 weeks, if the owner responds within 2 working days |
| **Confidence** | Low–medium. There is no delivery history in this repo, so these figures are uncalibrated. Recalibrate after #2 and #4 close |
| **Evidence** | Baseline on #1: the build and unit tests are green, and the Bybit public pipeline works from non-US egress. Defects sit in the data-validation, dependency, and CI layers, not in the core design |
| **Main risks** | Bybit API or CCXT behaving differently under strict assertions, the GCP runner setup, and owner availability |

## Review cadence

Review this file when a milestone closes, or when the owner asks. Update it in the same PR as the work that changes it. Changes of direction need owner approval in an issue.
