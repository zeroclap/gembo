# Non-US egress for CI (issue #4).
#
# Decision: a Cloud Run Job, not a GCE just-in-time runner. A Cloud Run Job
# scales to zero between runs (no VM to boot, register, and tear down), bills
# only for the seconds a job actually executes, and a finished execution
# leaves nothing running — the acceptance criterion "runner is gone after the
# job finishes" holds by construction instead of needing a self-delete step.
# The tradeoff is that the job is a plain container, not a registered GitHub
# Actions runner: the workflow authenticates to GCP from a GitHub-hosted
# runner via Workload Identity Federation (no long-lived key) and tells Cloud
# Run to execute the job, then reads back its logs. The job itself carries no
# exchange or account credentials and no production access — only public
# internet egress from a non-US region.

locals {
  # Workload Identity Federation subject/condition inputs. GitHub's OIDC
  # token sets `repository` to the base repository for both `push` and
  # `pull_request` events; fork PRs cannot obtain a token here at all because
  # GitHub refuses to mint one for a job that requests `id-token: write`
  # under a fork-triggered `pull_request` run (not `pull_request_target`).
  # The attribute condition below is defense in depth on top of that.
  wif_attribute_condition = "assertion.repository_owner == \"${var.github_repository_owner}\" && assertion.repository == \"${var.github_repository}\""
}

resource "google_project_service" "run" {
  project            = var.project_id
  service            = "run.googleapis.com"
  disable_on_destroy = false
}

resource "google_project_service" "iamcredentials" {
  project            = var.project_id
  service            = "iamcredentials.googleapis.com"
  disable_on_destroy = false
}

resource "google_project_service" "artifactregistry" {
  project            = var.project_id
  service            = "artifactregistry.googleapis.com"
  disable_on_destroy = false
}

# Holds the exchange-probe image used by the Cloud Run Job. Nothing else is
# pushed here.
resource "google_artifact_registry_repository" "ci_egress" {
  project       = var.project_id
  location      = var.region
  repository_id = "ci-egress"
  format        = "DOCKER"
  description   = "exchange-probe image for the non-US egress CI job (issue #4)."
  labels        = var.labels

  depends_on = [google_project_service.artifactregistry]
}

# Identity the Cloud Run Job executes as. It never leaves the project: no
# exchange keys, no production IAM roles.
resource "google_service_account" "runner" {
  project      = var.project_id
  account_id   = "ci-egress-runner"
  display_name = "CI egress probe runner (issue #4)"
}

resource "google_iam_workload_identity_pool" "github" {
  project                   = var.project_id
  workload_identity_pool_id = "github-actions"
  display_name              = "GitHub Actions"
  description               = "Federates GitHub Actions OIDC tokens for the non-US egress CI job. No long-lived keys."

  depends_on = [google_project_service.iamcredentials]
}

resource "google_iam_workload_identity_pool_provider" "github" {
  project                            = var.project_id
  workload_identity_pool_id          = google_iam_workload_identity_pool.github.workload_identity_pool_id
  workload_identity_pool_provider_id = "github"
  display_name                       = "GitHub"

  attribute_mapping = {
    "google.subject"       = "assertion.sub"
    "attribute.repository" = "assertion.repository"
    "attribute.ref"        = "assertion.ref"
  }

  # Restricts token exchange to this repository, regardless of what other
  # repositories might later be added to the same GCP project.
  attribute_condition = local.wif_attribute_condition

  oidc {
    issuer_uri = "https://token.actions.githubusercontent.com"
  }
}

# Lets only this repository's workflow runs impersonate the runner service
# account — no GitHub Actions secret ever holds a GCP key.
resource "google_service_account_iam_member" "wif_binding" {
  service_account_id = google_service_account.runner.name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principalSet://iam.googleapis.com/${google_iam_workload_identity_pool.github.name}/attribute.repository/${var.github_repository}"
}

# Only what's needed to kick off and read back one job execution.
resource "google_project_iam_member" "runner_run_developer" {
  project = var.project_id
  role    = "roles/run.developer"
  member  = "serviceAccount:${google_service_account.runner.email}"
}

resource "google_project_iam_member" "runner_logs_viewer" {
  project = var.project_id
  role    = "roles/logging.viewer"
  member  = "serviceAccount:${google_service_account.runner.email}"
}

resource "google_cloud_run_v2_job" "ci_egress_probe" {
  project  = var.project_id
  name     = "ci-egress-probe"
  location = var.region
  labels   = var.labels

  # This job is disposable CI infra recreated on every image/config change
  # (see README: "gone after job finishes"); it holds no state worth
  # protecting, so don't block replacement on accidental-deletion guards.
  deletion_protection = false

  template {
    template {
      service_account = google_service_account.runner.email
      timeout         = "${var.job_timeout_seconds}s"
      max_retries     = 0

      containers {
        image = var.runner_image
        args  = var.probe_args

        resources {
          limits = {
            cpu    = "1"
            memory = "512Mi"
          }
        }
      }
    }
  }

  depends_on = [google_project_service.run]
}
