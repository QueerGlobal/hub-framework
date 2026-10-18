# shellcheck shell=bash
# Install a mock curl for Cloudflare preflight tests.

cloudflare_preflight_mock_install() {
  local fake_bin=$1
  local shim_log_lib
  shim_log_lib="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/argv-shim-log.sh"
  mkdir -p "${fake_bin}"

  cat > "${fake_bin}/curl" <<MOCK
#!/usr/bin/bash
set -euo pipefail
# shellcheck source=scripts/test/lib/argv-shim-log.sh
source "${shim_log_lib}"
argv_shim_log_invocation curl "\$@"
MOCK
  cat >> "${fake_bin}/curl" <<'MOCK'
url=""
method="GET"
args=("$@")
idx=0
want_http_code=0
has_bearer=0
while [[ ${idx} -lt ${#args[@]} ]]; do
  case "${args[$idx]}" in
    -X)
      method="${args[$((idx + 1))]:-GET}"
      idx=$((idx + 2))
      ;;
    -H)
      header="${args[$((idx + 1))]:-}"
      if [[ "${header}" == "@-" ]]; then
        header="$(cat)"
      fi
      if [[ "${header}" == "Authorization: Bearer "?* ]]; then
        has_bearer=1
      fi
      idx=$((idx + 2))
      ;;
    -o)
      idx=$((idx + 2))
      ;;
    -w)
      if [[ "${args[$((idx + 1))]:-}" == *http_code* ]]; then
        want_http_code=1
      fi
      idx=$((idx + 2))
      ;;
    --data-binary)
      idx=$((idx + 2))
      ;;
    http*|/*)
      url="${args[$idx]}"
      idx=$((idx + 1))
      ;;
    *)
      idx=$((idx + 1))
      ;;
  esac
done
if [[ -z "${url}" ]]; then
  url="${@: -1}"
fi
if [[ "${has_bearer}" != 1 ]]; then
  echo "curl mock: request to ${url} carried no bearer token" >&2
  exit 1
fi
if [[ -n "${E2E_CURL_EVENT_LOG:-}" ]]; then
  printf '%s %s\n' "${method}" "${url}" >> "${E2E_CURL_EVENT_LOG}"
fi
fixture_root="${CF_PREFLIGHT_FIXTURE_ROOT:?}"

r2_list_ok=${CF_MOCK_R2_OK:-1}
account_verify_ok=${CF_MOCK_ACCOUNT_VERIFY_OK:-1}
token_create_http=${CF_MOCK_TOKEN_CREATE_HTTP:-400}
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

if [[ "${method}" == "POST" && "${url}" == *"/accounts/"*"/tokens" && "${url}" != *"/tokens/verify"* ]]; then
  if [[ "${CF_MOCK_OMIT_TOKEN_PROBE:-}" == "1" ]]; then
    echo "unexpected curl: token create probe disabled in mock" >&2
    exit 1
  fi
  if [[ "${want_http_code}" == 1 ]]; then
    printf '%s' "${token_create_http}"
    exit 0
  fi
  echo "token create probe must request the HTTP status code" >&2
  exit 1
fi

if [[ "${url}" == *"api.github.com/user"* ]]; then
  if [[ "${GITHUB_MOCK_USER_OK:-1}" == 1 ]]; then
    printf '%s\n' '{"login":"mock-github-user"}'
    exit 0
  fi
  printf '%s\n' '{"message":"Bad credentials"}'
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
      preflight_cloudflare_bootstrap_api_token "mock-token" "00000000000000000000000000000001"
    ) 2>&1
  )" || rc=$?
  rm -rf "${testdir}"
  printf '%s' "${out}"
  return "${rc}"
}
