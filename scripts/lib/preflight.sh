# shellcheck shell=bash
# Preflight checks for bootstrap-release-infra.sh (read-only; no infrastructure writes).

_PREFLIGHT_LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/lib/cloudflare-preflight-permissions.sh
source "${_PREFLIGHT_LIB_DIR}/cloudflare-preflight-permissions.sh"

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

preflight_github_tofu_release_env() {
  local project=$1
  local config=$2
  if [[ -n "${GITHUB_TOFU_RELEASE_ENV:-}" ]]; then
    return 0
  fi
  if preflight_doppler_project_exists "${project}"; then
    preflight_doppler_secret_exists GITHUB_TOFU_RELEASE_ENV "${project}" "${config}"
    return $?
  fi
  echo "Set GITHUB_TOFU_RELEASE_ENV in the environment, or create Doppler project ${project} with secret GITHUB_TOFU_RELEASE_ENV before bootstrap." >&2
  return 1
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

preflight_cloudflare_account_id_readonly() {
  local project=$1
  local config=$2
  local account_id
  account_id="$(preflight_cloudflare_account_id_resolved "${project}" "${config}" || true)"
  if [[ -z "${account_id}" ]]; then
    echo "Set CLOUDFLARE_ACCOUNT_ID or store CLOUDFLARE_ACCOUNT_ID in Doppler ${project}/${config} before bootstrap." >&2
    return 1
  fi
}

# Read-only validation of all inputs required before any Doppler project/config create.
preflight_bootstrap_inputs_readonly() {
  local project=$1
  local infra_config=$2

  preflight_doppler_logged_in || return 1

  if [[ -z "${CLOUDFLARE_API_TOKEN:-}" ]]; then
    echo "Set CLOUDFLARE_API_TOKEN (see infra/opentofu/state-bootstrap/README.md)." >&2
    return 1
  fi

  preflight_cloudflare_account_id_readonly "${project}" "${infra_config}" || return 1

  local account_id
  account_id="$(preflight_cloudflare_account_id_resolved "${project}" "${infra_config}")"
  preflight_cloudflare_api_token "${CLOUDFLARE_API_TOKEN}" "${account_id}" || return 1

  preflight_github_tofu_release_env "${project}" "${infra_config}" || return 1
}

preflight_cloudflare_account_id() {
  local account_id=$1
  if [[ -z "${account_id}" ]]; then
    echo "Set CLOUDFLARE_ACCOUNT_ID or store CLOUDFLARE_ACCOUNT_ID in Doppler." >&2
    return 1
  fi
}

preflight_cloudflare_fetch_token_details() {
  local token=$1
  local account_id=$2
  local token_id=$3
  local details=""
  if details="$(curl -fsS -H "Authorization: Bearer ${token}" \
    "https://api.cloudflare.com/client/v4/user/tokens/${token_id}" 2>/dev/null)" \
    && [[ "$(echo "${details}" | jq -r '.success')" == "true" ]]; then
    echo "${details}"
    return 0
  fi
  if details="$(curl -fsS -H "Authorization: Bearer ${token}" \
    "https://api.cloudflare.com/client/v4/accounts/${account_id}/tokens/${token_id}" 2>/dev/null)" \
    && [[ "$(echo "${details}" | jq -r '.success')" == "true" ]]; then
    echo "${details}"
    return 0
  fi
  echo "Cloudflare token details failed for user and account token endpoints (need User API Tokens Read or Account API Tokens Read)." >&2
  return 1
}

preflight_cloudflare_verify_token_id() {
  local token=$1
  local account_id=$2
  local verify token_id

  verify="$(curl -fsS -H "Authorization: Bearer ${token}" \
    "https://api.cloudflare.com/client/v4/user/tokens/verify" 2>/dev/null || true)"
  if [[ "$(echo "${verify}" | jq -r '.success // false')" == "true" ]]; then
    token_id="$(echo "${verify}" | jq -r '.result.id // empty')"
    if [[ -n "${token_id}" ]]; then
      printf '%s' "${token_id}"
      return 0
    fi
  fi

  verify="$(curl -fsS -H "Authorization: Bearer ${token}" \
    "https://api.cloudflare.com/client/v4/accounts/${account_id}/tokens/verify" 2>/dev/null || true)"
  if [[ "$(echo "${verify}" | jq -r '.success // false')" != "true" ]]; then
    echo "Cloudflare token verify failed for user and account verify endpoints." >&2
    return 1
  fi

  token_id="$(echo "${verify}" | jq -r '.result.id // empty')"
  if [[ -z "${token_id}" ]]; then
    echo "Cloudflare account token verify did not return a token id." >&2
    return 1
  fi
  printf '%s' "${token_id}"
}

preflight_cloudflare_api_token() {
  local token=$1
  local account_id=$2
  local token_id details catalog
  local r2_write_id api_tokens_write_id

  token_id="$(preflight_cloudflare_verify_token_id "${token}" "${account_id}")" || return 1

  preflight_cloudflare_capability_r2_list_buckets "${token}" "${account_id}" || return 1
  preflight_cloudflare_capability_list_account_tokens "${token}" "${account_id}" || return 1

  catalog="$(preflight_cloudflare_fetch_permission_groups_catalog "${token}" "${account_id}")" || return 1
  r2_write_id="$(preflight_cloudflare_catalog_permission_id "${catalog}" \
    "${PREFLIGHT_CF_PERM_R2_STORAGE_WRITE_NAME}")"
  api_tokens_write_id="$(preflight_cloudflare_catalog_permission_id "${catalog}" \
    "${PREFLIGHT_CF_PERM_ACCOUNT_API_TOKENS_WRITE_NAME}")"
  if [[ -z "${r2_write_id}" || -z "${api_tokens_write_id}" ]]; then
    echo "Cloudflare permission catalog missing ${PREFLIGHT_CF_PERM_R2_STORAGE_WRITE_NAME} or ${PREFLIGHT_CF_PERM_ACCOUNT_API_TOKENS_WRITE_NAME}." >&2
    return 1
  fi

  if details="$(preflight_cloudflare_fetch_token_details "${token}" "${account_id}" "${token_id}")"; then
    preflight_cloudflare_token_has_allow_permission_ids "${details}" \
      "${r2_write_id}" "${api_tokens_write_id}" || return 1
  else
    echo "Note: skipping token policy ID probe (grant User API Tokens Read or Account API Tokens Read to enable)." >&2
  fi
}
