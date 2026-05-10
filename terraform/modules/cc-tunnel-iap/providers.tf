# external provider は root.hcl の autogen required_providers に含まれないため
# 本モジュールで明示する。data "external" "iap_brand" で gcloud を起動して
# Console 作成済みの IAP OAuth brand を verify するために使う。
terraform {
  required_providers {
    external = {
      source  = "hashicorp/external"
      version = "~> 2.3"
    }
  }
}
