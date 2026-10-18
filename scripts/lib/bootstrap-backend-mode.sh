# shellcheck shell=bash
# Optional S3 backend via gitignored backend_override.tf (never rewrite tracked versions.tf).

bootstrap_backend_local_tf() {
  local dir=${1:?}
  echo "${dir}/backend_local.tf"
}

bootstrap_backend_override_tf() {
  local dir=${1:?}
  echo "${dir}/backend_override.tf"
}

bootstrap_backend_local_hidden() {
  local dir=${1:?}
  echo "${dir}/.bootstrap_backend_local.tf"
}

bootstrap_backend_enable_s3() {
  local dir=$1
  local local_tf hidden override
  local_tf="$(bootstrap_backend_local_tf "${dir}")"
  hidden="$(bootstrap_backend_local_hidden "${dir}")"
  override="$(bootstrap_backend_override_tf "${dir}")"

  if [[ -f "${local_tf}" && ! -f "${hidden}" ]]; then
    mv "${local_tf}" "${hidden}"
  fi

  cat > "${override}" <<'EOF'
terraform {
  backend "s3" {}
}
EOF
}

bootstrap_backend_disable_s3() {
  local dir=$1
  local local_tf hidden override
  local_tf="$(bootstrap_backend_local_tf "${dir}")"
  hidden="$(bootstrap_backend_local_hidden "${dir}")"
  override="$(bootstrap_backend_override_tf "${dir}")"

  rm -f "${override}"
  if [[ -f "${hidden}" && ! -f "${local_tf}" ]]; then
    mv "${hidden}" "${local_tf}"
  fi
}

bootstrap_backup_local_state_to_doppler() {
  local state_file=$1
  local project=$2
  local config=$3
  if [[ ! -f "${state_file}" ]]; then
    echo "No local bootstrap state at ${state_file}; skip backup." >&2
    return 1
  fi
  doppler secrets set TOFU_BOOTSTRAP_STATE_LOCAL_BACKUP \
    --project "${project}" --config "${config}" \
    --silent --no-interactive <"${state_file}"
}
