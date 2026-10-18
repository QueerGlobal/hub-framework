# shellcheck shell=bash
# CI read-only Doppler service token scoped to hub-framework / infra only.
# Caller must source scripts/lib/doppler-configs.sh first.

export DOPPLER_CI_INFRA_SERVICE_TOKEN_NAME="${DOPPLER_CI_INFRA_SERVICE_TOKEN_NAME:-hub-framework-ci-infra-read}"

doppler_ci_infra_service_token_exists() {
  local project=$1
  local config=$2
  local name=$3
  doppler configs tokens list --project "${project}" --config "${config}" --json 2>/dev/null \
    | jq -e --arg n "${name}" '.[] | select(.name == $n)' >/dev/null 2>&1
}

# Creates a read-only service token for infra config. Plaintext is shown once on stdout.
ensure_doppler_ci_infra_service_token() {
  local project=$1
  local config=$2
  local name=${3:-${DOPPLER_CI_INFRA_SERVICE_TOKEN_NAME}}

  if doppler_ci_infra_service_token_exists "${project}" "${config}" "${name}"; then
    echo "Doppler CI service token ${name} already exists for ${project}/${config} (rotate in dashboard if needed)." >&2
    return 0
  fi

  echo "Creating Doppler CI service token ${name} (read-only, ${project}/${config})..." >&2
  doppler configs tokens create "${name}" \
    --project "${project}" \
    --config "${config}" \
    --access read \
    --plain
}

# Fails closed unless DOPPLER_TOKEN demonstrably works for infra and is denied on bootstrap-admin.
# A missing CLI, a broken token, or a network error must never read as "denied".
doppler_assert_ci_token_cannot_read_bootstrap_admin() {
  local project=${1:-${DOPPLER_PROJECT_NAME}}
  local infra_config=${2:-${DOPPLER_INFRA_CONFIG}}
  local bootstrap_config=${3:-${DOPPLER_BOOTSTRAP_ADMIN_CONFIG}}
  local name

  if [[ -z "${DOPPLER_TOKEN:-}" ]]; then
    echo "FAIL: DOPPLER_TOKEN is unset; bootstrap-admin isolation guard fails closed" >&2
    return 1
  fi

  if ! command -v doppler >/dev/null 2>&1; then
    echo "FAIL: doppler CLI is not installed; bootstrap-admin isolation cannot be proven" >&2
    return 1
  fi

  if ! doppler secrets --only-names -p "${project}" -c "${infra_config}" >/dev/null 2>&1; then
    echo "FAIL: DOPPLER_TOKEN cannot list ${project}/${infra_config}; a denial on bootstrap-admin would prove nothing" >&2
    return 1
  fi

  if doppler secrets --only-names -p "${project}" -c "${bootstrap_config}" >/dev/null 2>&1; then
    echo "FAIL: DOPPLER_TOKEN can list bootstrap-admin secrets" >&2
    echo "Use a service token scoped to ${project}/${infra_config} only." >&2
    return 1
  fi

  for name in TOFU_BOOTSTRAP_STATE_ENCRYPTION_PASSPHRASE GITHUB_TOFU_RELEASE_ENV TOFU_BOOTSTRAP_STATE_LOCAL_BACKUP; do
    if doppler secrets get "${name}" --plain -p "${project}" -c "${bootstrap_config}" >/dev/null 2>&1; then
      echo "FAIL: DOPPLER_TOKEN can read bootstrap-admin secret ${name}" >&2
      echo "Use a service token scoped to ${project}/${infra_config} only." >&2
      return 1
    fi
  done

  echo "OK: DOPPLER_TOKEN cannot read bootstrap-admin secrets"
}
