# Manual verification contract

D03 introduces one versioned, evidence-only verification role. It does not
activate automation. See the [role](roles/verification.md),
[policy](policies/verification.json), and [review packet](../docs/reports/d03-verification-contract.md).
The existing [D02 baseline runner](../docs/verification.md) remains the way to
execute application checks.

## Reproduce the contract rehearsal

From the repository root, with Python 3.11+ (required by the pinned `rpds-py`):

```bash
python3 -m venv .artifacts/d03-tools
.artifacts/d03-tools/bin/python -m pip install -r tools/verification/contract-requirements.txt
make test-verification PYTHON=.artifacts/d03-tools/bin/python
make test-agent-contract PYTHON=.artifacts/d03-tools/bin/python
.artifacts/d03-tools/bin/python scripts/verification/contract.py validate --task docs/reports/d03-verification-task.json --result docs/reports/d03-verification-result.json
```

Dependency installation is a separate, operator-controlled step; the checker
does not install tools or use the network. The pinned validator and transitive
dependencies are separate from the unchanged D02 tooling image. Only macOS
arm64/Python 3.14 has been exercised for D03; other platforms require evidence.

To rehearse against a new clean selected revision, create a new local task and
result (existing destinations are rejected):

```bash
.artifacts/d03-tools/bin/python scripts/verification/contract.py prepare --task .artifacts/d03-manual/task.json --output .artifacts/d03-manual/result.json
.artifacts/d03-tools/bin/python scripts/verification/contract.py review --task .artifacts/d03-manual/task.json
.artifacts/d03-tools/bin/python scripts/verification/contract.py validate --task .artifacts/d03-manual/task.json --result .artifacts/d03-manual/result.json
```

`prepare` is an operator convenience for a **contract rehearsal**, not admission
or authorization. It captures Git HEAD, trusted-file/input hashes, fixed
read/output scope, limits, and a 30-minute deadline. `review` requires that HEAD
and all hashes still match, then imports all 21 D02 checks and their original
identities. It never runs their command arrays. Paths resolve relative to the
repository; symlinks, traversal, private locations, and overwrite are rejected.

`validate` checks schemas, hashes, task/result identity, scope, counts, original
commands/statuses/skips, evidence verdict, and timestamps. It works on archived
results after their deadline and after HEAD moves, provided the bound bundle
and input files remain identical. This distinguishes historical validation from
starting a new run. Restoring the recorded revision restores the bundle.
It does not execute candidate scripts or attest to the truth of imported logs.

Exit 0 means a valid contract/result, even for red imported evidence; exit 1
means evidence/policy/IO validation failed and no successful result was written;
exit 2 is invalid command-line usage. These are engineering-tool exits, separate
from the planned product CLI protocol. Interruptions propagate a nonzero process
exit. No automatic retry, subprocess service, or cleanup of shared resources occurs.

## Evidence and limits

The [sample task](../docs/reports/d03-verification-task.json) and
[result](../docs/reports/d03-verification-result.json) are a manually invoked
rehearsal over committed, sanitized D02 evidence. Run success means the review
completed; `evidence_verdict: gaps` and `release_eligible: false` retain the red
baseline. The report links each imported command to its original run/revision
and snapshot, rather than presenting an old result as a new application test.

The checker supplies measured wall time, Git identity, input hashes, and output
size limits. Model tokens/cost and conversational tool-call usage are unavailable
in this manual session and remain null with a reason. There is no new model/API
invocation. The operator supervises attempt count and conversational limits; D10 must
meter/enforce them in a disposable worker. This v1 grants zero network requests
and no Gembo operations; CLI artifact/profile/token fields must be added through
a reviewed schema version when C02/D10 implement the tool boundary.

Validation is not an authenticated approval record or trusted CI attestation.
The proposed D10 dispatcher/broker must pin the trusted bundle separately from
candidate content and independently verify results. Policy changes are R2 and
need owner and independent review; this role cannot approve its own revision.
