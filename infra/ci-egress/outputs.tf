output "workload_identity_provider" {
  description = "Full resource name for google-github-actions/auth's workload_identity_provider input."
  value       = google_iam_workload_identity_pool_provider.github.name
}

output "service_account_email" {
  description = "Service account the workflow impersonates to invoke the Cloud Run Job."
  value       = google_service_account.runner.email
}

output "cloud_run_job_name" {
  description = "Cloud Run Job name the workflow executes."
  value       = google_cloud_run_v2_job.ci_egress_probe.name
}

output "region" {
  value = var.region
}

output "artifact_registry_repository" {
  description = "Push the exchange-probe image here before the first apply (runner_image must resolve)."
  value       = google_artifact_registry_repository.ci_egress.name
}
