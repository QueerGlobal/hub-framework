#!/usr/bin/env bash
# Mutation-proves each Cloudflare capability probe (break one probe, expect failure + reason).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CONFIGURED_ACCOUNT="00000000000000000000000000000001"
# shellcheck source=scripts/test/lib/cloudflare-preflight-mock.sh
source "${ROOT}/scripts/test/lib/cloudflare-preflight-mock.sh"

assert_fails_with() {
  local label=$1 needle=$2
  shift 2
  local out rc=0
  out="$(cloudflare_preflight_run_preflight "${ROOT}" "$@" 2>&1)" || rc=$?
  [[ $rc -ne 0 ]] || { echo "FAIL: ${label} should fail" >&2; exit 1; }
  grep -Fq "${needle}" <<<"${out}" || {
    echo "FAIL: ${label} missing reason: ${needle}" >&2
    echo "${out}" >&2
    exit 1
  }
  echo "OK: mutation ${label} rejected with expected reason"
}

assert_passes() {
  local label=$1
  shift
  local out rc=0
  out="$(cloudflare_preflight_run_preflight "${ROOT}" "$@" 2>&1)" || rc=$?
  [[ $rc -eq 0 ]] || {
    echo "FAIL: control case ${label} should pass preflight" >&2
    echo "${out}" >&2
    exit 1
  }
  echo "OK: control case ${label} passes"
}

BASE=(
  CF_MOCK_ACCOUNT_VERIFY_OK=1
  CF_MOCK_VERIFY_STATUS=active
)

assert_passes "account verify and capability probes" "${BASE[@]}"

assert_fails_with "account verify fails" \
  "Bootstrap requires an account-owned Cloudflare API token (account verify failed)." \
  "${BASE[@]}" CF_MOCK_ACCOUNT_VERIFY_OK=0

assert_fails_with "user token (account verify 404)" \
  "Bootstrap requires an account-owned Cloudflare API token (account verify failed)." \
  "${BASE[@]}" CF_MOCK_ACCOUNT_VERIFY_OK=0 CF_MOCK_ACCOUNT_VERIFY_HTTP=404

assert_fails_with "inactive token status" \
  "status is not active" \
  "${BASE[@]}" CF_MOCK_VERIFY_STATUS=disabled

assert_fails_with "R2 list denied with 403" \
  "R2 list buckets failed" \
  "${BASE[@]}" CF_MOCK_R2_OK=0

assert_fails_with "wrong account on token verify" \
  "not configured account ${CONFIGURED_ACCOUNT}" \
  "${BASE[@]}" CF_MOCK_VERIFY_ACCOUNT_ID=00000000000000000000000000000002

assert_fails_with "permission groups probe skipped" \
  "permission groups returned success=false" \
  "${BASE[@]}" CF_MOCK_CATALOG_OK=0

assert_fails_with "token create denied with 403" \
  "lacks Account API Tokens Write (token create probe returned HTTP 403)" \
  "${BASE[@]}" CF_MOCK_TOKEN_CREATE_HTTP=403

assert_fails_with "token create denied with 401" \
  "lacks Account API Tokens Write (token create probe returned HTTP 401)" \
  "${BASE[@]}" CF_MOCK_TOKEN_CREATE_HTTP=401

assert_fails_with "token create probe succeeded" \
  "returned HTTP 201, so a token may have been created" \
  "${BASE[@]}" CF_MOCK_TOKEN_CREATE_HTTP=201

assert_fails_with "token create probe call failed" \
  "Account token create probe returned unexpected HTTP 000" \
  "${BASE[@]}" CF_MOCK_OMIT_TOKEN_PROBE=1

echo "OK: Cloudflare preflight probe mutation checks passed"
