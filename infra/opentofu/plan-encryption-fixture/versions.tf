terraform {
  required_version = ">= 1.10.0"

  backend "local" {
    path = "terraform.tfstate"
  }

  encryption {
    key_provider "pbkdf2" "bootstrap" {
      passphrase = var.bootstrap_state_encryption_passphrase
    }

    method "aes_gcm" "bootstrap" {
      keys = key_provider.pbkdf2.bootstrap
    }

    state {
      method = method.aes_gcm.bootstrap
    }

    plan {
      method = method.aes_gcm.bootstrap
    }
  }
}
