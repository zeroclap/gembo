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
2. The job runs under the `ci-egress` GitHub Environment, which the owner
   must configure (Settings → Environments) to restrict deployment to the
   `main` branch, so even a same-repo branch cannot trigger it outside of
   `main` and `workflow_dispatch`.
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
4. Set repository variables consumed by `.github/workflows/ci-egress.yml`:
   `CI_EGRESS_WORKLOAD_IDENTITY_PROVIDER`, `CI_EGRESS_SERVICE_ACCOUNT`,
   `CI_EGRESS_PROJECT_ID`, `CI_EGRESS_REGION`, `CI_EGRESS_JOB_NAME` — all
   produced as outputs of `terraform apply`.
5. Create the `ci-egress` GitHub Environment and restrict its deployment
   branch to `main`.

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
