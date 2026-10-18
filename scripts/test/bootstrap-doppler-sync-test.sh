#!/usr/bin/env bash
# An absent, unreadable or empty Doppler secret must count as missing, so bootstrap regenerates or refuses it.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
WORKDIR="$(mktemp -d)"
trap 'rm -rf "${WORKDIR}"' EXIT
mkdir -p "${WORKDIR}/bin"
cat >"${WORKDIR}/bin/doppler" <<'MOCK'
#!/usr/bin/bash
case "${MOCK_SECRET_STATE:?}" in
  present) printf '%s' 'stored-value-0123456789' ;;
  empty) ;;
  absent) echo "Doppler Error: Could not find requested secret" >&2; exit 1 ;;
esac
MOCK
chmod +x "${WORKDIR}/bin/doppler"
export PATH="${WORKDIR}/bin:${PATH}"
# shellcheck source=scripts/lib/bootstrap-doppler-sync.sh
source "${ROOT}/scripts/lib/bootstrap-doppler-sync.sh"

expect_missing() {
  local state=$1 want=$2 got=present
  if MOCK_SECRET_STATE="${state}" bootstrap_doppler_secret_missing NAME hub-framework infra; then
    got=missing
  fi
  if [[ "${got}" != "${want}" ]]; then
    echo "FAIL: Doppler secret state ${state} was treated as ${got}, expected ${want}" >&2
    exit 1
  fi
  echo "OK: Doppler secret state ${state} counts as ${want}"
}

expect_missing absent missing
expect_missing empty missing
expect_missing present present
echo "OK: bootstrap Doppler secret checks treat empty values as missing"
