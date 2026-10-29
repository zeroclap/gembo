# D02 — reproducible toolchain and baseline

Date: 28 October 2026. State: **awaiting approval**. D02 and Phase 0 acceptance remain open. Publication has not started.

Source/base: `444da1c14ca758ca0a253d16a85233cd19f81c58`, `main` / `origin/main`. GitHub confirmed the default branch and source head; no existing D02 PR was found. Task branch: `codex/roadmap-d02-reproducible-baseline`. The checkout was clean before D02; there are no pre-existing changes to exclude.

Scope: pinned tooling, isolated bootstrap, baseline evidence and reproduction documentation. Application Go/TypeScript source and dependencies are unchanged. The backend Makefile adds optional `TEST_FLAGS`; plain `make test` retains its behavior. Application repairs, database/image harnesses, CLI implementation and deployment are outside this package.

D01 source deliverables were rechecked: all 19 documented Make targets expand, five service paths match, and four npm scripts match the command audit. The [D01 handoff](d01-repository-instructions.md) still leaves independent review open; D02 preparation does not establish predecessor acceptance.

## Deliverables and evidence

- [Tool manifest](../../tools/verification/toolchain.json) and [digest-pinned tooling image](../../tools/verification/Dockerfile): Go 1.25.14, Node 22.23.4, npm 10.9.9, golangci-lint 2.15.0. The existing npm lockfile pins ESLint 9.39.4 and TypeScript 5.9.3.
- [Runner](../../scripts/verification/baseline.py) and [reproduction guide](../verification.md): clean source/home/caches; no inherited `.env`, host credentials or local Redis. Only dependency installation has network access; application checks run with isolated loopback and no network.
- [Compact baseline](d02-baseline.json): all 21 checks, actual commands or declared templates, exit codes, timings, memory peaks, failures, test skips, source/lock/tool hashes and platform. Repetitive container arguments and per-file hashes remain in local diagnostics.
- [Harness tests](../../scripts/verification/test_baseline.py): **8 pass**, covering export/isolation, fingerprints/symlinks, skip accounting, empty evidence, timeout/failure handling, formatting findings, report rendering and Make compatibility.

Authoritative run: `d02-20261028T191734Z-f2cb3941`, 19:17:34–19:24:38 UTC, Linux arm64 on macOS arm64. Source snapshot: `7864f5fbe06bf9fbcf5d9b4a48239964400e656ac21dede136b3dfb0b8b0dc28` (246 exported application files). The recorded runner/Dockerfile hashes and lock hashes match the proposed files. The dirty flag includes the uncommitted Makefile edit. Linux amd64 has not been executed.

## Actual outcomes

Overall: **11 pass, 4 fail, 1 skip, 5 not-run**. Baseline exit code **1** is retained; this is complete red evidence, not release approval.

| Boundary | Result |
|---|---|
| Bootstrap and dependencies | Tool image/version verification, clean Go download and npm install pass. Module verification and lock integrity pass; dependencies are unchanged. |
| Backend build | All five service binaries and eight command packages, including utilities, build. |
| Go tests | `make test TEST_FLAGS='-json -count=1 -timeout=120s'`: exit 0, **342 pass / 2 skip terminal test events**, including parent/subtest events. 13 packages pass; 22 package-level skips identify packages without tests. The two Redis WS case skips are listed explicitly in JSON. |
| Go analysis | Vet passes. Gofmt flags existing `simulation_test.go`. Go lint reports **31 issues**: 23 errcheck, 1 ineffassign, 3 staticcheck, 4 unused. |
| Frontend lint | Pass, with **3 warnings**. |
| Frontend build | Fail: existing Google Fonts imports need network access for Geist Mono and Space Grotesk. An online production build was not qualified. |
| Frontend types | Fail: `active-position.tsx:457` references `long_credential_id` and `short_credential_id`, absent from `Position`. |
| Not run | Application images (D04), disposable DB integration (D05), frontend test runner (D06/C06), CLI E2E (C01/C02/D07), live exchange probes (D12/D13). Reasons and commands are in JSON. |

The 30 existing Go test files cover calculation/simulation, adapters/helpers, config/security, API, position reactors/reconciliation, Redis utilities and the probe. This is an inventory, not evidence of real-store, private exchange, browser, CLI or production qualification. The two skipped WS cases contacted only empty container loopback.

The cold service build took **316.805 seconds** and peaked at **9,499,074,560 bytes (8.85 GiB)** cgroup memory. The profile uses a 10 GiB container ceiling, two CPUs, `-p=1 -trimpath`, `GOMEMLIMIT=6GiB`, `GOGC=50`. An earlier 6 GiB attempt hit its ceiling and the CCXT compiler was killed; that interrupted attempt is preserved locally. These measure build requirements, not production runtime sizing.

## Remaining gates and operational implications

D06 must address the format defect and 31 Go lint findings. D04/D06 must resolve the external font build dependency and two frontend type errors. D05 must replace Redis attachment/skip behavior with mandatory disposable stores. C01/C02/D07 must deliver the CLI contract and first headless journey. Independent D01/D02 review and the C01/D03 Phase 0 deliverables remain open; a red baseline is allowed for D02, not a waiver of later release gates.

No cloud resources, expenditure, private accounts, scheduled agents, application services or exchange operations were used. This tooling change needs no migration or deployment; reverting its files removes it. The next selected package is **D03**, subject to predecessor review; it has not started.

## Publication set and artifact disposition

The complete proposed commit is **14 files**, with all changed/new hunks owned by D02:

- `.gitignore`, `Makefile`, `README.md`, `backend/Makefile`;
- `tools/verification/.dockerignore`, `tools/verification/Dockerfile`, `tools/verification/toolchain.json`;
- `scripts/verification/baseline.py`, `scripts/verification/test_baseline.py`;
- `docs/development.md`, `docs/verification.md`, `docs/delivery/README.md`;
- `docs/reports/d02-reproducible-baseline.md`, `docs/reports/d02-baseline.json`.

Proposed commit: **`Make baseline verification reproducible and isolated`**. Push: **`origin/codex/roadmap-d02-reproducible-baseline`**. Draft PR base: **`main`**; title: **`Establish a reproducible, credential-free baseline`**. There is no unrelated staged content, D02 commit, pushed branch or PR yet.

Only this handoff and the compact baseline JSON are included as D02 reports. Raw logs, full JSON with per-file hashes, the duplicate Markdown index, resource-limit attempt, harness output, validation and the previous review manifest are local-only under `.artifacts/d02-review/d02-20261028T191734Z-f2cb3941/`, ignored by Git. The JSON records the full raw baseline's hash. This archive is absent from a fresh checkout; `make baseline` regenerates diagnostics. Earlier `/tmp/gembo-d02-*` downloads/development attempts remain temporary outputs excluded from publication. Exported sources, binaries, `.next`, dependency caches and temporary homes were removed; Python bytecode and reusable Docker images/build cache remain local and excluded. No private payload is involved.

Cleanup verification compares the compact summary with the preserved raw evidence, validates JSON/links, confirms the exact 14-file publication set and verifies the archive is ignored. Tooling and application inputs were unchanged by this cleanup, so their recorded checks remain applicable.

Owner approval of this revised set is still pending. The [roadmap-stage skill](../../.agents/skills/gembo-roadmap-stage/SKILL.md) requires review **before any commit or push**; artifact approval authorizes only the described commit, normal push and draft PR creation/update.
