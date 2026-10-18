#!/usr/bin/env bash
# Mutation-proves the CI bootstrap-admin guard: every broken or over-scoped token fails with its own reason.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
GUARD="${ROOT}/scripts/doppler-ci-guard.sh"
TESTDIR="$(mktemp -d)"
trap 'rm -rf "${TESTDIR}"' EXIT

MOCK_BIN="${TESTDIR}/bin"
NO_DOPPLER_BIN="${TESTDIR}/no-doppler-bin"
mkdir -p "${MOCK_BIN}" "${NO_DOPPLER_BIN}"
for tool in bash env dirname; do
  ln -s "$(command -v "${tool}")" "${NO_DOPPLER_BIN}/${tool}"
done

# MOCK_DOPPLER_READABLE lists the configs and secrets this fake token may read.
cat > "${MOCK_BIN}/doppler" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail
readable=" ${MOCK_DOPPLER_READABLE:-} "
[[ "${1:-}" == secrets ]] || { echo "unexpected doppler: $*" >&2; exit 1; }
shift
name=""
config=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    -c) config=$2; shift 2 ;;
    -p) shift 2 ;;
    get) name=$2; shift 2 ;;
    *) shift ;;
  esac
done
if [[ -z "${name}" ]]; then
  [[ "${readable}" == *" ${config} "* ]] && { echo "NAME"; exit 0; }
else
  [[ "${readable}" == *" ${config}/${name} "* ]] && { echo "value"; exit 0; }
fi
echo "doppler: access denied" >&2
exit 1
MOCK
chmod +x "${MOCK_BIN}/doppler"

run_guard() {
  local path=$1
  shift
  env -i PATH="${path}" "$@" bash "${GUARD}" 2>&1
}

expect_fail() {
  local label=$1 needle=$2 path=$3
  shift 3
  local out rc=0
  out="$(run_guard "${path}" "$@")" || rc=$?
  if [[ ${rc} -eq 0 ]]; then
    echo "FAIL: guard passed for ${label}" >&2
    exit 1
  fi
  if ! grep -Fq "${needle}" <<<"${out}"; then
    echo "FAIL: guard failed for ${label} without reason: ${needle}" >&2
    echo "${out}" >&2
    exit 1
  fi
  echo "OK: guard fails closed for ${label}"
}

MOCK_PATH="${MOCK_BIN}:${NO_DOPPLER_BIN}"

expect_fail "unset token" "DOPPLER_TOKEN is unset" "${MOCK_PATH}"
expect_fail "missing doppler CLI" "doppler CLI is not installed" "${NO_DOPPLER_BIN}" DOPPLER_TOKEN=dp.st.fake
expect_fail "token that cannot read infra" "cannot list hub-framework/infra" "${MOCK_PATH}" \
  DOPPLER_TOKEN=dp.st.fake MOCK_DOPPLER_READABLE=""
expect_fail "token that lists bootstrap-admin" "can list bootstrap-admin secrets" "${MOCK_PATH}" \
  DOPPLER_TOKEN=dp.st.fake MOCK_DOPPLER_READABLE="infra bootstrap-admin"
expect_fail "token that reads the bootstrap passphrase" \
  "can read bootstrap-admin secret TOFU_BOOTSTRAP_STATE_ENCRYPTION_PASSPHRASE" "${MOCK_PATH}" \
  DOPPLER_TOKEN=dp.st.fake MOCK_DOPPLER_READABLE="infra bootstrap-admin/TOFU_BOOTSTRAP_STATE_ENCRYPTION_PASSPHRASE"
expect_fail "token that reads the GitHub release token" \
  "can read bootstrap-admin secret GITHUB_TOFU_RELEASE_ENV" "${MOCK_PATH}" \
  DOPPLER_TOKEN=dp.st.fake MOCK_DOPPLER_READABLE="infra bootstrap-admin/GITHUB_TOFU_RELEASE_ENV"
expect_fail "token that reads the bootstrap state backup" \
  "can read bootstrap-admin secret TOFU_BOOTSTRAP_STATE_LOCAL_BACKUP" "${MOCK_PATH}" \
  DOPPLER_TOKEN=dp.st.fake MOCK_DOPPLER_READABLE="infra bootstrap-admin/TOFU_BOOTSTRAP_STATE_LOCAL_BACKUP"

out="$(run_guard "${MOCK_PATH}" DOPPLER_TOKEN=dp.st.fake MOCK_DOPPLER_READABLE="infra")" || {
  echo "FAIL: guard rejected an infra-only token" >&2
  echo "${out}" >&2
  exit 1
}
echo "OK: guard passes an infra-only token"
