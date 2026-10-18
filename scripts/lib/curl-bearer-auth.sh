# shellcheck shell=bash
# Bearer tokens on curl argv leak via ps; use a 0600 header file per invocation.

curl_bearer_header_file() {
  local token=$1
  local hdr_file
  hdr_file="$(mktemp)"
  chmod 0600 "${hdr_file}"
  printf 'Authorization: Bearer %s' "${token}" >"${hdr_file}"
  printf '%s' "${hdr_file}"
}

curl_with_bearer() {
  local token=$1
  shift
  local hdr_file
  hdr_file="$(curl_bearer_header_file "${token}")"
  # shellcheck disable=SC2064
  trap "rm -f '${hdr_file}'" RETURN
  curl -fsS -H @"${hdr_file}" "$@"
}

curl_with_bearer_capture_http() {
  local token=$1
  local url=$2
  local body_file=$3
  local hdr_file
  hdr_file="$(curl_bearer_header_file "${token}")"
  # shellcheck disable=SC2064
  trap "rm -f '${hdr_file}'" RETURN
  curl -sS -o "${body_file}" -w '%{http_code}' -H @"${hdr_file}" "${url}"
}
