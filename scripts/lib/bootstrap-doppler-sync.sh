# shellcheck shell=bash
# Sync required bootstrap inputs from environment into Doppler after project/config exist.

bootstrap_doppler_secret_missing() {
  local name=$1
  local project=$2
  local config=$3
  ! doppler secrets get "${name}" --plain -p "${project}" -c "${config}" >/dev/null 2>&1
}

bootstrap_sync_env_secrets_to_doppler() {
  local project=$1
  local infra_config=$2
  local confirm_fn=$3

  if [[ -n "${GITHUB_TOFU_RELEASE_ENV:-}" ]] \
    && bootstrap_doppler_secret_missing GITHUB_TOFU_RELEASE_ENV "${project}" "${infra_config}"; then
    "${confirm_fn}" "Write GITHUB_TOFU_RELEASE_ENV from environment to Doppler (${infra_config})?"
    doppler_secret_set_stdin GITHUB_TOFU_RELEASE_ENV "${GITHUB_TOFU_RELEASE_ENV}" "${project}" "${infra_config}"
  fi

  if [[ -n "${CLOUDFLARE_ACCOUNT_ID:-}" ]] \
    && bootstrap_doppler_secret_missing CLOUDFLARE_ACCOUNT_ID "${project}" "${infra_config}"; then
    "${confirm_fn}" "Write CLOUDFLARE_ACCOUNT_ID from environment to Doppler (${infra_config})?"
    doppler_secret_set_stdin CLOUDFLARE_ACCOUNT_ID "${CLOUDFLARE_ACCOUNT_ID}" "${project}" "${infra_config}"
  fi

  if bootstrap_doppler_secret_missing GITHUB_TOFU_RELEASE_ENV "${project}" "${infra_config}"; then
    echo "Missing GITHUB_TOFU_RELEASE_ENV in Doppler ${project}/${infra_config} (set env or store secret)." >&2
    return 1
  fi
}
