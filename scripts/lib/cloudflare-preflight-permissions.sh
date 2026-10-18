# shellcheck shell=bash
# Cloudflare bootstrap token permission names (API uses "Write", not dashboard "Edit").

export PREFLIGHT_CF_PERM_R2_STORAGE_WRITE_NAME="Workers R2 Storage Write"
export PREFLIGHT_CF_PERM_ACCOUNT_API_TOKENS_WRITE_NAME="Account API Tokens Write"

# Example IDs from Cloudflare permission group catalog (32-char hex). Runtime checks resolve
# IDs from the live catalog; tests use these constants in fixtures.
export PREFLIGHT_CF_FIXTURE_R2_STORAGE_WRITE_ID="f2a8c1d04e3b4a5f9c0d1e2f3a4b5c6d"
export PREFLIGHT_CF_FIXTURE_ACCOUNT_API_TOKENS_WRITE_ID="a1b2c3d4e5f6478990abcdef12345678"

preflight_cloudflare_response_success() {
  local body=$1
  echo "${body}" | jq -e '.success == true' >/dev/null 2>&1
}

preflight_cloudflare_catalog_permission_id() {
  local catalog_json=$1
  local permission_name=$2
  echo "${catalog_json}" | jq -r --arg n "${permission_name}" '
    .result[]? | select(.name == $n) | .id // empty
  ' | head -1
}

# Allow policies only; deny policies are ignored (not treated as grants).
preflight_cloudflare_token_has_allow_permission_ids() {
  local token_json=$1
  shift
  local required_id missing=0
  for required_id in "$@"; do
    if [[ -z "${required_id}" ]]; then
      continue
    fi
    if echo "${token_json}" | jq -e --arg id "${required_id}" '
      [
        .result.policies[]?
        | select((.effect // "allow") == "allow")
        | .permission_groups[]?
        | .id
      ] | index($id) != null
    ' >/dev/null; then
      continue
    fi
    echo "Cloudflare token is missing allow policy for permission group id ${required_id}" >&2
    missing=1
  done
  return "${missing}"
}

preflight_cloudflare_capability_r2_list_buckets() {
  local token=$1
  local account_id=$2
  local resp
  resp="$(curl -fsS -H "Authorization: Bearer ${token}" \
    "https://api.cloudflare.com/client/v4/accounts/${account_id}/r2/buckets?per_page=1" 2>/dev/null)" || {
    echo "Cloudflare R2 list buckets failed (need ${PREFLIGHT_CF_PERM_R2_STORAGE_WRITE_NAME} at account scope)." >&2
    return 1
  }
  if ! preflight_cloudflare_response_success "${resp}"; then
    echo "Cloudflare R2 list buckets returned success=false." >&2
    return 1
  fi
}

preflight_cloudflare_capability_list_account_tokens() {
  local token=$1
  local account_id=$2
  local resp
  resp="$(curl -fsS -H "Authorization: Bearer ${token}" \
    "https://api.cloudflare.com/client/v4/accounts/${account_id}/tokens?per_page=1" 2>/dev/null)" || {
    echo "Cloudflare account API tokens list failed (need Account API Tokens Read or Write)." >&2
    return 1
  }
  if ! preflight_cloudflare_response_success "${resp}"; then
    echo "Cloudflare account API tokens list returned success=false." >&2
    return 1
  fi
}

preflight_cloudflare_fetch_permission_groups_catalog() {
  local token=$1
  local account_id=$2
  local catalog
  catalog="$(curl -fsS -H "Authorization: Bearer ${token}" \
    "https://api.cloudflare.com/client/v4/accounts/${account_id}/tokens/permission_groups?per_page=50" 2>/dev/null)" || {
    echo "Cloudflare permission groups catalog request failed." >&2
    return 1
  }
  if ! preflight_cloudflare_response_success "${catalog}"; then
    echo "Cloudflare permission groups catalog returned success=false." >&2
    return 1
  fi
  printf '%s' "${catalog}"
}
