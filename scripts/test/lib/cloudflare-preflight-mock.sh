# shellcheck shell=bash
# Install a mock curl for Cloudflare preflight tests.

cloudflare_preflight_mock_install() {
  local fake_bin=$1
  mkdir -p "${fake_bin}"

  cat > "${fake_bin}/curl" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail
url="${@: -1}"
method="GET"
if [[ "${1:-}" == "-X" ]]; then
  method="${2:-GET}"
fi
fixture_root="${CF_PREFLIGHT_FIXTURE_ROOT:?}"

r2_list_ok=${CF_MOCK_R2_OK:-1}
r2_write_ok=${CF_MOCK_R2_WRITE_OK:-1}
user_verify_ok=${CF_MOCK_USER_VERIFY_OK:-1}
account_verify_ok=${CF_MOCK_ACCOUNT_VERIFY_OK:-1}
account_verify_http=${CF_MOCK_ACCOUNT_VERIFY_HTTP:-}
catalog_ok=${CF_MOCK_CATALOG_OK:-1}
verify_status=${CF_MOCK_VERIFY_STATUS:-active}
verify_account_id=${CF_MOCK_VERIFY_ACCOUNT_ID:-}

verify_body() {
  local id=$1
  if [[ -n "${verify_account_id}" ]]; then
    printf '%s\n' "{\"success\":true,\"errors\":[],\"messages\":[],\"result\":{\"id\":\"${id}\",\"status\":\"${verify_status}\",\"account\":{\"id\":\"${verify_account_id}\"}}}"
  else
    printf '%s\n' "{\"success\":true,\"errors\":[],\"messages\":[],\"result\":{\"id\":\"${id}\",\"status\":\"${verify_status}\"}}"
  fi
}

if [[ "${url}" == *"/accounts/"*"/tokens/verify" ]]; then
  if [[ "${account_verify_ok}" == 1 ]]; then
    verify_body "account-owned-token-id"
    exit 0
  fi
  if [[ "${account_verify_http}" == "404" ]]; then
    echo "not found" >&2
    exit 404
  fi
  printf '%s\n' '{"success":false,"errors":[{"code":1000,"message":"Invalid account token"}],"messages":[],"result":null}'
  exit 0
fi

if [[ "${url}" == *"/user/tokens/verify" ]]; then
  if [[ "${user_verify_ok}" == 1 ]]; then
    verify_body "user-owned-token-id"
    exit 0
  fi
  printf '%s\n' '{"success":false,"errors":[{"code":1000,"message":"Invalid user token"}],"messages":[],"result":null}'
  exit 0
fi

if [[ "${method}" == "POST" && "${url}" == *"/r2/buckets" && "${url}" != *"?"* ]]; then
  if [[ "${r2_write_ok}" == 1 ]]; then
    printf '%s\n' '{"success":true,"errors":[],"messages":[],"result":{}}'
    exit 0
  fi
  echo "forbidden" >&2
  exit 403
fi

if [[ "${url}" == *"/r2/buckets"* ]]; then
  if [[ "${r2_list_ok}" == 1 ]]; then
    printf '%s\n' '{"success":true,"errors":[],"messages":[],"result":{"buckets":[],"count":0}}'
    exit 0
  fi
  echo "forbidden" >&2
  exit 403
fi

if [[ "${url}" == *"/tokens/permission_groups"* ]]; then
  if [[ "${catalog_ok}" == 1 ]]; then
    cat "${fixture_root}/catalog-permission-groups.json"
    exit 0
  fi
  printf '%s\n' '{"success":false,"errors":[{"code":1000,"message":"catalog denied"}],"messages":[],"result":null}'
  exit 0
fi

echo "unexpected curl: ${method} ${url}" >&2
exit 1
MOCK
  chmod +x "${fake_bin}/curl"
}

cloudflare_preflight_run_preflight() {
  local root=$1
  shift
  local -a env_exports=("$@")

  local testdir fake_bin
  testdir="$(mktemp -d)"
  fake_bin="${testdir}/bin"
  export CF_PREFLIGHT_FIXTURE_ROOT="${root}/scripts/test/fixtures/cloudflare"
  cloudflare_preflight_mock_install "${fake_bin}"

  # shellcheck source=scripts/lib/preflight.sh
  local rc=0 out
  out="$(
    (
      export PATH="${fake_bin}:${PATH}"
      for kv in "${env_exports[@]}"; do
        export "${kv?}"
      done
      source "${root}/scripts/lib/preflight.sh"
      preflight_cloudflare_api_token "mock-token" "00000000000000000000000000000001"
    ) 2>&1
  )" || rc=$?
  rm -rf "${testdir}"
  printf '%s' "${out}"
  return "${rc}"
}
