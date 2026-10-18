mock_provider "cloudflare" {
  alias = "cloudflare"

  mock_resource "cloudflare_r2_bucket" {}

  mock_resource "cloudflare_api_token" {
    defaults = {
      id    = "mock-access-key-id"
      value = "mock-token-value"
    }
  }

  mock_data "cloudflare_api_token_permission_groups" {
    defaults = {
      account = {
        "Workers R2 Storage Bucket Item Write" = "mock-perm-group"
      }
    }
  }
}

run "plan_leak" {
  command = plan

  providers = {
    cloudflare = cloudflare.cloudflare
  }

  variables {
    cloudflare_account_id                 = "00000000000000000000000000000001"
    bootstrap_state_encryption_passphrase = "plan-leak-bootstrap-passphrase-xyzzy"
    state_bucket_name                     = "queerglobal-hub-framework-tofu-state"
  }
}
