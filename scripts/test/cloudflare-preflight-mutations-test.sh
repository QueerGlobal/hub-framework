#!/usr/bin/env bash
# Mutation-proves each Cloudflare capability probe (break one probe, expect failure).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=scripts/test/lib/cloudflare-preflight-mock.sh
source "${ROOT}/scripts/test/lib/cloudflare-preflight-mock.sh"

assert_fails() {
  local label=$1
  shift
  if cloudflare_preflight_run_preflight "${ROOT}" "$@"; then
    echo "FAIL: mutation ${label} should have failed preflight" >&2
    exit 1
  fi
  echo "OK: mutation ${label} rejected as expected"
}

assert_passes() {
  local label=$1
  shift
  if ! cloudflare_preflight_run_preflight "${ROOT}" "$@"; then
    echo "FAIL: control case ${label} should pass preflight" >&2
    exit 1
  fi
  echo "OK: control case ${label} passes"
}

BASE=(
  CF_MOCK_ACCOUNT_VERIFY_OK=1
  CF_MOCK_USER_VERIFY_OK=0
  CF_MOCK_VERIFY_STATUS=active
)

assert_passes "account verify and capability probes" "${BASE[@]}"

assert_fails "both verify endpoints fail" \
  "${BASE[@]}" CF_MOCK_ACCOUNT_VERIFY_OK=0 CF_MOCK_USER_VERIFY_OK=0

assert_fails "inactive token status" \
  "${BASE[@]}" CF_MOCK_VERIFY_STATUS=disabled

assert_fails "R2 list probe skipped" \
  "${BASE[@]}" CF_MOCK_R2_OK=0

assert_fails "permission groups probe skipped" \
  "${BASE[@]}" CF_MOCK_CATALOG_OK=0

echo "OK: Cloudflare preflight probe mutation checks passed"
