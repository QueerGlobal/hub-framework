# shellcheck shell=bash
# Temporary S3 backend for state-bootstrap without leaving a dirty tracked versions.tf.

bootstrap_versions_tf_path() {
  local dir=${1:?}
  echo "${dir}/versions.tf"
}

bootstrap_versions_snapshot_path() {
  local dir=${1:?}
  echo "${dir}/.bootstrap-versions.tf.snapshot"
}

bootstrap_versions_snapshot() {
  local dir=$1
  local versions_tf snapshot
  versions_tf="$(bootstrap_versions_tf_path "${dir}")"
  snapshot="$(bootstrap_versions_snapshot_path "${dir}")"
  if [[ ! -f "${snapshot}" ]]; then
    cp "${versions_tf}" "${snapshot}"
  fi
}

bootstrap_versions_restore() {
  local dir=$1
  local versions_tf snapshot
  versions_tf="$(bootstrap_versions_tf_path "${dir}")"
  snapshot="$(bootstrap_versions_snapshot_path "${dir}")"
  if [[ -f "${snapshot}" ]]; then
    cp "${snapshot}" "${versions_tf}"
    rm -f "${snapshot}"
  fi
}

bootstrap_versions_with_s3_backend() {
  local dir=$1
  local versions_tf
  versions_tf="$(bootstrap_versions_tf_path "${dir}")"
  bootstrap_versions_snapshot "${dir}"
  cat > "${versions_tf}" <<'EOF'
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

bootstrap_backend_is_remote() {
  local dir=$1
  [[ -f "${dir}/.terraform/terraform.tfstate" ]] \
    && grep -q '"type"[[:space:]]*:[[:space:]]*"s3"' "${dir}/.terraform/terraform.tfstate"
}
