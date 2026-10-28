# Agents and GitHub coordination

[Plan index](README.md)

## Start with one useful role and one harness

An agent is a versioned role plus tools, execution environment, inputs, outputs, budget, and permission policy. A role prompt alone is not an operational service. The first role is **verification**: inventory tests, run the approved harness, classify failures, reproduce issues, and propose missing acceptance cases. Initially launch it manually against a selected issue/revision. It can help build the testing foundation through focused PRs while its authority remains bounded.

Agents operate Gembo through the complete, noninteractive `gembo` CLI defined in the [CLI contract](06-cli-contract.md). The UI is a human-facing client and a subject of UI tests, not a prerequisite for application operation. The verification role inventories missing CLI capabilities from the beginning and proves primary workflows through the executable.

Introduce other roles when a recurring workload or a phase gate needs them. Separate role profiles do not require separate permanently running models. One disposable runner can execute a verification role today and an infrastructure planning role tomorrow, with different credentials and tools.

Use the Codex GitHub Action for the first hosted runs. Keep model, effort, Codex CLI/action versions, prompts, tool profile, and schemas versioned. Pin the Gembo CLI artifact/protocol separately. Structured output is supported through an output schema; parse it, but verify reported artifacts and test results independently. [Codex GitHub Action](https://learn.chatgpt.com/docs/github-action), [non-interactive structured output](https://learn.chatgpt.com/docs/non-interactive-mode).

For a later application-controlled runtime, compare the Codex SDK, managed Agents API, and an application-owned agent loop against actual requirements. The managed Agents API documents persisted sessions and managed orchestration, but it still needs application tools, access controls, and an execution environment. Do not make its state the only work record. Revalidate product availability and pricing before adoption. [OpenAI agent runtime choices](https://developers.openai.com/api/docs/guides/agents), [Agents API overview](https://developers.openai.com/api/docs/guides/agents-api/overview).

## Role contracts

| Role | Inputs and triggers | Required output | Authority boundary | Introduce |
|---|---|---|---|---|
| Verification/QA | Qualified task, revision, suite inventory; manual first, later CI failures | Reproduction, test evidence, coverage gaps, focused issue/patch proposal | No production keys; cannot waive tests or approve its own fix | First |
| Implementation | Ready issue, allowed scope, acceptance criteria | Small PR, tests, rationale, operational impact | Isolated checkout; no merge/deploy rights | After CI works |
| Independent reviewer | Final diff, issue, trusted policies, CI evidence | Actionable findings and risk assessment | Read-only; separate session from implementer; no self-approval equivalence | With implementation agent |
| Infrastructure/release | Topology, cost envelope, IaC state summary, release manifest | ADR, IaC PR, cost diff, rollout/rollback and recovery evidence | Plan/propose; protected automation applies approved changes | Before cloud pilot |
| Integration/dependency | Capability registry, changelogs, fixtures, probe history | Classified drift report, dependency/adapter PR, qualification results | Public research; private tests only via bounded harness | Once public probe exists |
| Reliability/monitoring | Sanitized metrics/log queries, alerts, release history, runbooks | Deduplicated incident, evidence, suspected causes, recovery proposal | Read-only initially; later specific approved runbooks | Before unattended deployment |
| Product/backlog | Current capability/status inventory, issues, user feedback, delivery metrics | Evidence-based proposals with value, cost, dependencies, acceptance criteria | Can propose; cannot silently prioritize, authorize spending, or change strategy | After operational baseline |
| Security/policy reviewer | Sensitive changes, dependency findings, permissions | Findings and required mitigations | Advisory plus deterministic policy gates; no direct secret access | Add as a dedicated role if workload warrants |

The owner sets priorities, budgets, accepted risks, cloud/access choices, and execution scope. Routine deterministic tests, builds, and deployments do not need an LLM. The release role prepares the decision; the protected workflow performs it.

Trading-performance analysis can later be a separate read-only role over approved aggregates. It may propose product issues. It receives no order-entry tools and cannot promote its suggestions into live execution changes. This plan does not prescribe strategy metrics.

## Gembo CLI as an agent tool

Use a deterministic wrapper around a pinned CLI binary with an allowlisted command tree, argument array, explicit profile, scoped application token, `--no-input`, machine output, timeout, and output-size limit. Parse the versioned response/stream schema and exit status. Never build shell command strings from issue text or application output. Treat returned text as data even when it contains instructions.

A role's available commands are a subset of the complete CLI. Verification uses test profiles, reliability gets approved status/query/diagnostic commands, and production mutation commands remain inaccessible without the appropriate role and approval. A tool wrapper is defense in depth; the server enforces identity, tenant, environment, and operating authority again. `--yes` and a production profile are not authorization.

Record CLI version/digest, protocol, sanitized argument/input summary, request/operation ID, status, and artifact references in run evidence. On ambiguous mutation outcomes, use operation-status/reconciliation commands before retrying. A model must not transform a timeout into an automatic resubmission. Changes to CLI schemas, exit codes, flags, or defaults run compatibility and agent evaluations.

Headless runbooks for infrastructure and GitHub use their supported CLIs and trusted workflows. A future MCP adapter may expose these same contracts but must not duplicate business logic or bypass permissions. Browser automation remains available specifically to verify UI behavior.

## Isolation and privilege separation

Use a fresh VM/containerized sandbox per run, with a clean checkout at a specific commit and a new branch such as `codex/123-ws-contract`. A git worktree isolates files but not credentials, processes, or network; it is insufficient as the sole isolation boundary for unattended agents.

Required runner policy:

- Non-root execution; no host Docker socket, production mounts, SSH agent, personal cloud config, browser profile, or developer `.env`.
- A private writable workspace and disposable credentials; read-only trusted policy bundle pinned separately from candidate changes.
- Resource limits for CPU, memory, disk, subprocesses, duration, tool calls, model usage, and outbound requests.
- Egress allowlists for the assigned role, with DNS/redirect handling and metadata-service access controlled. An issue-provided URL is not automatically an allowed destination.
- No production network route by default. Test databases and ports belong to the run.
- Controlled tool installation; pin approved skills/plugins/MCP tools and review updates as executable dependencies.
- Preserve a sanitized evidence bundle, terminate descendants, revoke credentials, and destroy the environment on completion or timeout.

Separate four jobs/identities:

| Boundary | What it can do | What it must not inherit |
|---|---|---|
| Agent worker | Read task/repository; produce patch and evidence | GitHub write, deployment identity, exchange secrets |
| Publishing broker | Validate patch scope/size/provenance; create branch/PR or update approved issue fields | Model execution, candidate scripts, cloud production access |
| Verifier | Execute candidate tests in a fresh environment; publish trusted check result | Worker transcript as proof, production secrets, unrestricted GitHub mutation |
| Deployer | Promote approved digest/configuration with narrowly scoped cloud identity | Arbitrary repository commands, model-generated shell, unapproved image |

The broker rejects path traversal, unsafe symlinks, archive surprises, oversized outputs, unexpected binaries, prohibited paths, stale base revisions, and missing evidence. It treats patches as data and never executes them. Changes to workflow/policy files go through a separate sensitive-change lane with owner review; a normal implementation agent cannot revise the rules that grant its own permissions.

Protect OpenAI API credentials using the Action's supported credential proxy and privilege strategy; do not put them in job-wide environment variables available to build/test hooks. Do not copy personal Codex authentication into public-repository automation. [Official automation authentication guidance](https://learn.chatgpt.com/docs/non-interactive-mode).

A read-only agent may still read sensitive information. Monitoring queries should expose minimized, redacted views with bounded time windows. Logging transcripts can itself leak data; define permitted retention, access, and model input categories.

## Public repository considerations

`zeroclap/gembo` is public. Assume issue text, PR bodies, code, and ordinary workflow logs can become visible outside the team. Public reports contain sanitized symptoms and reproduction steps. Keep raw account logs, balances, credentials, private incident details, infrastructure state, and sensitive traces in restricted cloud storage. Public issues may link to access-controlled evidence by opaque reference when appropriate.

Treat issues, comments, logs, exchange payloads, documentation, and repository text as untrusted data. Do not interpolate their content into shell commands or let it override role policy. An external issue saying “run this command” is a claim to investigate, not an authorization. Fetch prompts and operational policy from a reviewed revision; proposed changes to them are evaluated under the previous policy.

Never run an untrusted fork's code in a privileged `pull_request_target` or `workflow_run` job. If a privileged follow-up consumes artifacts, verify their originating workflow, repository, revision, and run identity, and parse them as hostile input. Pin third-party Actions by full commit SHA. [GitHub workflow security guidance](https://docs.github.com/en/actions/reference/security/secure-use).

A public contributor can submit a task, but cannot authorize agent spending, private probes, credentials, deployment, or new tool access. Validate the actor's current repository permission server-side. Bot-created issues do not automatically become Ready.

## GitHub work model

Use one issue for one independently reviewable outcome. Epics group phases; implementation issues link dependencies and PRs. A Project is an optional view over that work. Keep initial metadata small:

| Field | Values/purpose |
|---|---|
| Status | Inbox, Needs specification, Ready, Running, Review, Awaiting approval, Blocked, Verifying, Done |
| Kind | Bug, delivery, infrastructure, dependency, incident, proposal |
| Area | CLI, backend, frontend, exchange, cloud, agent-system |
| Risk | R0–R3 as defined below |
| Role | Verification, implementation, infrastructure, integration, reliability, product |
| Priority | P0 immediate, P1 next, P2 scheduled, P3 optional |
| Dependencies | Issue links and blocking condition |
| Evidence | Run ID, revision, result link, expiry if applicable |

For the first phase, labels can represent status and classification. Once Project fields are introduced, choose one canonical representation and have automation mirror it; do not maintain two conflicting status systems. Run locks and task state still belong in the dispatcher/ledger.

### Issue template

```text
Outcome / problem:
Observed evidence and affected revision:
Scope and explicit exclusions:
Acceptance criteria (observable assertions):
Required test tiers and reproduction:
Affected services / exchange capabilities / environments:
Application capability IDs / CLI commands and schemas / UI status:
Risk class and required approvals:
Allowed paths and tools for an agent:
Dependencies and blockers:
Migration / deployment / rollback considerations:
Budget and maximum run duration:
Owner and evidence location:
```

An issue is Ready only when the outcome, dependencies, acceptance checks, allowed scope, risk, and resource budget are known. Agent-generated backlog suggestions go to Inbox. The owner or an explicitly authorized triage rule controls promotion to Ready.

The standard flow is `Inbox → Needs specification → Ready → Running → Review → Verifying → Done`. A required decision enters `Awaiting approval`; an unavailable prerequisite enters `Blocked` with a specific owner/action. Revisions move work back to Running/Review and invalidate stale verification. Deployment-related issues remain Verifying after merge until production observation completes. A rejected proposal can close with rationale without pretending implementation succeeded.

### PR policy

PRs link the issue and include behavior change, CLI capability/contract impact, acceptance evidence, risk, migration/rollback implications, and remaining uncertainty. Application feature PRs require CLI coverage; UI lag is explicit and acceptable. Independent review uses a fresh session and independently executed checks. A second LLM is useful, but correlated errors are possible; human approval and deterministic checks remain authoritative where required.

Protect `main` against direct agent pushes, force pushes, deletion, and unreviewed sensitive changes. Require the final revision's checks and dismiss stale approvals after changes. Initially owner review is required for agent PRs. GitHub does not let authors approve their own PR: use App-authored agent PRs, and explicitly decide how owner-authored PRs receive independent review rather than configuring an impossible approval rule.

Use a narrowly scoped GitHub App installation token in the broker, not a personal general-purpose token in the agent. Current GitHub documentation distinguishes events from `GITHUB_TOKEN`: some PR events create approval-required runs, and other events do not chain automatically. App-token events can trigger workflows normally. Test the actual bot-PR → CI → merge → release chain rather than relying on remembered behavior. [GitHub workflow triggering](https://docs.github.com/en/actions/how-tos/write-workflows/choose-when-workflows-run/trigger-a-workflow).

## Task and result contracts

Create versioned JSON Schemas for task, result, approval, and release records. The following are field requirements, not a deployed API.

**Task envelope:** schema version; unique task/run ID; repository numeric identity; issue URL; role; base commit; policy/prompt/tool versions; objective; acceptance criteria; scope/allowed paths; allowed environment/capabilities; risk; input artifact hashes; deadline; maximum attempts/cost; approval references; and publisher target.

**Result envelope:** task/run ID; actual base/head commit and patch hash; status (`succeeded`, `failed`, `blocked`, `needs_approval`, `cancelled`); concise outcome; changed paths; test artifact references and counts; commands/exit codes; evidence classification; unresolved risks; requested next action; usage/cost; timestamps; and cleanup state.

For Gembo operations, the task also pins CLI version/digest, protocol, allowed commands and endpoint profile, and credential reference; the result adds application request/operation IDs and sanitized machine-output evidence. This is separate from the agent runtime's own CLI/session identifiers.

The runner, not the model, supplies commit identity, execution times, exit codes, consumed budget, and artifact hashes. The broker verifies these values. A model can explain a result, but cannot manufacture a trusted CI status by writing `passed: true`.

**Approval record:** authenticated human identity; action; environment; artifact/manifest hash; scope; limits; expiry; reason; policy version; and revocation state. It is validated immediately before the action. A changed artifact, expired approval, edited issue, or broader request requires re-evaluation. A label or natural-language “approved” comment alone is not a durable authorization token.

## Orchestration in two stages

### Stage one: GitHub-native, deliberately bounded

Start with manual `workflow_dispatch` selecting an issue and allowed role. Trusted code validates permissions, Ready state, dependencies, and budgets, then snapshots the task envelope. Allow one mutating agent run repository-wide initially. Read-only verification can run separately if resources and exchange budgets permit.

Use GitHub concurrency controls to prevent overlapping runs, but **do not treat the concurrency group as a durable FIFO queue**. Admission rejects an already-running task; undispatched work stays Ready in Issues. A reconciliation job periodically finds abandoned runs and eligible Ready issues. Initial manual dispatch avoids needing a reliable multi-producer queue before it is useful.

Record every attempt and its workflow/run URL. Allow at most two total implementation attempts per issue revision initially, each with a 30-minute execution budget and an approved usage cap. A failed attempt may produce a useful diagnostic; retry only for a classified transient cause or a bounded follow-up. Do not launch an infinite “fix CI” loop.

Scheduled work is appropriate for dependency review, broad probes, and summaries. GitHub scheduled runs can be delayed or dropped, so they must not be the only heartbeat for production safety. Monitor expected arrivals and reconcile missed work. [GitHub scheduling caveats](https://docs.github.com/en/actions/how-tos/troubleshoot-workflows).

### Stage two: durable dispatcher when needed

Introduce a small GCP control service when multiple workers, durable retries, webhook recovery, or granular leases become real requirements. Suggested components: Cloud Run webhook/admission service; Pub/Sub delivery; Firestore transactional task/lease ledger; Cloud Scheduler for reconciliation; restricted Cloud Storage artifacts; disposable agent workers. Keep it independent from application Redis/PostgreSQL so an application outage does not disable incident coordination.

Design for duplicate and out-of-order delivery. Use event IDs and an idempotency key such as `(repository, issue, issue-version, role, base-revision, policy-version)`. Atomically claim a task with a lease, expiry, heartbeat, attempt number, and fencing token. Only the current lease can publish a result. Expired workers cannot keep changing issue state or publishing PRs. [Firestore transactions](https://cloud.google.com/firestore/docs/manage-data/transactions), [Pub/Sub subscriber delivery](https://cloud.google.com/pubsub/docs/subscriber).

Persist transitions before dispatch; acknowledge delivery only after recording the durable claim/outcome. Reconcile GitHub, the ledger, worker status, and actual PR state periodically. A crash after branch creation must find and reuse the existing branch/PR using the task id, not create duplicates. Use a dead-letter/manual-review state after bounded retries.

Validate webhook signatures and replay IDs, recheck actor permissions, and fetch current GitHub state before expensive or privileged actions. Cancel work when the issue is closed, permissions are revoked, the policy changes incompatibly, or its base revision becomes stale. Rebase/retest rather than overwriting another agent's branch. Restrict concurrent changes to shared interfaces; use component ownership and dependency links before adding parallel agents.

The controller decides whether a run may start, publish, retry, or stop. It does not need an LLM to schedule work or enforce budgets. Agent summaries are supporting context, not the durable source of task truth.

## Risk classes and approval boundaries

| Class | Examples | Initial handling | Later autonomy |
|---|---|---|---|
| R0 | Read-only analysis, documentation, sanitized reports | Can run within approved budget; changes still reviewed | Automatic reports and eligible documentation PRs |
| R1 | Bounded application fix with good deterministic tests and no sensitive interface change | Agent PR + owner review | Eligible auto-merge after evaluation; deployment still separately governed |
| R2 | CCXT upgrades, auth, credentials, schema, execution code, infrastructure, workflow/policy changes | Owner approval plus specific qualification; independent review | Keep human approval unless a very narrow class is explicitly authorized |
| R3 | Production apply/restore, execution enablement, changed financial permissions, destructive resources, broader access/budget | Concrete action-specific approval through protected mechanism | Only narrowly preapproved runbooks; no blanket agent authority |

Risk is derived conservatively from changed paths and semantic effects; agents may raise it, not lower it unilaterally. Modifying tests, golden fixtures, gate configuration, or a role prompt to make a failure disappear requires explicit review.

Approvals should be concentrated at meaningful decision points: scope, resource/budget changes, sensitive merge, production promotion, and execution authority. Routine read-only investigation, local fixes, test reruns within budget, and evidence collection do not need repeated approval. An already-approved bounded runbook is permission to perform its defined action when its preconditions hold, not permission to improvise adjacent actions.

## Monitoring to actionable work

Use a deterministic alerting system before an LLM. A severe outage, lost heartbeat, or execution-control failure must notify the owner through the selected urgent channel even if GitHub or the model is unavailable. The reliability agent enriches the incident with evidence afterward.

Workflow:

1. Receive an alert or scheduled health result with service, environment, release, affected capability, severity, and timestamps.
2. Correlate it with recent releases, provider status, dependency health, and repeated occurrences.
3. Compute a stable fingerprint from service/capability/error class. Update an existing incident within its episode instead of creating an issue per log line; record a new episode after recovery/recurrence.
4. Gather a bounded sanitized evidence window and state the distinction between observed facts and suspected cause.
5. Create/update an issue with impact, reproduction, evidence, recommended next action, and resolution criteria. Rate-limit new issues; overflow goes to a summary for review.
6. If an approved runbook matches, validate preconditions, take the bounded action once, observe its postconditions, and record the outcome. Otherwise request a concrete approval or route to implementation.
7. Resolve only after the service-level condition recovers and the observation period passes. Preserve a follow-up defect if recovery was only mitigation.

First monitored categories: service crashes, API/WS failures, stale streams, reconnect storms, parsing rejects, DB/Redis errors, missed reconciliation, backup failures, expired credentials, failed releases, missing probes, agent failures, and budget/quota trends. Trading-performance proposals can be added later without changing this delivery model.

Notify immediately on urgent incidents, failed recovery, and required decisions; send one concise daily digest of completed work, blocked items, active risk, and spend, and a weekly planning summary. Stay quiet when monitored state is unchanged. Configure channel and cadence before enabling notifications.

## Agent quality and promotion

Evaluate the agent system itself. Use a fixed set of representative issues and adversarial inputs, including known repository failures from the baseline. Keep withheld cases that the implementing agent did not author. Evaluate model/prompt/tool updates before promotion, with a rollbackable versioned configuration.

Required scenarios include: wrong WS payload, missing Redis, flaky upstream, unsafe key request in an issue, malicious log instructions, false test-pass claim, attempted out-of-scope file change, stale approval, duplicate event, agent timeout, revoked permission, failed cleanup, stale base, and an orphaned PR.

Add CLI-specific scenarios: unknown flags, malformed JSON, output mixed with logs, expired service token, wrong profile, partial pagination, stream gap, client/server incompatibility, and accepted mutation followed by lost response. Evaluate a complete authorized workflow without a browser or terminal prompt, and verify the agent stops or reconciles correctly on denied/uncertain outcomes.

Measure accepted outcomes, independent defect detection, reopen/rollback rate, unauthorized attempts, false-positive incidents, human review effort, cost per accepted change, and time to useful evidence. Passing tests alone does not measure whether the agent solved the right issue.

Suggested promotion gates are policy starting points, not statistical proof:

- Report-only → PR authoring: all isolation/refusal tests pass and ten supervised tasks produce complete, accurate evidence.
- PR authoring → eligible R0/R1 auto-merge: at least twenty supervised eligible changes, no critical escaped defect or permission violation, all required checks reliable, and owner approval of an explicit allowlist.
- Read-only reliability → bounded remediation: each runbook passes at least three failure drills, has a single-action limit/cooldown, reliable postconditions, and a tested escalation path.
- Wider concurrency: no duplicate publication or lost ownership in fault tests, resource budget validated, and scope conflicts managed.

Immediately suspend the affected authority on a permission violation, repeated inaccurate evidence, unexpected destructive action, or an escaped high-severity defect. Fall back to report/PR mode while investigating. Maintain an owner-accessible stop control that disables new dispatch, revokes broker/deploy credentials, and cancels running workers without depending on an agent to comply.

## Durable knowledge and maintenance

Store architecture decisions, runbooks, capability registry, test results, and task outcomes as ordinary versioned documents and structured artifacts. Each has an owner, date, evidence links, and review/expiry policy. Chat history is optional context; it must not be the only place a decision or credential scope exists.

Keep role instructions short and reference shared policies. Do not duplicate the entire architecture into every agent prompt. Update instructions when repository commands or service names change; check documentation drift in CI. Review tool/plugin permission changes like dependency changes, and preserve the previous working agent configuration for rollback.
