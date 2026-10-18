#!/usr/bin/env bash
# Negative test: token without R2 capability must fail preflight.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=scripts/test/lib/cloudflare-preflight-mock.sh
source "${ROOT}/scripts/test/lib/cloudflare-preflight-mock.sh"

if cloudflare_preflight_run_preflight "${ROOT}" \
  CF_MOCK_USER_VERIFY_OK=1 \
  CF_MOCK_USER_DETAILS_OK=1 \
  CF_MOCK_DETAILS_FIXTURE=token-details-missing-r2-allow.json \
  CF_MOCK_R2_OK=0; then
  echo "FAIL: token without R2 list capability should not pass preflight" >&2
  exit 1
fi

echo "OK: restricted mock token correctly rejected (missing R2 capability)"
