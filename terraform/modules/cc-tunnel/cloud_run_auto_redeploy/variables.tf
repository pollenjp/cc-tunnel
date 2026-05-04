variable "project_id" {
  type = string
}

variable "name_prefix" {
  description = "Short prefix used for naming the deployer SA / workflow / trigger. Must be <= 17 chars to keep the SA account_id within 30 chars."
  type        = string

  validation {
    condition     = length(var.name_prefix) >= 1 && length(var.name_prefix) <= 17
    error_message = "name_prefix must be 1..17 characters."
  }
}

variable "cloud_run_name" {
  description = "Cloud Run service name to redeploy"
  type        = string
}

variable "cloud_run_location" {
  description = "Cloud Run service region (must match Eventarc trigger / Workflow region)"
  type        = string
}

variable "cloud_run_runtime_sa_id" {
  description = "Full ID of the Cloud Run runtime SA (projects/PROJECT/serviceAccounts/EMAIL). Granted iam.serviceAccountUser to the deployer SA."
  type        = string
}

variable "fqim" {
  description = "Fully qualified image name with tag, e.g. LOC-docker.pkg.dev/PROJECT/REPO/IMAGE:TAG"
  type        = string
}
