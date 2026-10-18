#!/usr/bin/env bash
# Ensures bootstrap backend switching restores tracked versions.tf (clean git tree).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
STACK="${ROOT}/infra/opentofu/state-bootstrap"

# shellcheck source=scripts/lib/bootstrap-backend-mode.sh
source "${ROOT}/scripts/lib/bootstrap-backend-mode.sh"

cd "${STACK}"
orig_hash="$(sha256sum versions.tf | awk '{print $1}')"

bootstrap_versions_with_s3_backend "${STACK}"
bootstrap_versions_restore "${STACK}"

restored_hash="$(sha256sum versions.tf | awk '{print $1}')"
if [[ "${orig_hash}" != "${restored_hash}" ]]; then
  echo "FAIL: versions.tf was not restored after S3 backend switch" >&2
  exit 1
fi

if ! git diff --quiet -- versions.tf; then
  echo "FAIL: versions.tf differs from git after restore" >&2
  git diff -- versions.tf >&2 || true
  exit 1
fi

echo "OK: bootstrap versions.tf restore leaves a clean tree"
