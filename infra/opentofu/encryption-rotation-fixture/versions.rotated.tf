terraform {
  required_version = ">= 1.10.0"

  backend "local" {
    path = "terraform.tfstate"
  }

  encryption {
    key_provider "pbkdf2" "bootstrap" {
      passphrase = var.passphrase_legacy
    }

    key_provider "pbkdf2" "bootstrap_primary" {
      passphrase = var.passphrase_primary
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
}
