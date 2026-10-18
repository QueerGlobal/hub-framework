# shellcheck shell=bash
# Read-only Cloudflare capability probes (no token policy or token list APIs).

_LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/lib/curl-bearer-auth.sh
source "${_LIB_DIR}/curl-bearer-auth.sh"

preflight_cloudflare_response_success() {
  local body=$1
  echo "${body}" | jq -e '.success == true' >/dev/null 2>&1
}

preflight_cloudflare_verify_result_active() {
  local verify_json=$1
  local status
  status="$(echo "${verify_json}" | jq -r '.result.status // empty')"
  if [[ "${status}" != "active" ]]; then
    echo "Cloudflare token verify status is not active (got: ${status:-unknown})." >&2
    return 1
  fi
}

preflight_cloudflare_capability_r2_list_buckets() {
  local token=$1
  local account_id=$2
  local resp
  resp="$(curl_with_bearer "${token}" \
    "https://api.cloudflare.com/client/v4/accounts/${account_id}/r2/buckets?per_page=1" 2>/dev/null)" || {
    echo "Cloudflare R2 list buckets failed (need account-scoped R2 access)." >&2
    return 1
  }
  if ! preflight_cloudflare_response_success "${resp}"; then
    echo "Cloudflare R2 list buckets returned success=false." >&2
    return 1
  fi
}

# Catalog read for account-owned token permission groups (same as OpenTofu data source).
preflight_cloudflare_capability_token_permission_groups() {
  local token=$1
  local account_id=$2
  local resp
  resp="$(curl_with_bearer "${token}" \
    "https://api.cloudflare.com/client/v4/accounts/${account_id}/tokens/permission_groups?per_page=1" 2>/dev/null)" || {
    echo "Cloudflare token permission groups read failed (needed for bootstrap OpenTofu data source)." >&2
    return 1
  }
  if ! preflight_cloudflare_response_success "${resp}"; then
    echo "Cloudflare token permission groups returned success=false." >&2
    return 1
  fi
}
