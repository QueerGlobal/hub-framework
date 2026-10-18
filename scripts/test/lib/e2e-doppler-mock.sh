# shellcheck shell=bash
# Mock Doppler CLI for bootstrap E2E and argv tests.

e2e_doppler_mock_install() {
  local fake_bin=$1
  mkdir -p "${fake_bin}"
  cat > "${fake_bin}/doppler" <<'DOPPLER'
#!/usr/bin/bash
set -euo pipefail
if [[ -n "${ARGV_SHIM_LOG:-}" ]]; then
  {
    printf 'SHIM:doppler ARGV:'
    printf '%q ' "$@"
    printf '\n'
    tr '\0' ' ' < /proc/self/cmdline
    printf '\n'
  } >> "${ARGV_SHIM_LOG}"
fi
write_log="${E2E_DOPPLER_WRITE_LOG:?}"
secret_db="${E2E_DOPPLER_SECRET_DB:?}"
preflight_marker="${BOOTSTRAP_PREFLIGHT_MARKER_PATH:-}"

preflight_marker_ready() {
  [[ -n "${preflight_marker}" && -f "${preflight_marker}" ]]
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
      if ! preflight_marker_ready; then
        echo "E2E FAIL: Doppler project create before preflight marker file" >&2
        exit 1
      fi
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
      if ! preflight_marker_ready; then
        echo "E2E FAIL: Doppler config create before preflight marker file" >&2
        exit 1
      fi
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
      project=""
      config=""
      while [[ $# -gt 0 ]]; do
        case "$1" in
          -p) project=$2; shift 2 ;;
          -c) config=$2; shift 2 ;;
          --plain) shift ;;
          *) shift ;;
        esac
      done
      line="$(awk -F '\t' -v n="${name}" -v p="${project}" -v c="${config}" \
        '$2 == n && $3 == p && $4 == c { last=$0 } END { print last }' "${secret_db}")"
      if [[ -z "${line}" ]]; then
        exit 1
      fi
      awk -F '\t' '{ print $5 }' <<<"${line}"
      exit 0
    fi
    if [[ "${sub}" == "set" ]]; then
      if ! preflight_marker_ready; then
        echo "E2E FAIL: Doppler secret write before preflight marker file" >&2
        exit 1
      fi
      project=""
      config=""
      name=${2:-}
      shift 2 || true
      while [[ $# -gt 0 ]]; do
        case "$1" in
          -p) project=$2; shift 2 ;;
          -c) config=$2; shift 2 ;;
          *) shift ;;
        esac
      done
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
    while [[ $# -gt 0 ]]; do
      case "$1" in
        -p|-c|--project|--config) shift 2 ;;
        --) shift; break ;;
        *) shift ;;
      esac
    done
    exec "$@"
    ;;
esac
echo "unexpected doppler: $*" >&2
exit 1
DOPPLER
  chmod +x "${fake_bin}/doppler"
}
