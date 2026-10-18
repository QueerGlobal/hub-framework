# shellcheck shell=bash
# Preflight checks for bootstrap-release-infra.sh (read-only; no infrastructure writes).

_PREFLIGHT_LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/lib/cloudflare-preflight-permissions.sh
source "${_PREFLIGHT_LIB_DIR}/cloudflare-preflight-permissions.sh"
# shellcheck source=scripts/lib/cloudflare-account-token-write-probe.sh
source "${_PREFLIGHT_LIB_DIR}/cloudflare-account-token-write-probe.sh"
# shellcheck source=scripts/lib/github-preflight.sh
source "${_PREFLIGHT_LIB_DIR}/github-preflight.sh"

preflight_require_commands() {
  local c
  for c in "$@"; do
    command -v "${c}" >/dev/null 2>&1 || {
      echo "Missing required command: ${c}" >&2
      return 1
    }
  done
}

preflight_doppler_logged_in() {
  if ! doppler me >/dev/null 2>&1; then
    echo "Doppler CLI is not logged in. Run: doppler login" >&2
    return 1
  fi
}

preflight_doppler_project_exists() {
  local project=$1
  if doppler projects get "${project}" >/dev/null 2>&1; then
    return 0
  fi
  return 1
}

preflight_ensure_doppler_project_and_config() {
  local project=$1
  local config=$2
  if ! doppler projects get "${project}" >/dev/null 2>&1; then
    echo "Creating Doppler project ${project}..."
    doppler projects create "${project}"
  fi
  if ! doppler configs get "${config}" --project "${project}" >/dev/null 2>&1; then
    echo "Creating Doppler config ${project}/${config}..."
    doppler configs create "${config}" --project "${project}"
  fi
}

preflight_doppler_secret_exists() {
  local name=$1
  local project=$2
  local config=$3
  if ! doppler secrets get "${name}" --plain -p "${project}" -c "${config}" >/dev/null 2>&1; then
    echo "Missing Doppler secret ${name} in ${project}/${config}." >&2
    return 1
  fi
}

preflight_cloudflare_account_id_resolved() {
  local project=$1
  local config=$2
  local from_doppler
  if [[ -n "${CLOUDFLARE_ACCOUNT_ID:-}" ]]; then
    printf '%s' "${CLOUDFLARE_ACCOUNT_ID}"
    return 0
  fi
  if preflight_doppler_project_exists "${project}"; then
    from_doppler="$(doppler secrets get CLOUDFLARE_ACCOUNT_ID --plain -p "${project}" -c "${config}" 2>/dev/null || true)"
    if [[ -n "${from_doppler}" ]]; then
      printf '%s' "${from_doppler}"
      return 0
    fi
  fi
  return 1
}

# Returns configured Cloudflare account id or prints a reason and fails.
preflight_cloudflare_account_id_required() {
  local project=$1
  local config=$2
  local account_id
  account_id="$(preflight_cloudflare_account_id_resolved "${project}" "${config}" || true)"
  if [[ -z "${account_id}" ]]; then
    echo "Set CLOUDFLARE_ACCOUNT_ID or store CLOUDFLARE_ACCOUNT_ID in Doppler ${project}/${config} before bootstrap." >&2
    return 1
  fi
  printf '%s' "${account_id}"
}

# Read-only validation of all inputs required before any Doppler project/config create.
preflight_bootstrap_inputs_readonly() {
  local project=$1
  local infra_config=$2
  local bootstrap_admin_config=$3
  local account_id

  preflight_doppler_logged_in || return 1

  preflight_github_tofu_release_env "${project}" "${bootstrap_admin_config}" || return 1

  if [[ -z "${CLOUDFLARE_API_TOKEN:-}" ]]; then
    echo "Set CLOUDFLARE_API_TOKEN (see infra/opentofu/state-bootstrap/README.md)." >&2
    return 1
  fi

  account_id="$(preflight_cloudflare_account_id_required "${project}" "${infra_config}")" || return 1
  preflight_cloudflare_bootstrap_api_token "${CLOUDFLARE_API_TOKEN}" "${account_id}" || return 1
}

preflight_cloudflare_bootstrap_api_token() {
  local token=$1
  local account_id=$2
  local verify

  verify="$(curl_with_bearer "${token}" \
    "https://api.cloudflare.com/client/v4/accounts/${account_id}/tokens/verify" 2>/dev/null || true)"
  if [[ "$(echo "${verify}" | jq -r '.success // false')" != "true" ]]; then
    echo "Bootstrap requires an account-owned Cloudflare API token (account verify failed)." >&2
    return 1
  fi
  preflight_cloudflare_verify_result_active "${verify}" || return 1
  local verify_account_id
  verify_account_id="$(echo "${verify}" | jq -r '.result.account.id // empty')"
  if [[ -n "${verify_account_id}" && "${verify_account_id}" != "${account_id}" ]]; then
    echo "Cloudflare token verify is for account ${verify_account_id}, not configured account ${account_id}." >&2
    return 1
  fi

  preflight_cloudflare_capability_r2_list_buckets "${token}" "${account_id}" || return 1
  preflight_cloudflare_capability_token_permission_groups "${token}" "${account_id}" || return 1
  cloudflare_account_token_write_probe "${token}" "${account_id}" || return 1
}
