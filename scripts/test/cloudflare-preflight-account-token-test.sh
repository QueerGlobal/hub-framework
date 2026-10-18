#!/usr/bin/env bash
# Account-owned tokens use GET /accounts/{account_id}/tokens/{id} when user token GET fails.
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
  printf '%s\n' '{"success":true,"result":{"id":"account-owned-token-id","status":"active"}}'
  exit 0
fi
if [[ "${url}" == *"/tokens/permission_groups" ]]; then
  printf '%s\n' '{"success":true,"result":[{"id":"1","name":"Workers R2 Storage Write"},{"id":"2","name":"Account API Tokens Write"}]}'
  exit 0
fi
if [[ "${url}" == *"/user/tokens/account-owned-token-id" ]]; then
  echo "user token details forbidden" >&2
  exit 403
fi
if [[ "${url}" == *"/accounts/00000000000000000000000000000001/tokens/account-owned-token-id" ]]; then
  printf '%s\n' '{"success":true,"result":{"id":"account-owned-token-id","policies":[{"effect":"allow","permission_groups":[{"id":"1","name":"Workers R2 Storage Write"},{"id":"2","name":"Account API Tokens Write"}]}]}}'
  exit 0
fi
echo "unexpected curl url: ${url}" >&2
exit 1
EOF
chmod +x "${FAKE_BIN}/curl"

export PATH="${FAKE_BIN}:${PATH}"

# shellcheck source=scripts/lib/preflight.sh
source "${ROOT}/scripts/lib/preflight.sh"

if ! preflight_cloudflare_api_token "account-owned-mock-token" "00000000000000000000000000000001"; then
  echo "FAIL: account-owned mock token should pass preflight via account token endpoint" >&2
  exit 1
fi

echo "OK: account-owned mock token accepted via GET /accounts/{account_id}/tokens/{id}"
