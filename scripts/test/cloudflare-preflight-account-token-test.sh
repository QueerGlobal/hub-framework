#!/usr/bin/env bash
# Account-owned tokens: user verify fails, account verify succeeds, account token details by ID.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=scripts/test/lib/cloudflare-preflight-mock.sh
source "${ROOT}/scripts/test/lib/cloudflare-preflight-mock.sh"

if ! cloudflare_preflight_run_preflight "${ROOT}" \
  CF_MOCK_USER_VERIFY_OK=0 \
  CF_MOCK_ACCOUNT_VERIFY_OK=1 \
  CF_MOCK_USER_DETAILS_OK=0 \
  CF_MOCK_ACCOUNT_DETAILS_OK=1 \
  CF_MOCK_DETAILS_FIXTURE=token-details-account-owned-allow.json; then
  echo "FAIL: account-owned mock token should pass preflight" >&2
  exit 1
fi

echo "OK: account-owned token passes user-then-account verify and capability checks"
