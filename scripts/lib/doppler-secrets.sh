# shellcheck shell=bash
# Shared helpers for writing Doppler secrets without exposing values on argv.

doppler_secret_set_stdin() {
  local key=$1
  local value=$2
  local project=${3:-${DOPPLER_PROJECT:?DOPPLER_PROJECT must be set}}
  local config=${4:-${DOPPLER_CONFIG:?DOPPLER_CONFIG must be set}}

  if [[ -z "${key}" ]]; then
    echo "doppler_secret_set_stdin: secret name is required" >&2
    return 1
  fi

  if [[ "${BOOTSTRAP_DOPPLER_SECRET_DRY_RUN:-}" == "1" ]]; then
    printf 'dry-run: doppler secrets set %s (value via stdin)\n' "${key}"
    return 0
  fi

  printf '%s' "${value}" | doppler secrets set "${key}" --silent --no-interactive -p "${project}" -c "${config}"
}
