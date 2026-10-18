#!/usr/bin/env bash
# Account-owned tokens: account verify succeeds, user verify not required.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=scripts/test/lib/cloudflare-preflight-mock.sh
source "${ROOT}/scripts/test/lib/cloudflare-preflight-mock.sh"

if ! cloudflare_preflight_run_preflight "${ROOT}" \
  CF_MOCK_USER_VERIFY_OK=0 \
  CF_MOCK_ACCOUNT_VERIFY_OK=1 \
  CF_MOCK_VERIFY_STATUS=active; then
  echo "FAIL: account-owned mock token should pass capability preflight" >&2
  exit 1
fi

echo "OK: account verify-first path passes with capability probes only"
