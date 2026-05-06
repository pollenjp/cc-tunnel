variable "project_id" {
  type = string
}

variable "region" {
  type        = string
  description = "Region for Cloud Run Job and Cloud Scheduler"
}

variable "image_family" {
  type        = string
  description = "GCE image family to clean up (e.g. 'cc-tunnel-vm')"
}

variable "keep_count" {
  type        = number
  description = "Number of latest images to retain in the family"
  default     = 2
}

variable "schedule" {
  type        = string
  description = "Cron schedule for cleanup. Default: daily at 03:00"
  default     = "0 3 * * *"
}

variable "time_zone" {
  type        = string
  description = "TZ for the schedule (IANA tz name)"
  default     = "Etc/UTC"
}

variable "name_prefix" {
  type        = string
  description = "Prefix used for resource names (SA, Job, Scheduler)"
  default     = "vm-image-cleaner"
}
