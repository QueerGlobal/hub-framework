provider "cloudflare" {}

data "cloudflare_api_token_permission_groups" "all" {}

locals {
  r2_bucket_item_write_id = data.cloudflare_api_token_permission_groups.all.account["Workers R2 Storage Bucket Item Write"]
  bucket_resource         = "com.cloudflare.edge.r2.bucket.${var.cloudflare_account_id}_default_${var.state_bucket_name}"
  r2_s3_endpoint          = "https://${var.cloudflare_account_id}.r2.cloudflarestorage.com"
}

resource "cloudflare_r2_bucket" "tofu_state" {
  account_id = var.cloudflare_account_id
  name       = var.state_bucket_name
  location   = "WNAM"
}

resource "cloudflare_api_token" "tofu_state_s3" {
  name = var.r2_api_token_name

  policy {
    permission_groups = [local.r2_bucket_item_write_id]
    resources = {
      (local.bucket_resource) = "*"
    }
  }

  depends_on = [cloudflare_r2_bucket.tofu_state]
}
