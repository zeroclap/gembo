---
name: gembo-roadmap-stage
description: Execute or resume a bounded Gembo delivery roadmap phase, increment, or D/C work package from docs/delivery, with dependency checks, artifact review, explicit approval, and Git publication. Use when asked to proceed with a roadmap stage; do not advance unrelated product work or infer deployment or trading approval.
---

# Gembo roadmap stage

Follow the roadmap through a bounded lifecycle:
`enter → implement → verify → awaiting approval → publish → handoff`.
A local implementation, owner approval, publication, and phase acceptance are distinct outcomes.

Resolve repository paths from its root. Read `AGENTS.md`, applicable nested instructions, `docs/delivery/README.md`, and `docs/delivery/05-roadmap-and-backlog.md`. Read the selected package's referenced plan documents and predecessor handoffs before implementation. The roadmap is authoritative for scope, dependency IDs, deliverables, and exit criteria; do not duplicate its backlog here.

## Enter or resume

1. Resolve the requested phase/increment/package. For a broad request, choose the smallest dependency-ready package and announce the selection. An explicit package request does not waive its dependencies. Check files, reports, Git history, existing PRs, and actual check results; do not infer completion from a previous assistant summary or a merged PR alone. If evidence is missing, prepare independent work and identify the missing gate without claiming acceptance.
2. Record the source commit, branch, remote/default branch, worktree status, requested outcome, allowed scope, prerequisites, acceptance checks, and any owner decisions. Inspect staged as well as unstaged and untracked changes. Preserve pre-existing work and keep a path/hunk ownership list. Never stage an unrelated dirty checkout wholesale.
3. Choose the existing task branch when appropriate. For new work, normally create `codex/roadmap-<id>-<short-purpose>` from the agreed base; respect explicit branch instructions and branch protections. Do not silently carry unrelated changes into the task branch or reset/stash them. Reuse suitable attached worktrees; isolate only when necessary and permitted. Detached HEAD must be resolved before publication.
4. On resume, locate the latest handoff, approval, branch, and PR. Continue the pending state: an approval of the current review packet proceeds to publication, not reimplementation. An approval without an identifiable matching artifact set requires clarification.

State the selected scope, why it is eligible, and what evidence will permit exit. Do not ask the user to reconfirm already authorized implementation.

## Implement and verify

Implement only the selected package or agreed bounded slice. Reuse existing assets and honor the CLI-first acceptance path. Keep mocked, planned, unverified, implemented, and tested behavior distinct. Do not start the next package automatically.

Run checks appropriate to the changed boundaries and all checks required by repository instructions before committing. Add behavioral tests for code changes. Documentation/skill-only changes need link, schema, command, or skill validation as applicable. Record exact commands, revision, outcomes, and skip/not-run reasons. Missing tools, missing services, unexpected skips, or failed checks are evidence gaps, never passes. Fix in-scope failures; otherwise report the blocked gate and request only the missing decision or explicit exception when needed. Do not manufacture acceptance to finish a stage.

Read-only preparation is allowed for later owner decisions; provisioning, paid runs, deployment, private-account access, scheduled agents, and exchange execution retain their separate authorization gates. Artifact approval below does not authorize any of them.

## Prepare a concrete review packet

If the stage creates or changes artifacts, present them to the owner **before any commit or push**. Artifacts include code, documentation, configuration, skills, tests, and generated evidence. Creating this workflow skill follows the same rule. A request to implement, a green check, elapsed time, agent review, or a vague earlier approval is not approval of a new artifact set.

Use a concise handoff at `docs/reports/<id>-<short-purpose>.md` when the package produces durable work; reuse an existing handoff on resume. Link substantial outputs instead of duplicating them. Include:

- Package ID and scope, source commit, branch/base, and pre-existing changes excluded.
- Deliverables with clickable file links, acceptance criteria mapped to evidence, and actual check results.
- Remaining failures/decisions, operational implications, and unmet phase gates.
- Exact files/hunks proposed for commit, proposed commit message, push destination, and PR base/title if a feature branch.
- Current state: `awaiting approval`, `blocked`, or `no changes`; never mark the phase accepted prematurely.

Ensure every generated artifact has an explicit disposition: included for commit, private evidence linked from its permitted store, or temporary output excluded with a reason. Do not commit secrets, private payloads, caches, or transient build output.

Present the completed review packet and ask once for approval to **commit and push this artifact set, and create/update its PR when on a feature branch**. Explain that this approval checkpoint is required by this skill's owner-review rule. End the turn awaiting the response; an asynchronous question or timeout is not approval. If no artifacts changed, report the findings and proposed next package without making an empty commit.

## Publish after approval

Owner approval of the identified current review packet authorizes its commit, push, and feature-branch PR under this workflow. Do not ask again for the same publication steps. Explicit narrower approval takes precedence. If approval includes requested edits, implement them and return the revised artifacts for approval unless the owner explicitly authorized publishing those exact edits without another review.

Before publishing, reread the approved scope and inspect current Git state/diff. Confirm the approved content and relevant check results still match; rerun affected checks if anything changed. Material artifact, branch/base, or scope changes need refreshed approval. Do not silently add artifacts after approval, including status edits to the reviewed report.

1. Stage only approved files/hunks. Inspect the complete staged diff, including previously staged files. If unrelated staged work cannot be safely separated, stop and explain; never commit someone else's changes.
2. Commit with the approved concise message. Verify the commit includes exactly the approved content. Preserve any other working-tree changes.
3. Push the approved commit to its intended remote branch with upstream tracking where needed. Use a normal push; do not force-push, rewrite published history, bypass hooks/protections, or push unapproved local commits. Inspect the outgoing commit range first. For protected/default-branch rejection or unexpected divergence, preserve work and report the blocker; changing the delivery route requires approval when it changes the reviewed destination.
4. On a feature branch, find an existing PR for that exact branch/base and update it as necessary, or create a focused PR with outcome, scope, validation, limitations, and the handoff link. Prefer the GitHub connector when sufficient; otherwise use authenticated `gh`, with multiline bodies passed through `--body-file`. Do not create duplicate PRs or merge a PR as part of this skill. Attach every created PR to the chat with the app's `attach_artifact` tool when available; also attach an existing PR being continued.
5. On the repository's actual default branch, no PR is required. Never assume its name is `main`. Verify the remote contains the approved commit and, when applicable, that the PR points at that branch/head. Observe required PR check results when available; pending checks remain pending evidence.

A failed push or PR creation is `publication incomplete`, not delivery complete. Report the exact successful steps, saved commit, remaining step, and actionable blocker. On retry inspect remote/PR state first; a timeout may have succeeded. Do not create duplicate commits or PRs.

## Exit and handoff

End with the selected package, outcome, artifact/report links, validation and limitations, approval status, commit SHA, pushed branch, and PR URL where applicable. Report excluded local changes briefly when relevant. Name the next dependency-ready package without starting it.

Use precise outcomes: `awaiting approval`, `blocked`, `no changes`, `publication incomplete`, or `published; review/acceptance pending`. Declare a package accepted only when its required evidence and review exist. A phase exits only when its roadmap exit criteria are demonstrated; an owner approving artifacts does not waive independent review, CI, deployment observation, or separate execution gates. Record any remaining acceptance work in the handoff/PR or final response without making unapproved follow-up commits.
