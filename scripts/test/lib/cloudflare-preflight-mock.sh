# shellcheck shell=bash
# Install a mock curl for Cloudflare preflight tests.

cloudflare_preflight_mock_install() {
  local fake_bin=$1
  mkdir -p "${fake_bin}"

  cat > "${fake_bin}/curl" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail
url="${@: -1}"
fixture_root="${CF_PREFLIGHT_FIXTURE_ROOT:?}"

r2_ok=${CF_MOCK_R2_OK:-1}
tokens_list_ok=${CF_MOCK_TOKENS_LIST_OK:-1}
user_verify_ok=${CF_MOCK_USER_VERIFY_OK:-1}
account_verify_ok=${CF_MOCK_ACCOUNT_VERIFY_OK:-1}
user_details_ok=${CF_MOCK_USER_DETAILS_OK:-0}
account_details_ok=${CF_MOCK_ACCOUNT_DETAILS_OK:-1}
catalog_ok=${CF_MOCK_CATALOG_OK:-1}
details_fixture=${CF_MOCK_DETAILS_FIXTURE:-token-details-account-owned-allow.json}

if [[ "${url}" == *"/user/tokens/verify" ]]; then
  if [[ "${user_verify_ok}" == 1 ]]; then
    printf '%s\n' '{"success":true,"errors":[],"messages":[],"result":{"id":"user-owned-token-id","status":"active"}}'
    exit 0
  fi
  printf '%s\n' '{"success":false,"errors":[{"code":1000,"message":"Invalid request for user-owned token verify"}],"messages":[],"result":null}'
  exit 0
fi

if [[ "${url}" == *"/accounts/"*"/tokens/verify" ]]; then
  if [[ "${account_verify_ok}" == 1 ]]; then
    printf '%s\n' '{"success":true,"errors":[],"messages":[],"result":{"id":"account-owned-token-id","status":"active"}}'
    exit 0
  fi
  printf '%s\n' '{"success":false,"errors":[{"code":1000,"message":"Invalid account token"}],"messages":[],"result":null}'
  exit 0
fi

if [[ "${url}" == *"/r2/buckets"* ]]; then
  if [[ "${r2_ok}" == 1 ]]; then
    printf '%s\n' '{"success":true,"errors":[],"messages":[],"result":{"buckets":[],"count":0}}'
    exit 0
  fi
  echo "forbidden" >&2
  exit 403
fi

if [[ "${url}" == *"/accounts/"*"/tokens?"* ]]; then
  if [[ "${tokens_list_ok}" == 1 ]]; then
    printf '%s\n' '{"success":true,"errors":[],"messages":[],"result":[{"id":"existing-token-id","name":"existing","status":"active"}],"result_info":{"page":1,"per_page":1,"count":1,"total_count":1}}'
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

if [[ "${url}" == *"/user/tokens/user-owned-token-id" ]]; then
  if [[ "${user_details_ok}" == 1 ]]; then
    cat "${fixture_root}/${details_fixture}"
    exit 0
  fi
  echo "user token details forbidden" >&2
  exit 403
fi

if [[ "${url}" == *"/accounts/"*"/tokens/account-owned-token-id" ]]; then
  if [[ "${account_details_ok}" == 1 ]]; then
    cat "${fixture_root}/${details_fixture}"
    exit 0
  fi
  echo "account token details forbidden" >&2
  exit 403
fi

echo "unexpected curl url: ${url}" >&2
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
  (
    export PATH="${fake_bin}:${PATH}"
    for kv in "${env_exports[@]}"; do
      export "${kv?}"
    done
    source "${root}/scripts/lib/preflight.sh"
    preflight_cloudflare_api_token "mock-token" "00000000000000000000000000000001"
  )
  local rc=$?
  rm -rf "${testdir}"
  return "${rc}"
}
