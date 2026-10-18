# shellcheck shell=bash
# E2E local backend override for release-environment (never rewrite tracked backend_s3.tf).

release_backend_s3_tf() {
  local dir=${1:?}
  echo "${dir}/backend_s3.tf"
}

release_backend_s3_hidden() {
  local dir=${1:?}
  echo "${dir}/.release_backend_s3.tf"
}

release_backend_local_override_tf() {
  local dir=${1:?}
  echo "${dir}/backend_override.tf"
}

release_backend_enable_local() {
  local dir=$1
  local s3_tf hidden override
  s3_tf="$(release_backend_s3_tf "${dir}")"
  hidden="$(release_backend_s3_hidden "${dir}")"
  override="$(release_backend_local_override_tf "${dir}")"

  if [[ -f "${s3_tf}" && ! -f "${hidden}" ]]; then
    mv "${s3_tf}" "${hidden}"
  fi

  cat > "${override}" <<'EOF'
terraform {
  backend "local" {
    path = "terraform.tfstate"
  }
}
EOF
}

release_backend_disable_local() {
  local dir=$1
  local s3_tf hidden override
  s3_tf="$(release_backend_s3_tf "${dir}")"
  hidden="$(release_backend_s3_hidden "${dir}")"
  override="$(release_backend_local_override_tf "${dir}")"

  rm -f "${override}"
  if [[ -f "${hidden}" && ! -f "${s3_tf}" ]]; then
    mv "${hidden}" "${s3_tf}"
  fi
}
