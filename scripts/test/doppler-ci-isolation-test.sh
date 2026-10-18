#!/usr/bin/env bash
# CI must not read bootstrap-admin; infra-scoped service token is denied there.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
INFRA_YML="${ROOT}/.github/workflows/infra.yml"

if grep -qE 'bootstrap-admin|DOPPLER_BOOTSTRAP|TOFU_BOOTSTRAP_STATE' "${INFRA_YML}"; then
  echo "FAIL: infra workflow references bootstrap-admin or bootstrap state secrets" >&2
  exit 1
fi

if grep -qE 'AWS_ACCESS_KEY_ID|AWS_SECRET_ACCESS_KEY' "${INFRA_YML}"; then
  echo "FAIL: infra workflow must not inject R2/AWS credentials" >&2
  exit 1
fi

if ! grep -q 'doppler-ci-runtime-guard.sh' "${INFRA_YML}"; then
  echo "FAIL: infra workflow must run doppler-ci-runtime-guard when DOPPLER_TOKEN is set" >&2
  exit 1
fi

"${ROOT}/scripts/test/doppler-ci-scope-test.sh"

echo "OK: CI workflow and infra-scoped Doppler token cannot read bootstrap-admin"
