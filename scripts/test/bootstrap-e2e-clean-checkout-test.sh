#!/usr/bin/env bash
# Clean-checkout E2E: preflight before writes, config split, bootstrap test, release init.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CLONE_DIR="$(mktemp -d)"
TESTDIR="$(mktemp -d)"
trap 'rm -rf "${CLONE_DIR}" "${TESTDIR}"' EXIT

git -C "${ROOT}" archive HEAD | tar -x -C "${CLONE_DIR}"
cp -a "${ROOT}/scripts/." "${CLONE_DIR}/scripts/"
cp -a "${ROOT}/infra/." "${CLONE_DIR}/infra/"
cp -a "${ROOT}/.github/." "${CLONE_DIR}/.github/"

FAKE_BIN="${TESTDIR}/bin"
mkdir -p "${FAKE_BIN}"
WRITE_LOG="${TESTDIR}/doppler-writes.log"
SECRET_DB="${TESTDIR}/doppler-secrets.tsv"
: >"${WRITE_LOG}"
: >"${SECRET_DB}"

# shellcheck source=scripts/test/lib/cloudflare-preflight-mock.sh
source "${ROOT}/scripts/test/lib/cloudflare-preflight-mock.sh"
export CF_PREFLIGHT_FIXTURE_ROOT="${ROOT}/scripts/test/fixtures/cloudflare"
cloudflare_preflight_mock_install "${FAKE_BIN}"

cat > "${FAKE_BIN}/doppler" <<'DOPPLER'
#!/usr/bin/env bash
set -euo pipefail
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
          TOFU_BOOTSTRAP_*) ;;
          *)
            echo "E2E FAIL: unexpected secret ${name} in bootstrap-admin" >&2
            exit 1
            ;;
        esac
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
chmod +x "${FAKE_BIN}/doppler"

export PATH="${FAKE_BIN}:${PATH}"
export E2E_DOPPLER_WRITE_LOG="${WRITE_LOG}"
export E2E_DOPPLER_SECRET_DB="${SECRET_DB}"
export CLOUDFLARE_API_TOKEN=e2e-mock-token
export CLOUDFLARE_ACCOUNT_ID=00000000000000000000000000000001
export GITHUB_TOFU_RELEASE_ENV=e2e-mock-github-token
export BOOTSTRAP_AUTO_CONFIRM=1
export BOOTSTRAP_E2E=1

# Preset legacy env must not bypass the marker file gate.
export BOOTSTRAP_PREFLIGHT_COMPLETE=1
export BOOTSTRAP_PREFLIGHT_MARKER_PATH="${TESTDIR}/preset-env-not-a-real-marker"
if PATH="${FAKE_BIN}:${PATH}" E2E_DOPPLER_WRITE_LOG="${WRITE_LOG}" E2E_DOPPLER_SECRET_DB="${SECRET_DB}" \
  doppler projects create hub-framework --project hub-framework 2>/dev/null; then
  echo "FAIL: preset BOOTSTRAP_PREFLIGHT_COMPLETE must not satisfy Doppler write gate" >&2
  exit 1
fi
echo "OK: preset env var does not bypass preflight marker file gate"
export BOOTSTRAP_DOPPLER_SECRET_DRY_RUN=0
export CF_MOCK_USER_VERIFY_OK=1
export CF_MOCK_USER_DETAILS_OK=1
export CF_MOCK_DETAILS_FIXTURE=token-details-account-owned-allow.json

(
  cd "${CLONE_DIR}"
  ./scripts/bootstrap-release-infra.sh
)

if ! grep -q 'secret set GITHUB_TOFU_RELEASE_ENV' "${WRITE_LOG}"; then
  echo "FAIL: E2E did not write GITHUB_TOFU_RELEASE_ENV to infra after create" >&2
  exit 1
fi

if grep -q 'bootstrap-admin' "${WRITE_LOG}" && grep -q 'TOFU_STATE_BUCKET' "${WRITE_LOG}"; then
  echo "FAIL: E2E mixed infra and bootstrap-admin writes unexpectedly" >&2
  exit 1
fi

echo "OK: clean-checkout E2E bootstrap path (preflight before writes, config split, init paths)"
