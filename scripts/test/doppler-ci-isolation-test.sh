#!/usr/bin/env bash
# CI must not read bootstrap-admin; infra-scoped service token is denied there.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
INFRA_YML="${ROOT}/.github/workflows/infra.yml"

if grep -qE 'DOPPLER_BOOTSTRAP|TOFU_BOOTSTRAP_STATE|-c[[:space:]]+bootstrap-admin|config:[[:space:]]+bootstrap-admin' "${INFRA_YML}"; then
  echo "FAIL: infra workflow references bootstrap-admin or bootstrap state secrets" >&2
  exit 1
fi

if grep -qE 'AWS_ACCESS_KEY_ID|AWS_SECRET_ACCESS_KEY' "${INFRA_YML}"; then
  echo "FAIL: infra workflow must not inject R2/AWS credentials" >&2
  exit 1
fi

if ! grep -q 'uses: dopplerhq/cli-action@' "${INFRA_YML}"; then
  echo "FAIL: infra workflow must install the Doppler CLI before the bootstrap-admin guard" >&2
  exit 1
fi

if ! grep -q 'scripts/doppler-ci-guard.sh' "${INFRA_YML}"; then
  echo "FAIL: infra workflow must run scripts/doppler-ci-guard.sh" >&2
  exit 1
fi

"${ROOT}/scripts/test/doppler-ci-runtime-guard-test.sh"
"${ROOT}/scripts/test/doppler-ci-scope-test.sh"

echo "OK: CI workflow and infra-scoped Doppler token cannot read bootstrap-admin"
