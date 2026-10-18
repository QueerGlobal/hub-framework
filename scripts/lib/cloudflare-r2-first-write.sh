# shellcheck shell=bash
# Probes whether the token can perform the first R2 write (create bucket). Preflight list cannot prove Write.

cloudflare_r2_first_write_allowed() {
  local token=$1
  local account_id=$2
  local bucket_name=$3
  local http_code body_file
  body_file="$(mktemp)"
  http_code="$(curl -sS -o "${body_file}" -w '%{http_code}' \
    -X POST \
    -H "Authorization: Bearer ${token}" \
    -H "Content-Type: application/json" \
    -d "{\"name\":\"${bucket_name}\"}" \
    "https://api.cloudflare.com/client/v4/accounts/${account_id}/r2/buckets" 2>/dev/null || echo "000")"
  rm -f "${body_file}"
  if [[ "${http_code}" =~ ^2 ]]; then
    return 0
  fi
  return 1
}
