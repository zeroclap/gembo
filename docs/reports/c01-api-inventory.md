# C01 — API inventory slice

29 October 2026. State: **awaiting approval**. C01 and Phase 0 acceptance remain open.

Source/base: `3dc539cd125764709e288ce5d17f10725e505ea3`, local `main` and
`origin/main`. GitHub repository metadata confirms default branch `main`;
remote is `origin`, `git@github.com:zeroclap/gembo.git`. Branch:
`codex/roadmap-c01-api-inventory`. Index, tracked files and untracked list were
clean at entry. No pre-existing changes are included or excluded.

Requested outcome: next roadmap package. Selected scope is a bounded first
C01 slice: current API registrations and proposed CLI paths, documentation only.
C01 depends on D01, whose instruction artifacts exist but whose independent
review remains unrecorded. D02 and D03 artifacts and reports were inspected;
D03 is present in the source commit despite its historical report saying
publication had not started. GitHub PR search returned only D02 PR #1, no C01
PR. Local remote refs match the source; this is not a fresh remote-head fetch.
Preparation does not claim predecessor acceptance.

## Deliverables and evidence

- [Readable inventory](../cli/api-inventory.md): all 35 route registrations,
  proposed commands and follow-up packages; current HTTP/event behavior and
  missing contracts distinguished.
- [Machine inventory](../cli/api-inventory.json): preliminary unique IDs,
  route/handler/source, authentication, source implementation status,
  explicit missing CLI status and null runtime evidence.
- This handoff records the bounded publication set and remaining gates.

C01's complete API/UI/utility/domain registry, command option contracts,
OpenAPI/event/machine schemas and full parity report are **not complete**.
The inventory lists the remaining work explicitly. The next bounded package
is the remaining C01 inventory and schema work, subject to predecessor review;
C02 must not treat this partial slice as completed C01 evidence.

## Validation

Executed against the source commit plus the three proposed documentation files:

| Check | Actual result |
|---|---|
| Python standard-library JSON/source reconciliation | Pass: exact set equality against all 35 `HandleFunc` registrations; 35 unique IDs; every handler present in its cited source; every CLI gap explicit and runtime evidence null. |
| Python local Markdown link resolution | Pass for inventory and handoff. |
| `git diff --check` and explicit trailing whitespace check on new files | Pass. |
| Application tests/build/lint/vet | Not run: this slice changes documentation/data only. Repository checks remain required before any commit; prior D03 baseline records unresolved failures, not waived here. |
| Services, stores, live exchanges and CLI scenarios | Not run; no runtime claim or operational change. |

Validation used inline Python assertions, with no generated logs or caches.
Route reconciliation detects omissions in this snapshot, not semantic option
coverage or future CI drift. No executable validator is shipped in this slice.
Before publication, required repository checks must run and remaining failures
must be resolved or explicitly excepted; artifact approval alone is not a waiver.

## Publication set

All new content in exactly these three files is proposed for commit:

1. `docs/cli/api-inventory.md`
2. `docs/cli/api-inventory.json`
3. `docs/reports/c01-api-inventory.md`

Proposed commit: **`Expose current API gaps before building CLI commands`**.
Destination: `origin/codex/roadmap-c01-api-inventory`.
Draft PR base: `main`; title: **`Inventory current API operations for C01 CLI planning`**.
No commit or push has occurred. All generated artifacts are included above;
there are no private or temporary evidence artifacts. No deployment, migration,
private access, cloud purchase or exchange execution is involved. Removing these
three documentation files reverses the slice.

The [roadmap-stage skill](../../.agents/skills/gembo-roadmap-stage/SKILL.md)
requires presenting artifacts **before any commit or push**. Owner approval of
this set authorizes publication steps, subject to the required check gate above;
independent review and package acceptance remain separate.
