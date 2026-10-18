#!/usr/bin/env bash
# Release local backend override must not be invoked from production bootstrap.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
BOOTSTRAP="${ROOT}/scripts/bootstrap-release-infra.sh"
RELEASE_SRC="${ROOT}/infra/opentofu/release-environment"
# shellcheck source=scripts/lib/release-backend-mode.sh
source "${ROOT}/scripts/lib/release-backend-mode.sh"

if grep -q 'release_backend_enable_local' "${BOOTSTRAP}"; then
  echo "FAIL: bootstrap-release-infra.sh must not call release_backend_enable_local" >&2
  exit 1
fi
echo "OK: production bootstrap does not call release_backend_enable_local"

WORKDIR="$(mktemp -d)"
trap 'rm -rf "${WORKDIR}"' EXIT
cp -a "${RELEASE_SRC}/." "${WORKDIR}/"

if [[ -f "${WORKDIR}/backend_override.tf" ]]; then
  echo "FAIL: release stack copy must not include backend_override.tf" >&2
  exit 1
fi
release_backend_enable_local "${WORKDIR}"
if [[ ! -f "${WORKDIR}/backend_override.tf" ]]; then
  echo "FAIL: test helper release_backend_enable_local should create backend_override.tf" >&2
  exit 1
fi
release_backend_disable_local "${WORKDIR}"
if [[ -f "${WORKDIR}/backend_override.tf" ]] || [[ ! -f "${WORKDIR}/backend_s3.tf" ]]; then
  echo "FAIL: release_backend_disable_local should restore tracked backend files" >&2
  exit 1
fi
echo "OK: release backend E2E guard checks passed"
