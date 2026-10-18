#!/usr/bin/env bash
# Negative test: catalog-only tokens must fail preflight grant checks.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
TESTDIR="$(mktemp -d)"
trap 'rm -rf "${TESTDIR}"' EXIT

FAKE_BIN="${TESTDIR}/bin"
mkdir -p "${FAKE_BIN}"

cat > "${FAKE_BIN}/curl" <<'EOF'
#!/usr/bin/env bash
url="${@: -1}"
if [[ "${url}" == *"/user/tokens/verify" ]]; then
  printf '%s\n' '{"success":true,"result":{"id":"restricted-token-id","status":"active"}}'
  exit 0
fi
if [[ "${url}" == *"/tokens/permission_groups" ]]; then
  printf '%s\n' '{"success":true,"result":[{"id":"1","name":"Workers R2 Storage Write"},{"id":"2","name":"Account API Tokens Write"}]}'
  exit 0
fi
if [[ "${url}" == *"/user/tokens/restricted-token-id" ]]; then
  printf '%s\n' '{"success":true,"result":{"id":"restricted-token-id","policies":[{"effect":"allow","permission_groups":[{"id":"2","name":"Account API Tokens Write"}]}]}}'
  exit 0
fi
echo "unexpected curl url: ${url}" >&2
exit 1
EOF
chmod +x "${FAKE_BIN}/curl"

export PATH="${FAKE_BIN}:${PATH}"

# shellcheck source=scripts/lib/preflight.sh
source "${ROOT}/scripts/lib/preflight.sh"

if preflight_cloudflare_api_token "restricted-mock-token" "00000000000000000000000000000001"; then
  echo "FAIL: restricted mock token should not pass preflight" >&2
  exit 1
fi

echo "OK: restricted mock token correctly rejected by preflight_cloudflare_api_token"
