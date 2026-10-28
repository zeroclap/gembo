# D01 — repository instructions reconciliation

Date: 28 October 2026. Source revision: `e16c5658b6f7524a3e193da8bea07050b7c71f7b`. Working tree was clean before D01 edits. Scope: documentation and environment-example comments only; no application, infrastructure, or dependency changes.

D01 deliverables are implemented locally and source checks pass. Independent review and the D02 execution baseline remain outstanding. This report supersedes stale setup instructions, not the historical observations in the original delivery baseline.

## Changes delivered

- Root README and AGENTS now use the five current process names and existing Make targets; PostgreSQL, frontend, utilities, service packages, and exchange factory wiring are explicit.
- The [repository guide](../development.md) maps processes/stores/packages, all Make targets and npm scripts, specialist utility interfaces, startup effects, complete config/env groups, and Compose behavior.
- Frontend README now follows the committed npm lockfile and describes actual backend routing and verification commands.
- Environment-example comments include Aster public wiring, authentication overrides, and the distinction between stored runtime credentials and legacy utility config/env credentials.
- Setup guidance identifies the full Compose stack's execution process, tracked YAML defaults, current migration behavior, missing product CLI, and unqualified exchange/browser paths.

## Validation evidence

[Command audit data](d01-command-audit.json) records target names, frontend scripts, tool availability, configuration override names, and Docker mismatches. All documented Make commands were matched to `backend/Makefile`; all documented npm script commands were matched to `frontend/package.json`. `npm ci` is the lockfile installation command, not a package script. Utility paths and flags were checked against their `main.go` files. Commands were checked against source; none of the run commands was executed.

| Check | Result and meaning |
|---|---|
| Make target inspection and `make -n <target>` | Pass: all 19 explicit targets expand successfully. This validates command wiring only. |
| Service paths | Pass: all five service entrypoints and implementation directories exist; Make build paths match. |
| Frontend scripts | Pass: `dev`, `build`, `start`, `lint` match package.json. No test script exists. |
| Utility interfaces | Pass: all three command directories and documented flags match source. |
| Config inventory | Pass: loader override names are documented, including dynamic exchange credential names; entrypoint `CONFIG_PATH`/`PPROF_ADDR` and frontend WS override checked separately. No private `.env` read. |
| Local documentation links | Pass: updated/new Markdown links resolve to local files. |
| Whitespace | Pass: `git diff --check`. |
| Image packaging source check | Known failure: four requested runtime targets missing; four removed build paths. Recorded below for D04. No image build attempted. |
| Go build/tests/vet/lint | Not run: Go and golangci-lint unavailable on shell PATH. |
| Frontend install/build/lint | Not run: Node and npm unavailable on shell PATH. |
| Live services, DB migrations, exchange probes, cloud operations | Not run; not needed for D01 source reconciliation. |

To repeat Make wiring checks without executing services, run from `backend/`:

```bash
for target in build market-data-collector account-data-collector api arbitrage-worker position-manager run-market-data-collector run-account-data-collector run-api run-arbitrage-worker run-position-manager tidy vet test lint clean docker-build docker-up docker-down; do
  make -n "$target" || exit 1
done
```

From the repository root, `git diff --check` verifies changed tracked text. Dependency installation, actual application checks, and deterministic isolation are D02/D05 work. No existing shared Redis was used, no credentials were accessed, and no service was started.

## Remaining bounded work

| Package | Evidence and next outcome |
|---|---|
| D02 | Go/Node/npm/linter absent on this shell PATH; frontend runtime versions unpinned. Establish credential-free pinned tooling, then execute and retain baseline build/test/lint results. |
| C01 | No `cmd/gembo`; utilities are partial specialist tools. Inventory API/UI/domain capabilities and define CLI coverage/contracts. |
| D04 | Make requests `market-data-collector`, `account-data-collector`, `arbitrage-worker`, `position-manager` runtime stages absent from Dockerfile. Dockerfile builds removed `cmd/collector-public`, `cmd/collector-private`, `cmd/worker`, `cmd/trader`. Repair packaging and configuration injection; add frontend packaging if shipped. |
| D05 | WS tests use localhost Redis DB 15 and skip on connection failure. Add disposable configurable dependencies and fail required integration checks when absent. |
| D07 / C06 | Frontend WS sends no bearer header and uses subscription fields different from server expectations. Verify a delivered authenticated update via CLI first; qualify UI before exposure. |
| D08 | Development signing fallback and optional credential cipher remain runtime behavior. Config/env documentation cannot enforce production security. Add focused negative tests and fail-closed behavior. |

No delivery phase is declared complete by this documentation change. Next independent packages after D01 are C01, D02, and D03, as specified in the roadmap.
