variable "project_id" {
  description = "GCP Project ID"
  type        = string
}

variable "deploy_env" {
  description = "Deployment environment (ex: dev, staging, prod)"
  type        = string
}

variable "principals" {
  description = "Principals to grant roles/iam.serviceAccountTokenCreator to (ex: ['user:email@example.com'])"
  type        = list(string)
}
