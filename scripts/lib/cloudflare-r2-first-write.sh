# shellcheck shell=bash
# Probes whether the token can perform the first R2 write (create bucket). Preflight list cannot prove Write.

_LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/lib/curl-bearer-auth.sh
source "${_LIB_DIR}/curl-bearer-auth.sh"

cloudflare_r2_first_write_allowed() {
  local token=$1
  local account_id=$2
  local bucket_name=$3
  local body_file hdr_file http_code
  body_file="$(mktemp)"
  hdr_file="$(curl_bearer_header_file "${token}")"
  chmod 0600 "${body_file}"
  printf '{"name":"%s"}' "${bucket_name}" >"${body_file}"
  # shellcheck disable=SC2064
  trap "rm -f '${body_file}' '${hdr_file}'" RETURN
  http_code="$(curl -sS -o /dev/null -w '%{http_code}' \
    -X POST \
    -H "Content-Type: application/json" \
    -H @"${hdr_file}" \
    --data-binary @"${body_file}" \
    "https://api.cloudflare.com/client/v4/accounts/${account_id}/r2/buckets" 2>/dev/null || echo "000")"
  if [[ "${http_code}" =~ ^2 ]]; then
    return 0
  fi
  return 1
}
