include "root" {
  path   = find_in_parent_folders("root.hcl")
  expose = true
}

dependency "init" {
  config_path = "./../init"
}

terraform {
  source = "./../../../modules//vm_image_cleaner"
}

inputs = {
  project_id = "${include.root.locals.gcp_project_id}"
  region     = "${include.root.locals.provider_default_region}"

  # Packer が焼く GCE custom image の family。modules/cc-tunnel/vm_image.tf の
  # local.vm_image_family と一致させる。
  image_family = "cc-tunnel-vm"

  # 最新 2 件だけ残してそれより古いものを削除する。
  keep_count = 2

  # 毎日 1 回 (Asia/Tokyo の 03:00) に実行。
  schedule  = "0 3 * * *"
  time_zone = "Asia/Tokyo"
}
