variable "project_id" {
  type = string
}

variable "deploy_env" {
  description = "Deployment environment name (used as a prefix for the OAuth client display name)"
  type        = string
}

variable "create_brand" {
  type        = bool
  description = "If true, manage the OAuth brand via terraform. Each project can have only one brand; set false and use existing_brand_name when one already exists."
  default     = false
}

variable "existing_brand_name" {
  type        = string
  description = "Existing IAP brand resource name (format: projects/<project_number>/brands/<brand_id>). Used when create_brand=false."
  default     = ""

  validation {
    condition     = var.existing_brand_name == "" || can(regex("^projects/[0-9]+/brands/[0-9]+$", var.existing_brand_name))
    error_message = "existing_brand_name must be of the form projects/<project_number>/brands/<brand_id>."
  }
}

variable "application_title" {
  type        = string
  description = "OAuth consent screen application title (used only when create_brand=true)"
  default     = "cc-tunnel"
}

variable "support_email" {
  type        = string
  description = "OAuth consent screen support email. Group address (e.g. team@example.com) recommended. Required when create_brand=true."
  default     = ""
}
