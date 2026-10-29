# D03 — first verification role contract

Date: 29 October 2026. State: **awaiting approval**. No D03 commit, push or PR
has been made. Package acceptance and Phase 0 acceptance remain open.

Source/base: `158eeaaa27eea6858e0e0bf081cd5c30d4d2b2b0`, the merged D02
revision on `origin/main`. Default branch `main` was confirmed through GitHub;
remote refs were fetched before branching. Task branch:
`codex/roadmap-d03-verification-contract`. The checkout, index and untracked
file list were clean at entry; no pre-existing changes are included or excluded.
There were no attached worktrees or PRs and no existing D03 PR.

D03 depends on D01. Its command/instruction deliverables exist in the selected
source; independent D01 review is still missing. D02's published baseline and
local raw archive were inspected, and GitHub PR #1 was confirmed merged with no
recorded reviews/comments. This permits independent D03 preparation, not a claim
that predecessor acceptance has been demonstrated.

## Scope and acceptance evidence

| D03 requirement | Delivered evidence |
|---|---|
| Versioned verification role | [Role v1.0.0](../../agents/roles/verification.md), [manual runbook](../../agents/README.md); shared plan policies remain authoritative. |
| Task/result contracts | Strict Draft 2020-12 [task schema](../../agents/schemas/verification-task.schema.json) and [result schema](../../agents/schemas/verification-result.schema.json); repository/revision, hash-bound bundle/input, scope, provenance, statuses, usage/unknowns, timestamps and cleanup. |
| Allowed commands/tools, budgets and stops | [Policy v1.0.0](../../agents/policies/verification.json) allows one evidence-only review: 30 minutes, one attempt, 1 MiB per input/output file, zero network/new paid API calls, no publisher or Gembo operations; conversational ceilings are supervised, not hosted enforcement. |
| Manual run on D02 evidence | [Task](d03-verification-task.json) and [result](d03-verification-result.json), run `d03-20261029t074717z-652eee9a`, retain all 21 D02 outcomes and their original revision/snapshot. Review succeeds with `evidence_verdict: gaps` and `release_eligible: false`. |
| Deterministic validation and negative cases | [Checker](../../scripts/verification/contract.py) and [11 behavioral tests](../../scripts/verification/test_contract.py): tampering, false passes, hidden skips, missing checks, stale/expired tasks, scope/budget violations, symlinks/size limits, command injection as data, overwrite refusal and executable error/usage exits. |
| Reproducible operator tooling | [Pinned validator dependencies](../../tools/verification/contract-requirements.txt), separate `make test-agent-contract`; D02's standard-library-only harness target remains available. Python 3.11+ is required by the new validator dependency set; the actual D03 run used 3.14.6 on macOS arm64. |

The task's `risk: R0` describes reading sanitized evidence. The **D03 artifact
change is R2** because it establishes policy, role and schema files; owner and
independent review remain required. The task is a contract rehearsal with no
issue dispatch or approval token. Its hash-bound bundle includes the uncommitted
D03 files; its base/head identify the selected Git source. It produces no patch,
so `patch_sha256` is null and `changed_paths` is empty. These fields describe the
role run, not this implementation's publication set.

## Validation

From the repository root, against the D03 working files on the source above:

| Exact command | Result |
|---|---|
| `.artifacts/d03-tools/bin/python -m pip install --no-cache-dir jsonschema==4.23.0` | Pass; isolated ignored virtual environment. Resolved versions were pinned in `contract-requirements.txt` and checked against installed metadata. Initial sandbox DNS failure was resolved by approved network access. |
| `make test-agent-contract PYTHON=.artifacts/d03-tools/bin/python` | Pass: 11 tests, no skips. |
| `make test-verification PYTHON=.artifacts/d03-tools/bin/python` | Pass: all 8 D02 harness tests. |
| `make test-verification` | Pass: the same 8 tests with host Python and no JSON Schema dependency. |
| `.artifacts/d03-tools/bin/python -m pip check` | Pass: no broken requirements. |
| `.artifacts/d03-tools/bin/python scripts/verification/contract.py prepare --task docs/reports/d03-verification-task.json --output docs/reports/d03-verification-result.json` | Pass: new task captures the selected source and trusted hashes. |
| `.artifacts/d03-tools/bin/python scripts/verification/contract.py review --task docs/reports/d03-verification-task.json` | Pass: imported **11 pass / 4 fail / 1 skip / 5 not-run**; 342 passed and 2 skipped Go case events retained. No application command executed by this checker. |
| `.artifacts/d03-tools/bin/python scripts/verification/contract.py validate --task docs/reports/d03-verification-task.json --result docs/reports/d03-verification-result.json` | Pass: schemas, hashes, task/result scope/identity, provenance, outcomes and timestamps. |
| `python3 scripts/verification/baseline.py --output .artifacts/d03-application-baseline` | Exit **1**, retaining **11 pass / 4 fail / 1 skip / 5 not-run**. All five services and eight command packages build. `make test TEST_FLAGS='-json -count=1 -timeout=120s'` exits 0 with 342 passed / 2 skipped case events; `make vet` and frontend `npm run lint` pass (three warnings). `make lint` fails with the same 31 issues; gofmt, offline frontend build and frontend typecheck retain their existing failures. |
| Local Markdown links, all 14 changed/new files' JSON and whitespace; `git diff --check` | Pass. The bound D02 input, trusted bundle and local D02 raw-baseline hash also match. |

