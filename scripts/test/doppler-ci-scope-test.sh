#!/usr/bin/env bash
# Unit test: mock Doppler CLI enforces infra-only scope (no bootstrap-admin reads).
# Live CI proof: scripts/test/doppler-ci-runtime-guard.sh with the real infra-scoped
# DOPPLER_TOKEN secret (see scripts/bootstrap-release-infra.sh service token create).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
TESTDIR="$(mktemp -d)"
trap 'rm -rf "${TESTDIR}"' EXIT

FAKE_BIN="${TESTDIR}/bin"
mkdir -p "${FAKE_BIN}"

cat > "${FAKE_BIN}/doppler" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

cmd=${1:-}
shift || true

case "${cmd}" in
  configs)
    sub=${1:-}
    shift || true
    if [[ "${sub}" == tokens ]]; then
      sub2=${1:-}
      shift || true
      if [[ "${sub2}" == create ]]; then
        name=${1:-}
        echo "dp.st.ci.mock.${name}"
        exit 0
      fi
      if [[ "${sub2}" == list ]]; then
        printf '%s\n' '[]'
        exit 0
      fi
    fi
    ;;
  secrets)
    sub=${1:-}
    shift || true
    if [[ "${sub}" == get ]]; then
      name=${1:-}
      shift || true
      project=""
      config=""
      while [[ $# -gt 0 ]]; do
        case "$1" in
          -p) project=$2; shift 2 ;;
          -c) config=$2; shift 2 ;;
          *) shift ;;
        esac
      done
      if [[ "${config}" == "bootstrap-admin" ]]; then
        echo "doppler: access denied to config bootstrap-admin" >&2
        exit 1
      fi
      if [[ "${name}" == "TOFU_BOOTSTRAP_STATE_ENCRYPTION_PASSPHRASE" || "${name}" == "GITHUB_TOFU_RELEASE_ENV" ]]; then
        echo "doppler: secret not found" >&2
        exit 1
      fi
      echo "mock-infra-secret-value"
      exit 0
    fi
    if [[ "${sub}" == set ]]; then
      echo "unexpected doppler secrets set in scope test" >&2
      exit 1
    fi
    ;;
esac
echo "unexpected doppler invocation: $*" >&2
exit 1
EOF
chmod +x "${FAKE_BIN}/doppler"

export PATH="${FAKE_BIN}:${PATH}"
export DOPPLER_PROJECT_NAME=hub-framework
export DOPPLER_INFRA_CONFIG=infra
export DOPPLER_BOOTSTRAP_ADMIN_CONFIG=bootstrap-admin
export DOPPLER_CI_INFRA_SERVICE_TOKEN_NAME=e2e-ci-infra-read

# shellcheck source=scripts/lib/doppler-ci-token.sh
source "${ROOT}/scripts/lib/doppler-ci-token.sh"

token="$(ensure_doppler_ci_infra_service_token "${DOPPLER_PROJECT_NAME}" "${DOPPLER_INFRA_CONFIG}")"
if [[ "${token}" != dp.st.ci.mock.e2e-ci-infra-read ]]; then
  echo "FAIL: expected mock service token from ensure_doppler_ci_infra_service_token" >&2
  exit 1
fi

export DOPPLER_TOKEN="${token}"
if DOPPLER_TOKEN="${DOPPLER_TOKEN}" doppler secrets get TOFU_STATE_ENCRYPTION_PASSPHRASE \
  --plain -p "${DOPPLER_PROJECT_NAME}" -c "${DOPPLER_INFRA_CONFIG}" >/dev/null; then
  echo "OK: infra-scoped token can read infra config"
else
  echo "FAIL: infra-scoped token could not read infra config" >&2
  exit 1
fi

if DOPPLER_TOKEN="${DOPPLER_TOKEN}" doppler secrets get TOFU_BOOTSTRAP_STATE_ENCRYPTION_PASSPHRASE \
  --plain -p "${DOPPLER_PROJECT_NAME}" -c "${DOPPLER_BOOTSTRAP_ADMIN_CONFIG}" >/dev/null 2>&1; then
  echo "FAIL: infra-scoped token must not read bootstrap-admin" >&2
  exit 1
fi

if ! doppler_assert_ci_token_cannot_read_bootstrap_admin; then
  exit 1
fi

if DOPPLER_TOKEN="${DOPPLER_TOKEN}" doppler secrets get GITHUB_TOFU_RELEASE_ENV \
  --plain -p "${DOPPLER_PROJECT_NAME}" -c "${DOPPLER_INFRA_CONFIG}" >/dev/null 2>&1; then
  echo "FAIL: infra-scoped token must not read GITHUB_TOFU_RELEASE_ENV from infra config" >&2
  exit 1
fi
echo "OK: infra-scoped token cannot read GITHUB_TOFU_RELEASE_ENV from infra"

echo "OK: infra-scoped Doppler service token denied on bootstrap-admin"
