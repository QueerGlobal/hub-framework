#!/usr/bin/env bash
# R2 list can succeed without Write; first bucket create must fail closed before state changes.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=scripts/test/lib/cloudflare-preflight-mock.sh
source "${ROOT}/scripts/test/lib/cloudflare-preflight-mock.sh"
# shellcheck source=scripts/lib/cloudflare-r2-first-write.sh
source "${ROOT}/scripts/lib/cloudflare-r2-first-write.sh"

TESTDIR="$(mktemp -d)"
trap 'rm -rf "${TESTDIR}"' EXIT
FAKE_BIN="${TESTDIR}/bin"
export CF_PREFLIGHT_FIXTURE_ROOT="${ROOT}/scripts/test/fixtures/cloudflare"
export CF_MOCK_R2_OK=1
export CF_MOCK_R2_WRITE_OK=0
cloudflare_preflight_mock_install "${FAKE_BIN}"
export PATH="${FAKE_BIN}:${PATH}"

if ! cloudflare_preflight_run_preflight "${ROOT}" \
  CF_MOCK_ACCOUNT_VERIFY_OK=1 CF_MOCK_USER_VERIFY_OK=0 CF_MOCK_VERIFY_STATUS=active; then
  echo "FAIL: preflight should pass when list works but write is denied" >&2
  exit 1
fi

if cloudflare_r2_first_write_allowed "mock-token" "00000000000000000000000000000001" "probe-bucket-denied"; then
  echo "FAIL: first R2 write probe should fail when token lacks Write" >&2
  exit 1
fi

echo "OK: R2 list preflight passes but first write probe fails closed (apply would not mutate resources)"
