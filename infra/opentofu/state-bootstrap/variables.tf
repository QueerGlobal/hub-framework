variable "cloudflare_account_id" {
  type        = string
  description = "Cloudflare account ID that owns the R2 bucket."
}

variable "state_bucket_name" {
  type        = string
  description = "Globally unique R2 bucket name for OpenTofu remote state."
  default     = "queerglobal-hub-framework-tofu-state"
}

variable "bootstrap_state_encryption_passphrase" {
  type        = string
  description = "Primary passphrase encrypting bootstrap state and plans (min. 16 characters)."
  sensitive   = true
}

variable "bootstrap_state_encryption_passphrase_legacy" {
  type        = string
  description = "Legacy passphrase under key_provider name bootstrap (fallback). Set equal to primary until rotation."
  sensitive   = true
}

variable "r2_api_token_name" {
  type        = string
  description = "Name for the scoped R2 S3 API token written to Doppler."
  default     = "hub-framework-tofu-state"
}
