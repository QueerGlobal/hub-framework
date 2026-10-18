#!/usr/bin/env bash
# Ensures S3 backend switching uses gitignored override files and leaves tracked files clean.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
STACK="${ROOT}/infra/opentofu/state-bootstrap"

# shellcheck source=scripts/lib/bootstrap-backend-mode.sh
source "${ROOT}/scripts/lib/bootstrap-backend-mode.sh"

cd "${STACK}"
orig_versions_hash="$(sha256sum versions.tf | awk '{print $1}')"
orig_local_hash="$(sha256sum backend_local.tf | awk '{print $1}')"

bootstrap_backend_enable_s3 "${STACK}"
bootstrap_backend_disable_s3 "${STACK}"

if [[ "$(sha256sum versions.tf | awk '{print $1}')" != "${orig_versions_hash}" ]]; then
  echo "FAIL: versions.tf changed after backend switch" >&2
  exit 1
fi

if [[ "$(sha256sum backend_local.tf | awk '{print $1}')" != "${orig_local_hash}" ]]; then
  echo "FAIL: backend_local.tf changed after backend switch" >&2
  exit 1
fi

if [[ -f backend_override.tf || -f .bootstrap_backend_local.tf ]]; then
  echo "FAIL: gitignored backend files left behind after disable" >&2
  exit 1
fi

if ! git diff --quiet -- versions.tf backend_local.tf; then
  echo "FAIL: tracked backend files differ from git after restore" >&2
  git diff -- versions.tf backend_local.tf >&2 || true
  exit 1
fi

echo "OK: backend_override.tf approach leaves a clean tree"
