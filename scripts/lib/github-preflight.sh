# shellcheck shell=bash
# Read-only GitHub token check for GITHUB_TOFU_RELEASE_ENV (no repo writes).

_GITHUB_PREFLIGHT_LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/lib/curl-bearer-auth.sh
source "${_GITHUB_PREFLIGHT_LIB_DIR}/curl-bearer-auth.sh"

preflight_github_tofu_release_env_token() {
  local project=$1
  local bootstrap_admin_config=$2
  if [[ -n "${GITHUB_TOFU_RELEASE_ENV:-}" ]]; then
    printf '%s' "${GITHUB_TOFU_RELEASE_ENV}"
    return 0
  fi
  if preflight_doppler_project_exists "${project}"; then
    doppler secrets get GITHUB_TOFU_RELEASE_ENV --plain -p "${project}" -c "${bootstrap_admin_config}" 2>/dev/null
    return $?
  fi
  echo "Set GITHUB_TOFU_RELEASE_ENV in the environment, or store it in Doppler ${project}/${bootstrap_admin_config} before bootstrap." >&2
  return 1
}

preflight_github_tofu_release_env_api() {
  local token=$1
  local body
  body="$(printf 'Authorization: Bearer %s\n' "${token}" \
    | curl -sS -H @- \
      -H "Accept: application/vnd.github+json" \
      -H "X-GitHub-Api-Version: 2022-11-28" \
      "https://api.github.com/user" 2>/dev/null)" || {
    echo "GITHUB_TOFU_RELEASE_ENV could not reach the GitHub API (network or TLS failure)." >&2
    return 1
  }
  if ! echo "${body}" | jq -e '.login != null and (.login | length) > 0' >/dev/null 2>&1; then
    echo "GITHUB_TOFU_RELEASE_ENV is not a valid GitHub token (user API returned no login)." >&2
    return 1
  fi
}

preflight_github_tofu_release_env() {
  local project=$1
  local bootstrap_admin_config=$2
  local token
  token="$(preflight_github_tofu_release_env_token "${project}" "${bootstrap_admin_config}")" || return 1
  preflight_github_tofu_release_env_api "${token}" || return 1
}
