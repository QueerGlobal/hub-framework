# shellcheck shell=bash
# Sync required bootstrap inputs from environment into Doppler after project/config exist.
# Absent and empty secrets count as missing; a Doppler error stops the run instead.

_BOOTSTRAP_SYNC_LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/lib/doppler-secret-read.sh
source "${_BOOTSTRAP_SYNC_LIB_DIR}/doppler-secret-read.sh"
# shellcheck source=scripts/lib/doppler-secrets.sh
source "${_BOOTSTRAP_SYNC_LIB_DIR}/doppler-secrets.sh"
# shellcheck source=scripts/lib/secret-shell-guard.sh
source "${_BOOTSTRAP_SYNC_LIB_DIR}/secret-shell-guard.sh"

bootstrap_sync_env_secrets_to_doppler() {
  local project=$1
  local infra_config=$2
  local bootstrap_admin_config=$3
  local confirm_fn=$4
  local state

  if [[ -n "${GITHUB_TOFU_RELEASE_ENV:-}" ]]; then
    state="$(doppler_secret_state GITHUB_TOFU_RELEASE_ENV "${project}" "${bootstrap_admin_config}")" || return 1
    if [[ "${state}" == missing ]]; then
      "${confirm_fn}" "Write GITHUB_TOFU_RELEASE_ENV from environment to Doppler (${bootstrap_admin_config})?"
      doppler_secret_set_stdin GITHUB_TOFU_RELEASE_ENV "${GITHUB_TOFU_RELEASE_ENV}" "${project}" "${bootstrap_admin_config}"
    fi
  fi

  if [[ -n "${CLOUDFLARE_ACCOUNT_ID:-}" ]]; then
    state="$(doppler_secret_state CLOUDFLARE_ACCOUNT_ID "${project}" "${infra_config}")" || return 1
    if [[ "${state}" == missing ]]; then
      "${confirm_fn}" "Write CLOUDFLARE_ACCOUNT_ID from environment to Doppler (${infra_config})?"
      doppler_secret_set_stdin CLOUDFLARE_ACCOUNT_ID "${CLOUDFLARE_ACCOUNT_ID}" "${project}" "${infra_config}"
    fi
  fi

  state="$(doppler_secret_state GITHUB_TOFU_RELEASE_ENV "${project}" "${bootstrap_admin_config}")" || return 1
  if [[ "${state}" == missing ]]; then
    echo "Missing GITHUB_TOFU_RELEASE_ENV in Doppler ${project}/${bootstrap_admin_config} (set env or store secret)." >&2
    return 1
  fi
}

# Generates and stores a passphrase only when Doppler says it is absent or empty.
bootstrap_ensure_generated_passphrase() {
  local name=$1
  local project=$2
  local config=$3
  local confirm_fn=$4
  local state pass
  state="$(doppler_secret_state "${name}" "${project}" "${config}")" || return 1
  if [[ "${state}" == present ]]; then
    return 0
  fi
  pass="$(secret_shell_generate_passphrase)" || return 1
  "${confirm_fn}" "Create ${name} in Doppler (${config})?"
  doppler_secret_set_stdin "${name}" "${pass}" "${project}" "${config}"
}
