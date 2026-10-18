# shellcheck shell=bash
# Account API Tokens Write cannot be proven with GET. POST an invalid body; 403 means missing Write.

_LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/lib/curl-bearer-auth.sh
source "${_LIB_DIR}/curl-bearer-auth.sh"

cloudflare_account_token_write_probe() {
  local token=$1
  local account_id=$2
  local body_file hdr_file http_code
  body_file="$(mktemp)"
  hdr_file="$(curl_bearer_header_file "${token}")"
  chmod 0600 "${body_file}"
  printf '%s' '{"name":"","policies":[]}' >"${body_file}"
  # shellcheck disable=SC2064
  trap "rm -f '${body_file}' '${hdr_file}'" RETURN
  http_code="$(curl -sS -o /dev/null \
    -X POST \
    -H "Content-Type: application/json" \
    -H @"${hdr_file}" \
    --data-binary @"${body_file}" \
    -w '%{http_code}' \
    "https://api.cloudflare.com/client/v4/accounts/${account_id}/tokens" 2>/dev/null || echo "000")"
  case "${http_code}" in
    403|401)
      echo "Bootstrap token lacks Account API Tokens Write (token create probe returned HTTP ${http_code})." >&2
      echo "Grant Account API Tokens Write at account scope before running apply." >&2
      return 1
      ;;
    400|422|200|201)
      return 0
      ;;
    *)
      echo "Account token create probe returned unexpected HTTP ${http_code}." >&2
      return 1
      ;;
  esac
}
