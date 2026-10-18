output "state_bucket_name" {
  description = "R2 bucket used for OpenTofu remote state."
  value       = cloudflare_r2_bucket.tofu_state.name
}

output "r2_s3_endpoint" {
  description = "S3-compatible endpoint for the R2 bucket."
  value       = local.r2_s3_endpoint
}

output "tofu_state_access_key_id" {
  description = "S3 access key ID (Cloudflare API token id) for state access."
  value       = cloudflare_api_token.tofu_state_s3.id
}

output "tofu_state_secret_access_key" {
  description = "S3 secret access key (SHA-256 of the API token value)."
  value       = sha256(cloudflare_api_token.tofu_state_s3.value)
  sensitive   = true
}
