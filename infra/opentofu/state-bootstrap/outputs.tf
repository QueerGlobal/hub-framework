output "state_bucket_name" {
  description = "R2 bucket used for release-environment remote state."
  value       = cloudflare_r2_bucket.tofu_state.name
}

output "r2_s3_endpoint" {
  description = "S3-compatible endpoint for the state bucket."
  value       = local.r2_s3_endpoint
}

output "tofu_state_access_key_id" {
  description = "Access key ID for the scoped R2 token (Cloudflare account token id)."
  value       = cloudflare_account_token.tofu_state_s3.id
}

output "tofu_state_secret_access_key" {
  description = "Secret access key for the scoped R2 token."
  value       = sha256(cloudflare_account_token.tofu_state_s3.value)
  sensitive   = true
}
