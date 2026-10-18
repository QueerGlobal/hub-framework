mock_provider "cloudflare" {
  alias = "cloudflare"

  mock_resource "cloudflare_r2_bucket" {}

  mock_resource "cloudflare_account_token" {
    defaults = {
      id    = "mock-access-key-id"
      value = "mock-token-value"
    }
  }

  mock_data "cloudflare_account_api_token_permission_groups" {
    defaults = {
      permission_groups = [
        {
          id            = "mock-perm-group"
          name          = "Workers R2 Storage Bucket Item Write"
          category      = "Account"
          is_selectable = "true"
          scopes        = ["com.cloudflare.api.account"]
        },
      ]
    }
  }
}

run "plan_leak" {
  command = plan

  providers = {
    cloudflare = cloudflare.cloudflare
  }

  override_data {
    target = data.cloudflare_account_api_token_permission_groups.r2_bucket_item_write
    values = {
      permission_groups = [
        {
          id            = "mock-perm-group"
          name          = "Workers R2 Storage Bucket Item Write"
          category      = "Account"
          is_selectable = true
          scopes        = ["com.cloudflare.api.account"]
        },
      ]
    }
  }

  variables {
    cloudflare_account_id                 = "00000000000000000000000000000001"
    bootstrap_state_encryption_passphrase = "plan-leak-bootstrap-passphrase-xyzzy"
    state_bucket_name                     = "queerglobal-hub-framework-tofu-state"
  }
}
