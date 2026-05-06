output "instance_name" {
  value = google_compute_instance.vm.name
}

output "instance_zone" {
  value = google_compute_instance.vm.zone
}

output "service_account_email" {
  value = google_service_account.vm_sa.email
}

output "fqim_under_test" {
  value       = local.fqim
  description = "Fully-qualified image the verify.sh attempts to pull"
}

output "serial_log_command" {
  value       = "gcloud compute instances get-serial-port-output ${google_compute_instance.vm.name} --zone=${google_compute_instance.vm.zone} --project=${var.project_id}"
  description = "Run this to inspect verify.sh output captured on the serial console"
}

output "ssh_command" {
  value       = "gcloud compute ssh ${google_compute_instance.vm.name} --zone=${google_compute_instance.vm.zone} --project=${var.project_id}"
  description = "SSH into the VM to re-run /opt/metadata-test/verify.sh manually"
}
