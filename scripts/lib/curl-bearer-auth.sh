# shellcheck shell=bash
# Bearer tokens go to curl on stdin (-H @-), never on argv and never on disk.
# Callers must not send a request body on stdin; pass a non-secret body with --data-binary.

curl_with_bearer() {
  local token=$1
  shift
  printf 'Authorization: Bearer %s\n' "${token}" | curl -fsS -H @- "$@"
}

# Prints only the HTTP status code so the caller can branch on it.
curl_with_bearer_http_code() {
  local token=$1
  shift
  printf 'Authorization: Bearer %s\n' "${token}" \
    | curl -sS -o /dev/null -w '%{http_code}' -H @- "$@"
}
