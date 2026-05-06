variable "project_id" {
  type = string
}

variable "deploy_env" {
  type = string
}

variable "terraform_runner_sa_email" {
  type        = string
  description = "Runner SA to impersonate when invoking gcloud from local-exec"
}

variable "github_owner" {
  type        = string
  description = "GitHub owner/org for the Cloud Build trigger source"
  default     = "pollenjp"
}

variable "github_repo_name" {
  type        = string
  description = "GitHub repository name for the Cloud Build trigger source"
  default     = "cc-tunnel"
}

variable "github_branch_name" {
  type        = string
  description = "GitHub branch the Cloud Build trigger watches AND the branch checked out for the one-shot Packer build run from terraform_data. Set to the feature branch when applying before merge to main."
  default     = "main"
}

variable "artifact_registry_repository_location" {
  type = string
}

variable "artifact_registry_repository_name" {
  type = string
}

variable "zone" {
  type    = string
  default = "us-central1-a"
}

variable "machine_type" {
  type    = string
  default = "e2-small"
}

variable "image_family" {
  type        = string
  description = "Image family produced by apps/metadata-test-vm/packer.pkr.hcl"
  default     = "metadata-test-vm"
}

variable "network_name" {
  type    = string
  default = "default"
}

variable "subnetwork_name" {
  type        = string
  description = "Subnet to attach the VM to. null = auto subnet for default network."
  default     = null
}

variable "cc_remote_agent_image_name" {
  type    = string
  default = "cc-remote-agent"
}

variable "cc_remote_agent_image_tag" {
  type    = string
  default = "latest"
}
