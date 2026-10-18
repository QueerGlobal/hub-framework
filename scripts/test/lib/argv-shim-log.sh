# shellcheck shell=bash
# Shared argv logging for PATH shims (curl mock, doppler mock, argv-logging-shims).

argv_shim_log_invocation() {
  local tool=$1
  shift
  if [[ -z "${ARGV_SHIM_LOG:-}" ]]; then
    return 0
  fi
  {
    printf 'SHIM:%s ARGV:' "${tool}"
    printf '%q ' "$@"
    printf '\n'
  } >> "${ARGV_SHIM_LOG}"
}
