output "bucket_name" {
  description = "The name of the bucket for terraform state"
  value       = google_storage_bucket.tfstate_bucket.name
}
