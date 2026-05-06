variable "project_id" {
  type = string
}

variable "deploy_env" {
  type = string
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
