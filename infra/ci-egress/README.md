# CI egress (issue #4)

Non-US egress for CI. Bybit and Binance geo-block US IPs; GitHub-hosted runners are
US-only, and there is no sandbox or staging environment, so this is provisioned
directly for a dedicated **non-production** GCP project.

## Decision: Cloud Run Job, not a GCE just-in-time runner

The issue offered two options. This picks the Cloud Run Job:

| | Cloud Run Job (chosen) | GCE JIT runner |
|---|---|---|
| Idle cost | $0 — no container runs between jobs | A VM must boot and register before every job |
| "Gone after the job finishes" | True by construction: the execution ends, the container is torn down | Needs a self-delete step in the startup script, and a failure path if it doesn't run |
| Operational surface | One job resource, one container image | VM image, startup script, JIT registration token handling, teardown on failure |
| How it's invoked | `gcloud run jobs execute`, authenticated from a GitHub-hosted runner via Workload Identity Federation | A hosted runner still has to create and poll the VM the same way |

The tradeoff: the job is a plain container, not a registered GitHub Actions
runner. The workflow (`.github/workflows/ci-egress.yml`) runs on a
GitHub-hosted (US) runner, authenticates to GCP, tells Cloud Run to execute
the job in `europe-west1`, waits for it, and reads back its logs. The job
itself never touches GitHub Actions state and carries no exchange or account
credentials — only public internet egress.

## What this provisions

- An Artifact Registry Docker repository (`ci-egress`) for the `exchange-probe`
  image (`backend/deployments/Dockerfile`, target `exchange-probe`).
- A service account (`ci-egress-runner`) with only `roles/run.developer` and
  `roles/logging.viewer` in the project — it cannot touch anything else.
- A Workload Identity Federation pool and provider for GitHub's OIDC issuer,
  scoped by an attribute condition to `github_repository_owner`/
  `github_repository` (default `zeroclap`/`zeroclap/gembo`). No GCP key is
  ever stored as a GitHub secret.
- A Cloud Run Job (`ci-egress-probe`) that runs `exchange-probe
  -print-egress -exchanges bybit -symbols BTC/USDT:USDT,ETH/USDT:USDT -ws`.
  `-print-egress` makes the probe fetch its public egress country and exit
  non-zero if it is `US`, so the job's own exit code — not just its log
  output — proves non-US egress on every run.

## Fork-PR protection

Two independent controls, either of which would be sufficient on its own:

1. The workflow requests `permissions: id-token: write`. GitHub refuses to
   mint an OIDC token for that permission on a fork-triggered `pull_request`
   run (this repo does not use `pull_request_target`), so the auth step
   fails closed for any fork PR.
2. The job's `if:` gates on `github.ref == 'refs/heads/main'` directly, so
   PR runs (same-repo or fork) skip cleanly rather than ever attempting to
   enter the environment. The `ci-egress` GitHub Environment itself (owner-
   configured, Settings → Environments, deployment branch policy restricted
   to `main`) is a second, independent layer enforcing the same boundary
   server-side — belt and suspenders, not reliance on the `if:` alone.
3. The Workload Identity Federation provider's `attribute_condition` only
   accepts tokens whose `repository_owner`/`repository` claims match this
   repository, regardless of what else might later be federated into the
   same project.

## Cost estimate

Target: near zero when idle, which a Cloud Run Job gets by default (billed
per vCPU-second/memory-second of execution only, nothing between runs).

