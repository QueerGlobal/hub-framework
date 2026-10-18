# shellcheck shell=bash
# Account API Tokens Write cannot be proven with a GET. POST an invalid body instead:
# 400 or 422 means the token may create tokens, 401 or 403 means it may not.
# A 2xx means Cloudflare created a token, which preflight must never do, so it fails.

_CF_WRITE_PROBE_LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/lib/curl-bearer-auth.sh
source "${_CF_WRITE_PROBE_LIB_DIR}/curl-bearer-auth.sh"

cloudflare_account_token_write_probe() {
  local token=$1
  local account_id=$2
  local http_code
  http_code="$(curl_with_bearer_http_code "${token}" \
    -X POST \
    -H "Content-Type: application/json" \
    --data-binary '{"name":"","policies":[]}' \
    "https://api.cloudflare.com/client/v4/accounts/${account_id}/tokens" 2>/dev/null)" || http_code="000"
  case "${http_code}" in
    400|422)
      return 0
      ;;
    401|403)
      echo "Bootstrap token lacks Account API Tokens Write (token create probe returned HTTP ${http_code})." >&2
      echo "Grant Account API Tokens Write at account scope before running apply." >&2
      return 1
      ;;
    2??)
      echo "Account token create probe returned HTTP ${http_code}, so a token may have been created." >&2
      echo "Check the account's API tokens, delete any unnamed token, then retry." >&2
      return 1
      ;;
    *)
      echo "Account token create probe returned unexpected HTTP ${http_code}." >&2
      return 1
      ;;
  esac
}
