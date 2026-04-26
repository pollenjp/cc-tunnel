output "sa_account_email" {
  value       = google_service_account.terraform_sa.email
  description = "Service account email for Terraform execution"
}
