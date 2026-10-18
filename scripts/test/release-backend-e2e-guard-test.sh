#!/usr/bin/env bash
# Release local backend override is allowed only when BOOTSTRAP_E2E=1.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
BOOTSTRAP="${ROOT}/scripts/bootstrap-release-infra.sh"
RELEASE_SRC="${ROOT}/infra/opentofu/release-environment"
# shellcheck source=scripts/lib/release-backend-mode.sh
source "${ROOT}/scripts/lib/release-backend-mode.sh"

assert_release_backend_enable_local_guarded() {
  local file=$1
  local call_line guard_line
  call_line="$(grep -n 'release_backend_enable_local' "${file}" | head -1 | cut -d: -f1)" || return 0
  guard_line="$(grep -n 'if \[\[ "\${BOOTSTRAP_E2E:-}" == "1" \]\]; then' "${file}" \
    | awk -F: -v call="${call_line}" '$1 < call { last=$1 } END { print last }')"
  if [[ -z "${guard_line}" || "${guard_line}" -ge "${call_line}" ]]; then
    echo "FAIL: release_backend_enable_local is not guarded by BOOTSTRAP_E2E in ${file}" >&2
    return 1
  fi
  if ! sed -n "${guard_line},$((call_line + 20))p" "${file}" | grep -q '^fi$'; then
    echo "FAIL: BOOTSTRAP_E2E block missing fi after release_backend_enable_local in ${file}" >&2
    return 1
  fi
  return 0
}

if ! assert_release_backend_enable_local_guarded "${BOOTSTRAP}"; then
  exit 1
fi
echo "OK: baseline bootstrap script guards release_backend_enable_local with BOOTSTRAP_E2E"

MUTATED="$(mktemp)"
WORKDIR="$(mktemp -d)"
trap 'rm -f "${MUTATED}"; rm -rf "${WORKDIR}"' EXIT

printf '%s\n' 'release_backend_enable_local "${RELEASE_DIR}"' >"${MUTATED}"
if assert_release_backend_enable_local_guarded "${MUTATED}" 2>/dev/null; then
  echo "FAIL: unguarded release_backend_enable_local should fail static check" >&2
  exit 1
fi
echo "OK: mutation without BOOTSTRAP_E2E guard fails static check"

cp -a "${RELEASE_SRC}/." "${WORKDIR}/"
if [[ ! -f "${WORKDIR}/backend_s3.tf" ]]; then
  echo "FAIL: fixture release stack missing backend_s3.tf" >&2
  exit 1
fi

unset BOOTSTRAP_E2E
release_backend_e2e_init_dry_run "${WORKDIR}"
if [[ -f "${WORKDIR}/backend_override.tf" ]]; then
  echo "FAIL: dry-run with BOOTSTRAP_E2E unset created backend_override.tf" >&2
  exit 1
fi
if [[ ! -f "${WORKDIR}/backend_s3.tf" ]]; then
  echo "FAIL: dry-run with BOOTSTRAP_E2E unset hid backend_s3.tf" >&2
  exit 1
fi
echo "OK: dry-run with BOOTSTRAP_E2E unset leaves release backend files unchanged"

export BOOTSTRAP_E2E=1
release_backend_e2e_init_dry_run "${WORKDIR}"
if [[ ! -f "${WORKDIR}/backend_override.tf" ]]; then
  echo "FAIL: E2E dry-run should create backend_override.tf" >&2
  exit 1
fi
echo "OK: release backend E2E guard checks passed"
