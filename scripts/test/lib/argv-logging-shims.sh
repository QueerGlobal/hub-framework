# shellcheck shell=bash
# Install PATH shims that log full argv for selected tools.

ARGV_SHIM_DEFAULT_TOOLS=(curl doppler bash gh tofu jq)

argv_shim_install() {
  local fake_bin=$1
  shift 2
  local shim_log_lib
  shim_log_lib="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/argv-shim-log.sh"
  local tools=("$@")
  if [[ ${#tools[@]} -eq 0 ]]; then
    tools=("${ARGV_SHIM_DEFAULT_TOOLS[@]}")
  fi
  mkdir -p "${fake_bin}"
  local tool real_path
  for tool in "${tools[@]}"; do
    real_path="$(PATH="/usr/local/bin:/usr/bin:/bin" command -v "${tool}" 2>/dev/null || true)"
    if [[ -z "${real_path}" ]]; then
      echo "argv_shim_install: ${tool} is not installed in /usr/local/bin, /usr/bin or /bin" >&2
      return 1
    fi
    cat > "${fake_bin}/${tool}" <<SHIM
#!/usr/bin/bash
set -euo pipefail
# shellcheck source=scripts/test/lib/argv-shim-log.sh
source "${shim_log_lib}"
argv_shim_log_invocation "${tool}" "\$@"
if [[ "${tool}" == "tofu" && "\${ARGV_SHIM_TOFU_TEST_MODE:-}" == "1" ]]; then
  case "\${1:-}" in
    init)
      if [[ "\$*" == *-backend-config=* ]]; then
        "${real_path}" init -input=false -backend=false
        exit \$?
      fi
      ;;
    plan)
      "${real_path}" test
      exit \$?
      ;;
    apply)
      "${real_path}" test
      if [[ -f backend_local.tf ]]; then
        printf '%s\n' '{"version":4,"serial":1,"lineage":"argv-shim","outputs":{},"resources":[]}' > terraform.tfstate
      fi
      exit 0
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

# Prints the shimmed tool whose argv carried the secret and returns 0 when a leak is found.
# Never prints the secret itself.
argv_shim_find_leak() {
  local log=$1
  local secret=$2
  local line
  line="$(grep -F -m1 -- "${secret}" "${log}" || true)"
  if [[ -z "${line}" ]]; then
    return 1
  fi
  line="${line#SHIM:}"
  printf '%s\n' "${line%% *}"
}

# Fails with a reason naming the segment and the tool when any secret reached argv.
argv_shim_assert_no_secrets() {
  local log=$1
  local segment=$2
  shift 2
  local secret tool
  for secret in "$@"; do
    if tool="$(argv_shim_find_leak "${log}" "${secret}")"; then
      echo "FAIL: ${segment}: a secret reached the argv of ${tool}" >&2
      return 1
    fi
  done
}
