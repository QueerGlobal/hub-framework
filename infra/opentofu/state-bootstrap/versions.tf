terraform {
  required_version = ">= 1.10.0"

  encryption {
    key_provider "pbkdf2" "bootstrap" {
      passphrase = var.bootstrap_state_encryption_passphrase_legacy
    }

    key_provider "pbkdf2" "bootstrap_primary" {
      passphrase = var.bootstrap_state_encryption_passphrase
    }

    method "aes_gcm" "bootstrap" {
      keys = key_provider.pbkdf2.bootstrap
    }

    method "aes_gcm" "bootstrap_primary" {
      keys = key_provider.pbkdf2.bootstrap_primary
    }

    state {
      method   = method.aes_gcm.bootstrap_primary
      enforced = true

      fallback {
        method = method.aes_gcm.bootstrap
      }
    }

    plan {
      method   = method.aes_gcm.bootstrap_primary
      enforced = true

      fallback {
        method = method.aes_gcm.bootstrap
      }
    }
  }

  required_providers {
    cloudflare = {
      source  = "cloudflare/cloudflare"
      version = "~> 4.40"
    }
  }
}
