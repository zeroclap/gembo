variable "project_id" {
  description = <<-EOT
    Non-production GCP project dedicated to the CI egress runner. It must
    have billing enabled and hold no exchange or account credentials, and no
    production data. Created by the owner (see issue #4); not provisioned by
    this configuration.
  EOT
  type        = string
}

variable "region" {
  description = "Non-US region for the Cloud Run Job, so Bybit/Binance public endpoints are reachable."
  type        = string
  default     = "europe-west1"
}

variable "github_repository" {
  description = "The only repository allowed to impersonate the runner service account (owner/name)."
  type        = string
  default     = "zeroclap/gembo"
}

variable "github_repository_owner" {
  description = "GitHub org/user that owns github_repository. Used to scope the Workload Identity Federation condition."
  type        = string
  default     = "zeroclap"
}

variable "runner_image" {
  description = <<-EOT
    Container image for the ephemeral probe job, built from
    backend/deployments/Dockerfile (target: exchange-probe) and pushed to the
    Artifact Registry repository this configuration creates. Supply the full
    tag (e.g. REGION-docker.pkg.dev/PROJECT/ci-egress/exchange-probe:TAG)
    after the first image push; Cloud Run Jobs cannot be created without a
    resolvable image.
  EOT
  type        = string
}

variable "probe_args" {
  description = "Arguments passed to exchange-probe inside the Cloud Run Job."
  type        = list(string)
  default     = ["-exchanges", "bybit", "-symbols", "BTC/USDT:USDT,ETH/USDT:USDT", "-ws"]
}

variable "job_timeout_seconds" {
  description = "Hard timeout for a single probe job execution."
  type        = number
  default     = 300
}

variable "labels" {
  description = "Common resource labels."
  type        = map(string)
  default = {
    app     = "gembo"
    purpose = "ci-egress"
  }
}
