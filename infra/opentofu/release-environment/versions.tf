terraform {
  required_version = ">= 1.10.0"

  backend "s3" {}

  encryption {
    key_provider "pbkdf2" "state" {
      passphrase = var.state_encryption_passphrase_legacy
    }

    key_provider "pbkdf2" "state_primary" {
      passphrase = var.state_encryption_passphrase
    }

    method "aes_gcm" "state" {
      keys = key_provider.pbkdf2.state
    }

    method "aes_gcm" "state_primary" {
      keys = key_provider.pbkdf2.state_primary
    }

    state {
      method   = method.aes_gcm.state_primary
      enforced = true

      fallback {
        method = method.aes_gcm.state
      }
    }

    plan {
      method   = method.aes_gcm.state_primary
      enforced = true

      fallback {
        method = method.aes_gcm.state
      }
    }
  }

  required_providers {
    github = {
      source  = "integrations/github"
      version = "~> 6.13"
    }
  }
}
