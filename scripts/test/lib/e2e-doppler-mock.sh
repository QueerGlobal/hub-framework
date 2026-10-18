# shellcheck shell=bash
# Mock Doppler CLI for bootstrap E2E and argv tests.

e2e_doppler_mock_install() {
  local fake_bin=$1
  local shim_log_lib
  shim_log_lib="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/argv-shim-log.sh"
  mkdir -p "${fake_bin}"
  cat > "${fake_bin}/doppler" <<DOPPLER
#!/usr/bin/bash
set -euo pipefail
# shellcheck source=scripts/test/lib/argv-shim-log.sh
source "${shim_log_lib}"
argv_shim_log_invocation doppler "\$@"
DOPPLER
  cat >> "${fake_bin}/doppler" <<'DOPPLER'
write_log="${E2E_DOPPLER_WRITE_LOG:?}"
secret_db="${E2E_DOPPLER_SECRET_DB:?}"
curl_events="${E2E_CURL_EVENT_LOG:?}"

# No write may come before the GitHub token check or the last Cloudflare probe (token create).
require_preflight() {
  local what=$1
  if ! grep -Eq '^GET https://api\.github\.com/user$' "${curl_events}" 2>/dev/null; then
    echo "E2E FAIL: Doppler ${what} before GitHub token preflight" >&2
    exit 1
  fi
  if ! grep -Eq '^POST https://api\.cloudflare\.com/client/v4/accounts/[^/]+/tokens$' "${curl_events}" 2>/dev/null; then
    echo "E2E FAIL: Doppler ${what} before Cloudflare preflight finished" >&2
    exit 1
  fi
}

# Sets project and config from -p/-c (or long forms); args after -- land in rest.
parse_scope() {
  project=""
  config=""
  rest=()
  while [[ $# -gt 0 ]]; do
    case "$1" in
      -p|--project) project=$2; shift 2 ;;
      -c|--config) config=$2; shift 2 ;;
      --) shift; rest=("$@"); return 0 ;;
      *) shift ;;
    esac
  done
}

cmd=${1:-}
shift || true

if [[ "${cmd}" == "me" ]]; then
  exit 0
fi

case "${cmd}" in
  projects)
    sub=${1:-}
    if [[ "${sub}" == "get" ]]; then
      if grep -q 'created project' "${write_log}" 2>/dev/null; then
        exit 0
      fi
      exit 1
    fi
    if [[ "${sub}" == "create" ]]; then
      require_preflight "project create"
      echo "created project" >>"${write_log}"
      exit 0
    fi
    ;;
  configs)
    sub=${1:-}
    if [[ "${sub}" == "get" ]]; then
      cfg_name=${2:-}
      if grep -q "config ${cfg_name} " "${write_log}" 2>/dev/null; then
        exit 0
      fi
      exit 1
    fi
    if [[ "${sub}" == "create" ]]; then
      require_preflight "config create"
      cfg_name=${2:-}
      echo "config ${cfg_name} created" >>"${write_log}"
      exit 0
    fi
    if [[ "${sub}" == "tokens" ]]; then
      sub2=${2:-}
      if [[ "${sub2}" == "list" ]]; then
        printf '%s\n' '[]'
        exit 0
      fi
      if [[ "${sub2}" == "create" ]]; then
        echo "dp.st.e2e.mock.ci"
        exit 0
      fi
    fi
    ;;
  secrets)
    sub=${1:-}
    if [[ "${sub}" == "get" ]]; then
      name=${2:-}
      shift 2 || true
      parse_scope "$@"
      line="$(awk -F '\t' -v n="${name}" -v p="${project}" -v c="${config}" \
        '$2 == n && $3 == p && $4 == c { last=$0 } END { print last }' "${secret_db}")"
      if [[ -z "${line}" ]]; then
        exit 1
      fi
      awk -F '\t' '{ print $5 }' <<<"${line}"
      exit 0
    fi
    if [[ "${sub}" == "set" ]]; then
      require_preflight "secret write"
      name=${2:-}
      shift 2 || true
      parse_scope "$@"
      if [[ "${config}" == "bootstrap-admin" ]]; then
        case "${name}" in
          TOFU_BOOTSTRAP_*|GITHUB_TOFU_RELEASE_ENV) ;;
          *)
            echo "E2E FAIL: unexpected secret ${name} in bootstrap-admin" >&2
            exit 1
            ;;
        esac
      fi
      if [[ "${config}" == "infra" && "${name}" == "GITHUB_TOFU_RELEASE_ENV" ]]; then
        echo "E2E FAIL: GITHUB_TOFU_RELEASE_ENV must not be written to infra config" >&2
        exit 1
      fi
      if [[ "${config}" == "infra" ]]; then
        case "${name}" in
          TOFU_BOOTSTRAP_*)
            echo "E2E FAIL: bootstrap secret written to infra config" >&2
            exit 1
            ;;
        esac
      fi
      value="$(cat -)"
      printf '%s\t%s\t%s\t%s\t%s\n' "$(date +%s)" "${name}" "${project}" "${config}" "${value}" >>"${secret_db}"
      echo "secret set ${name} ${project}/${config}" >>"${write_log}"
      exit 0
    fi
    ;;
  run)
    parse_scope "$@"
    # Like the real CLI, inject the config's secrets as environment variables.
    while IFS=$'\t' read -r _ name p c value; do
      if [[ "${p}" == "${project}" && "${c}" == "${config}" ]]; then
        export "${name}=${value}"
      fi
    done <"${secret_db}"
    exec "${rest[@]}"
    ;;
esac
echo "unexpected doppler: $*" >&2
exit 1
DOPPLER
  chmod +x "${fake_bin}/doppler"
}
