terraform {
  required_providers {
    cloudflare = {
      source  = "cloudflare/cloudflare"
      version = "~> 5.0"
    }
  }
}

# Provider config は環境変数 CLOUDFLARE_API_TOKEN から取得
provider "cloudflare" {}
