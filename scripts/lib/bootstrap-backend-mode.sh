# shellcheck shell=bash
# Switches state-bootstrap versions.tf between local and S3 backends for bootstrap/migration.

bootstrap_versions_tf="${1:?bootstrap stack directory}/versions.tf"

bootstrap_versions_use_local() {
  cat > "${bootstrap_versions_tf}" <<'EOF'
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

  required_providers {
    cloudflare = {
      source  = "cloudflare/cloudflare"
      version = "~> 4.40"
    }
  }
}
EOF
}

bootstrap_versions_use_s3() {
  cat > "${bootstrap_versions_tf}" <<'EOF'
terraform {
  required_version = ">= 1.10.0"

  backend "s3" {}

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

  required_providers {
    cloudflare = {
      source  = "cloudflare/cloudflare"
      version = "~> 4.40"
    }
  }
}
EOF
}

bootstrap_versions_is_s3() {
  grep -q 'backend "s3"' "${bootstrap_versions_tf}"
}
