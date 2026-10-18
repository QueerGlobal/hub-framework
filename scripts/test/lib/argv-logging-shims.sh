# shellcheck shell=bash
# Install PATH shims that log full argv for selected tools.

argv_shim_install() {
  local fake_bin=$1
  shift 2
  local tools=("$@")
  if [[ ${#tools[@]} -eq 0 ]]; then
    tools=(curl doppler bash gh tofu)
  fi
  mkdir -p "${fake_bin}"
  local tool real_path
  for tool in "${tools[@]}"; do
    real_path="$(PATH="/usr/local/bin:/usr/bin:/bin" command -v "${tool}" 2>/dev/null | while read -r p; do
      [[ "${p}" != "${fake_bin}/${tool}" ]] && echo "${p}" && break
    done)"
    real_path="${real_path:-${tool}}"
    cat > "${fake_bin}/${tool}" <<SHIM
#!/usr/bin/bash
set -euo pipefail
log="\${ARGV_SHIM_LOG:?}"
{
  printf 'SHIM:${tool} ARGV:'
  printf '%q ' "\$@"
  printf '\n'
  tr '\\0' ' ' < /proc/self/cmdline
  printf '\n'
} >> "\${log}"
if [[ "${tool}" == "tofu" && "\${ARGV_SHIM_TOFU_TEST_MODE:-}" == "1" ]]; then
  case "\${1:-}" in
    apply|plan)
      "${real_path}" test
      exit \$?
      ;;
    output)
      if [[ "\${ARGV_SHIM_TOFU_MOCK_OUTPUT:-}" == "1" ]]; then
        if [[ "\$*" == *state_bucket_name* ]]; then
          echo "queerglobal-hub-framework-tofu-state"
          exit 0
        fi
        if [[ "\$*" == *r2_s3_endpoint* ]]; then
          echo "https://00000000000000000000000000000001.r2.cloudflarestorage.com"
          exit 0
        fi
        if [[ "\$*" == *tofu_state_access_key_id* ]]; then
          echo "mock-access-key-id"
          exit 0
        fi
        if [[ "\$*" == *tofu_state_secret_access_key* ]]; then
          echo "mock-secret-access-key-value-0123456789abcdef"
          exit 0
        fi
      fi
      ;;
  esac
fi
exec "${real_path}" "\$@"
SHIM
    chmod +x "${fake_bin}/${tool}"
  done
}

argv_shim_assert_no_secrets() {
  local log=$1
  shift
  local secret
  for secret in "$@"; do
    if grep -F "${secret}" "${log}"; then
      echo "FAIL: secret value appeared in argv log" >&2
      return 1
    fi
  done
  return 0
}