The checker validates imported evidence against its bound input; it does not
authenticate the truth of D02's raw logs. New application test results are kept
separate from the manual role's historical D02 input.

Fresh application run: `d02-20261029T074716Z-fbc0bc86`, 07:47:16–07:55:00 UTC
(the unchanged baseline runner retains its `d02-` run prefix). Its exported
application snapshot hash is the same as the original D02 input:
`7864f5fbe06bf9fbcf5d9b4a48239964400e656ac21dede136b3dfb0b8b0dc28`.
The full local JSON is `.artifacts/d03-application-baseline/baseline.json`, SHA-256
`d647663a1cadd14e76bef97508097480c5ec19bd66392ebf57013af5553fec61`; it records
every exact container command, timing, measurement and skip. The four failures
remain one format defect, 31 lint findings, external Google Fonts in the offline
build, and two missing `Position` properties. The five not-run boundaries remain
images, disposable stores, frontend tests, CLI E2E and live probes. These results
are not waived by the passing D03 tests or proposed artifact approval.

## Remaining gates and operational implications

D02 findings remain bounded follow-ups: D04 packaging/offline font handling;
D05 mandatory disposable-store integration and Redis skips; D06 Go format/lint
and frontend types; C01/C02/D07 complete CLI inventory/foundation/journey;
C06 exposed UI qualification; D12/D13 exchange qualification. The D03 role
proposes those packages without changing their priorities or acceptance.

Model token/cost/conversational tool-call measurements are unavailable and are
null with a reason. The checker measures its wall time and enforces local
hash/path/byte/deadline rules; the operator supervises attempts and conversational
budgets. It is not a sandbox, authenticated task dispatcher, independent verifier
or publishing broker. D10 must pin trusted policy outside candidate content,
meter usage, enforce cancellation/isolation and attest results before hosted use.
Only the declared D02 evidence format is supported by this v1 checker.

No cloud, application service, database, account credential, exchange operation,
new model/API invocation or scheduled agent was used. Dependency installation
and the isolated application baseline use their separate download steps. No
migration/deployment is needed. Revert the D03 files to remove this manual
contract; D02's baseline remains usable. Artifact approval does not authorize
runtime activation, deployment, private access or execution.

The next package is **C01**, the application capability/CLI contract inventory,
subject to predecessor review. It has not started. C01 and independent reviews
are still needed before Phase 0 can exit.

## Publication set and artifact disposition

All changed/new hunks in the following **14 files** are owned by D03:

- `Makefile`;
- `agents/README.md`, `agents/roles/verification.md`, `agents/policies/verification.json`;
- `agents/schemas/verification-task.schema.json`, `agents/schemas/verification-result.schema.json`;
- `scripts/verification/contract.py`, `scripts/verification/test_contract.py`;
- `tools/verification/contract-requirements.txt`;
- `docs/delivery/README.md`, `docs/verification.md`;
- `docs/reports/d03-verification-contract.md`, `docs/reports/d03-verification-task.json`, `docs/reports/d03-verification-result.json`.

Proposed commit: **`Keep verification evidence bounded and explicit`**.
Push: **`origin/codex/roadmap-d03-verification-contract`**.
Draft PR base: **`main`**; title: **`Define the first manual verification role and evidence contract`**.
There are no unrelated staged files or outgoing D03 commits.

Only the role run's sanitized task/result and this handoff are proposed as
versioned D03 reports. Test logs and final validation manifests under
`.artifacts/d03-review/`, the isolated application rerun under
`.artifacts/d03-application-baseline/`, and the validator virtual environment
under `.artifacts/d03-tools/` are local diagnostics/tooling excluded by `.gitignore`.
Python bytecode, tooling images/build cache and temporary runner source/caches
are not publication artifacts; the runner removes its temporary sources/caches
and containers. The pre-existing D02 archive is unchanged and excluded.

Owner approval is pending. The [roadmap-stage skill](../../.agents/skills/gembo-roadmap-stage/SKILL.md)
requires presenting artifacts **before any commit or push**. Approval of this
identified set authorizes its commit, normal push and draft PR creation/update;
independent review and existing failing release checks remain separate gates.
