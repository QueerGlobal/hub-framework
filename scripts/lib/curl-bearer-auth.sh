# shellcheck shell=bash
# Bearer tokens go to curl on stdin (-H @-), never on argv and never on disk.
# Callers must not send a request body on stdin; pass a non-secret body with --data-binary.
# Every call has a connect and a total timeout so a hung API cannot stall a run.

CURL_BEARER_TIMEOUT_ARGS=(--connect-timeout 10 --max-time 30)

curl_with_bearer() {
  local token=$1
  shift
  printf 'Authorization: Bearer %s\n' "${token}" | curl -fsS "${CURL_BEARER_TIMEOUT_ARGS[@]}" -H @- "$@"
}

# Prints only the HTTP status code so the caller can branch on it.
curl_with_bearer_http_code() {
  local token=$1
  shift
  printf 'Authorization: Bearer %s\n' "${token}" \
    | curl -sS "${CURL_BEARER_TIMEOUT_ARGS[@]}" -o /dev/null -w '%{http_code}' -H @- "$@"
}
