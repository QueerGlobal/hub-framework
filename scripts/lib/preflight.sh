# shellcheck shell=bash
# Preflight checks for bootstrap-release-infra.sh (read-only; no infrastructure writes).

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

preflight_cloudflare_account_id() {
  local account_id=$1
  if [[ -z "${account_id}" ]]; then
    echo "Set CLOUDFLARE_ACCOUNT_ID or store CLOUDFLARE_ACCOUNT_ID in Doppler." >&2
    return 1
  fi
}

# preflight_cloudflare_token_has_permission_names checks token policy grants (not the account catalog).
preflight_cloudflare_token_has_permission_names() {
  local token_json=$1
  local catalog_json=$2
  shift 2
  local required name missing=0 id
  for required in "$@"; do
    if echo "${token_json}" | jq -e --arg n "${required}" '
      [.result.policies[]?.permission_groups[]? | .name] | index($n) != null
    ' >/dev/null; then
      continue
    fi
    id="$(echo "${catalog_json}" | jq -r --arg n "${required}" '
      .result[]? | select(.name == $n) | .id // empty
    ' | head -1)"
    if [[ -n "${id}" ]] && echo "${token_json}" | jq -e --arg id "${id}" '
      [.result.policies[]?.permission_groups[]? | .id] | index($id) != null
    ' >/dev/null; then
      continue
    fi
    echo "Cloudflare token is missing required permission: ${required}" >&2
    missing=1
  done
  return "${missing}"
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
  echo "Cloudflare token details failed for user and account token endpoints (need API Tokens Read/Write)." >&2
  return 1
}

preflight_cloudflare_api_token() {
  local token=$1
  local account_id=$2
  local verify token_id details catalog
  verify="$(curl -fsS -H "Authorization: Bearer ${token}" \
    "https://api.cloudflare.com/client/v4/user/tokens/verify")" || {
    echo "Cloudflare token verify request failed." >&2
    return 1
  }
  if [[ "$(echo "${verify}" | jq -r '.success')" != "true" ]]; then
    echo "Cloudflare token verify failed: $(echo "${verify}" | jq -r '.errors[0].message // .')" >&2
    return 1
  fi

  token_id="$(echo "${verify}" | jq -r '.result.id // empty')"
  if [[ -z "${token_id}" ]]; then
    echo "Cloudflare token verify did not return a token id." >&2
    return 1
  fi

  details="$(preflight_cloudflare_fetch_token_details "${token}" "${account_id}" "${token_id}")" || return 1

  catalog="$(curl -fsS -H "Authorization: Bearer ${token}" \
    "https://api.cloudflare.com/client/v4/accounts/${account_id}/tokens/permission_groups")" || {
    echo "Cloudflare permission groups request failed (need permission to read token groups)." >&2
    return 1
  }
  if [[ "$(echo "${catalog}" | jq -r '.success')" != "true" ]]; then
    echo "Cloudflare permission groups failed: $(echo "${catalog}" | jq -r '.errors[0].message // .')" >&2
    return 1
  fi

  preflight_cloudflare_token_has_permission_names "${details}" "${catalog}" \
    "Workers R2 Storage Write" \
    "Account API Tokens Write"
}
