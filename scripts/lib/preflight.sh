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

preflight_cloudflare_account_id() {
  local account_id=$1
  if [[ -z "${account_id}" ]]; then
    echo "Set CLOUDFLARE_ACCOUNT_ID or store CLOUDFLARE_ACCOUNT_ID in Doppler." >&2
    return 1
  fi
}

preflight_cloudflare_api_token() {
  local token=$1
  local account_id=$2
  local verify
  verify="$(curl -fsS -H "Authorization: Bearer ${token}" \
    "https://api.cloudflare.com/client/v4/user/tokens/verify")" || {
    echo "Cloudflare token verify request failed." >&2
    return 1
  }
  if [[ "$(echo "${verify}" | jq -r '.success')" != "true" ]]; then
    echo "Cloudflare token verify failed: $(echo "${verify}" | jq -r '.errors[0].message // .')" >&2
    return 1
  fi

  local groups
  groups="$(curl -fsS -H "Authorization: Bearer ${token}" \
    "https://api.cloudflare.com/client/v4/accounts/${account_id}/tokens/permission_groups")" || {
    echo "Cloudflare permission groups request failed (need Account API Tokens Read or Write)." >&2
    return 1
  }
  if [[ "$(echo "${groups}" | jq -r '.success')" != "true" ]]; then
    echo "Cloudflare permission groups failed: $(echo "${groups}" | jq -r '.errors[0].message // .')" >&2
    return 1
  fi

  local missing=0
  for required in "Workers R2 Storage Write" "Account API Tokens Write"; do
    if ! echo "${groups}" | jq -e --arg n "${required}" '.result[] | select(.name == $n)' >/dev/null; then
      echo "Cloudflare token cannot list required permission group: ${required}" >&2
      missing=1
    fi
  done
  if [[ "${missing}" -ne 0 ]]; then
    echo "Bootstrap needs API Tokens Read/Write (Create additional tokens template) plus R2 edit." >&2
    return 1
  fi
}