- Cloud Run Job: 1 vCPU, 512 MiB, timeout up to 300s. A run that completes in
  under a minute (it fetches markets/funding/one WS ticker for two symbols)
  costs a small fraction of a cent. At, say, 50 runs/month (every PR touching
  exchange/collector code, plus a daily schedule once #7 lands), this is well
  under $1/month.
- Artifact Registry: one small (<50 MB) image, a few cents/month for storage.
- Workload Identity Federation and IAM: no cost.
- `google_project_service` enablement: no cost by itself.

## Owner actions (not done by this configuration)

This configuration is **written, not applied**. Agents plan; the owner
applies (`ROADMAP.md`, operating constraints). Specifically:

1. Create the non-production GCP project with billing enabled (recorded as
   approved in principle in issue #1's checklist; the project itself does
   not exist yet as far as this repository can tell).
2. Build and push the `exchange-probe` image once
   (`make docker-build-ci-egress-probe` in `backend/`, then push it to the
   Artifact Registry repository this configuration creates) and pass its
   full tag as `runner_image`.
3. Run `terraform init && terraform plan` with real credentials
   (`gcloud auth application-default login` or a CI principal) and `-var
   project_id=... -var runner_image=...`, review the plan, then `terraform
   apply`. This agent has no GCP credentials in its environment and cannot
   produce real plan output; `terraform validate` passes, which is as far as
   it can verify this configuration without them.
4. Create the `ci-egress` GitHub Environment and restrict its deployment
   branch to `main`.
5. Set the identifiers produced by `terraform apply` as variables scoped to
   the `ci-egress` **environment** (not repo-wide) — they aren't secrets,
   but scoping them to the environment keeps them behind its branch
   protection anyway: `CI_EGRESS_WORKLOAD_IDENTITY_PROVIDER`,
   `CI_EGRESS_SERVICE_ACCOUNT`, `CI_EGRESS_PROJECT_ID`, `CI_EGRESS_REGION`,
   `CI_EGRESS_JOB_NAME`. Separately set one plain **repository** variable,
   `CI_EGRESS_READY=true`, used only so the workflow can skip cleanly
   before setup is finished (a job's `if:` can't see environment-scoped
   vars — see the comment in `ci-egress.yml`).

Exact commands for all five steps are in "Step-by-step runbook" below.

## Step-by-step runbook

Run this from `infra/ci-egress/` unless noted. Replace `PROJECT_ID` and
`BILLING_ACCOUNT_ID` with real values throughout.

### 1. Create the project and enable billing

```bash
gcloud projects create PROJECT_ID --name="gembo-ci-egress"
gcloud billing projects link PROJECT_ID --billing-account=BILLING_ACCOUNT_ID
gcloud config set project PROJECT_ID
```

### 2. First (targeted) apply: just the Artifact Registry repo

The Cloud Run Job needs `runner_image` to already exist in the registry, and
the registry is created by this same configuration, so bootstrap it in two
passes. `runner_image` isn't used by these two resources, so any string
satisfies the variable:

```bash
terraform init
terraform apply \
  -target=google_project_service.artifactregistry \
  -target=google_artifact_registry_repository.ci_egress \
  -var="project_id=PROJECT_ID" \
  -var="runner_image=not-yet-pushed"
```

### 3. Build and push the probe image

```bash
cd ../../backend
make docker-build-ci-egress-probe
gcloud auth configure-docker europe-west1-docker.pkg.dev
IMAGE="europe-west1-docker.pkg.dev/PROJECT_ID/ci-egress/exchange-probe:$(git rev-parse --short HEAD)"
docker tag gembo-exchange-probe:latest "$IMAGE"
docker push "$IMAGE"
cd ../infra/ci-egress
```

### 4. Full plan and apply

```bash
terraform plan -var="project_id=PROJECT_ID" -var="runner_image=$IMAGE" -out=ci-egress.tfplan
# review the plan, then:
terraform apply ci-egress.tfplan
```

Attach the `terraform plan` output to the PR per issue #4's acceptance
criteria.

### 5. Create the protected `ci-egress` environment

`gh` has no dedicated environment command; use the API. This repo is public,
so deployment branch policies are free (no GitHub Team/Enterprise needed).
Create this **before** step 6 — environment-scoped variables require the
environment to already exist:

```bash
gh api --method PUT repos/zeroclap/gembo/environments/ci-egress \
  --input - <<'JSON'
{
  "deployment_branch_policy": { "protected_branches": true, "custom_branch_policies": false }
}
JSON
```

That restricts the environment to branches with branch protection — `main`
only, if `main` is the sole protected branch. Confirm in Settings →
Environments → ci-egress in the GitHub UI.

### 6. Wire the outputs into GitHub

The five identifiers below aren't secrets, but they're scoped to the
`ci-egress` **environment** (`--env ci-egress`) rather than the whole repo,
so they stay behind that environment's branch protection as a defense-in-
depth layer — a workflow run from an arbitrary branch can declare
`environment: ci-egress` all it wants, but GitHub won't dispatch the job
into that environment (and thus won't resolve these vars) unless the
branch satisfies the policy from step 5:

```bash
gh variable set CI_EGRESS_WORKLOAD_IDENTITY_PROVIDER --env ci-egress \
  --body "$(terraform output -raw workload_identity_provider)" --repo zeroclap/gembo
gh variable set CI_EGRESS_SERVICE_ACCOUNT --env ci-egress \
  --body "$(terraform output -raw service_account_email)" --repo zeroclap/gembo
gh variable set CI_EGRESS_PROJECT_ID --env ci-egress --body "PROJECT_ID" --repo zeroclap/gembo
gh variable set CI_EGRESS_REGION --env ci-egress \
  --body "$(terraform output -raw region)" --repo zeroclap/gembo
gh variable set CI_EGRESS_JOB_NAME --env ci-egress \
  --body "$(terraform output -raw cloud_run_job_name)" --repo zeroclap/gembo
```

A job's `if:` condition can't see environment-scoped vars (they only
resolve for steps, after environment protection rules pass), so the
workflow's skip-until-configured guard needs a separate, plain **repo**
variable instead — this one is fine at repo scope since it carries no
information beyond "setup is done":

```bash
gh variable set CI_EGRESS_READY --body true --repo zeroclap/gembo
```

### 7. Verify end to end

```bash
gh workflow run ci-egress.yml --repo zeroclap/gembo
gh run watch --repo zeroclap/gembo
```

Confirm the "Print job logs" step shows a non-US `egress_country=` line and
`[bybit]` check output, and that a second run completes with no leftover
Cloud Run execution (`gcloud run jobs executions list --job=ci-egress-probe
--region=europe-west1` should only ever show finished executions, never a
running one outside an active workflow run).

## Acceptance criteria from issue #4

- [ ] `terraform plan` output attached to the PR — blocked on the owner
      actions above; `terraform validate` passes today.
- [x] A workflow job on the runner prints the egress country (not US) and
      passes `exchange-probe -exchanges bybit -symbols
      BTC/USDT:USDT,ETH/USDT:USDT -ws` — implemented
      (`.github/workflows/ci-egress.yml`, `-print-egress` in
      `cmd/exchange-probe`). Not yet run for real; needs the project.
- [x] A fork PR cannot schedule a job on it — enforced by the `id-token:
      write` + non-`pull_request_target` control, the `ci-egress` environment,
      and the WIF attribute condition; see "Fork-PR protection" above.
- [x] Runner VM/job is gone after the job finishes — true by construction
      for a Cloud Run Job; no teardown step needed.
