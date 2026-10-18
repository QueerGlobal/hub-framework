provider "cloudflare" {}

data "cloudflare_account_api_token_permission_groups" "r2_bucket_item_write" {
  account_id = var.cloudflare_account_id
  name       = "Workers R2 Storage Bucket Item Write"
}

locals {
  r2_bucket_item_write_id = data.cloudflare_account_api_token_permission_groups.r2_bucket_item_write.permission_groups[0].id
  bucket_resource         = "com.cloudflare.edge.r2.bucket.${var.cloudflare_account_id}_default_${var.state_bucket_name}"
  r2_s3_endpoint          = "https://${var.cloudflare_account_id}.r2.cloudflarestorage.com"
}

resource "cloudflare_r2_bucket" "tofu_state" {
  account_id = var.cloudflare_account_id
  name       = var.state_bucket_name
  location   = "WNAM"
}

resource "cloudflare_account_token" "tofu_state_s3" {
  account_id = var.cloudflare_account_id
  name       = var.r2_api_token_name

  policies = [{
    effect = "allow"
    permission_groups = [{
      id = local.r2_bucket_item_write_id
    }]
    resources = jsonencode({
      (local.bucket_resource) = "*"
    })
  }]

  depends_on = [cloudflare_r2_bucket.tofu_state]
}
