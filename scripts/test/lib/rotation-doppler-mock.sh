# shellcheck shell=bash
# In-memory Doppler mock for encryption rotation tests.
# Like the real CLI, "secrets get" exits 0 with an empty value for an absent secret only when
# --no-exit-on-missing-secret is passed. ROTATION_DOPPLER_FAIL_GET lists secret names whose
# reads fail with an auth error whose text says "not found", to prove errors are never read as missing.
# Values read from a file with newlines (state backups) are stored as sha256:<hex> of the bytes. Argv goes to ARGV_SHIM_LOG when set.

rotation_doppler_mock_install() {
  local fake_bin=$1
  local db=$2
  local shim_log_lib
  shim_log_lib="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/argv-shim-log.sh"
  mkdir -p "${fake_bin}"
  cat > "${fake_bin}/doppler" <<MOCK
#!/usr/bin/bash
set -euo pipefail
# shellcheck source=scripts/test/lib/argv-shim-log.sh
source "${shim_log_lib}"
argv_shim_log_invocation doppler "\$@"
db="\${ROTATION_DOPPLER_DB:-${db}}"
MOCK
  cat >> "${fake_bin}/doppler" <<'MOCK'
parse_scope() {
  project=""
  config=""
  missing_ok=0
  while [[ $# -gt 0 ]]; do
    case "$1" in
      -p|--project) project=$2; shift 2 ;;
      -c|--config) config=$2; shift 2 ;;
      --no-exit-on-missing-secret) missing_ok=1; shift ;;
      *) shift ;;
    esac
  done
}

[[ "${1:-}" == secrets ]] || { echo "unexpected doppler: $*" >&2; exit 1; }
sub=${2:-}
name=${3:-}
shift 3
parse_scope "$@"
case "${sub}" in
  get)
    if [[ " ${ROTATION_DOPPLER_FAIL_GET:-} " == *" ${name} "* ]]; then
      echo "Doppler Error: Unable to authenticate: token not found (HTTP 404 from proxy)" >&2
      exit 1
    fi
    found=0
    value=""
    while IFS=$'\t' read -r _ n p c v; do
      if [[ "${n}" == "${name}" && "${p}" == "${project}" && "${c}" == "${config}" ]]; then
        found=1
        value=${v}
      fi
    done <"${db}"
    if [[ ${found} -eq 0 && ${missing_ok} -eq 0 ]]; then
      echo "Doppler Error: Could not find requested secret: ${name}" >&2
      exit 1
    fi
    printf '%s\n' "${value}"
    ;;
  set)
    raw="$(mktemp)"
    cat >"${raw}"
    if [[ "$(wc -l <"${raw}")" -gt 0 ]]; then
      value="sha256:$(sha256sum <"${raw}" | cut -d' ' -f1)"
    else
      value="$(cat "${raw}")"
    fi
    rm -f "${raw}"
    awk -F '\t' -v n="${name}" -v p="${project}" -v c="${config}" \
      '!( $2 == n && $3 == p && $4 == c )' "${db}" >"${db}.tmp"
    mv "${db}.tmp" "${db}"
    printf '%s\t%s\t%s\t%s\t%s\n' "$(date +%s)" "${name}" "${project}" "${config}" "${value}" >>"${db}"
    printf 'set %s %s/%s\n' "${name}" "${project}" "${config}" >>"${db}.writes"
    ;;
  delete)
    awk -F '\t' -v n="${name}" -v p="${project}" -v c="${config}" \
      '!( $2 == n && $3 == p && $4 == c )' "${db}" >"${db}.tmp"
    mv "${db}.tmp" "${db}"
    printf 'delete %s %s/%s\n' "${name}" "${project}" "${config}" >>"${db}.writes"
    ;;
  *)
    echo "unexpected doppler: secrets ${sub} ${name}" >&2
    exit 1
    ;;
esac
MOCK
  chmod +x "${fake_bin}/doppler"
}

rotation_doppler_seed() {
  local db=$1 name=$2 project=$3 config=$4 value=$5
  printf '%s\t%s\t%s\t%s\t%s\n' 0 "${name}" "${project}" "${config}" "${value}" >>"${db}"
}

# Prints the stored value of one secret (empty when absent).
rotation_doppler_value() {
  local db=$1 name=$2
  awk -F '\t' -v n="${name}" '$2 == n { v = $5 } END { print v }' "${db}"
}
