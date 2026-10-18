#!/usr/bin/env bash
# Proves rotation passphrases never appear on any child process argv.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
FIXTURE="${ROOT}/infra/opentofu/encryption-rotation-fixture"
ROTATE="${ROOT}/scripts/tofu-rotate-encryption-passphrase.sh"
WORKDIR="$(mktemp -d)"
SHIM_BIN="${WORKDIR}/bin"
ARGV_LOG="${WORKDIR}/tofu-argv.log"
trap 'rm -rf "${WORKDIR}"' EXIT

PASS_A='rotation-argv-test-passphrase-a16'
PASS_B='rotation-argv-test-passphrase-b16'

if command -v tofu-1.13.1 >/dev/null 2>&1; then
  REAL_TOFU="$(command -v tofu-1.13.1)"
else
  REAL_TOFU="$(command -v tofu)"
fi

mkdir -p "${SHIM_BIN}"
cat > "${SHIM_BIN}/tofu" <<SHIM
#!/usr/bin/env bash
set -euo pipefail
log="\${TOFU_ARGV_LOG:?}"
{
  printf 'ARGV:%s\n' "\$*"
  tr '\\0' ' ' < /proc/self/cmdline
  printf '\n'
} >> "\${log}"
exec "${REAL_TOFU}" "\$@"
SHIM
chmod +x "${SHIM_BIN}/tofu"
ln -sf "${SHIM_BIN}/tofu" "${SHIM_BIN}/tofu-1.13.1"

mkdir -p "${WORKDIR}/stack"
cp "${FIXTURE}/variables.tf" "${FIXTURE}/main.tf" "${FIXTURE}/versions.tf" "${WORKDIR}/stack/"

(
  cd "${WORKDIR}/stack"
  export TF_VAR_passphrase="${PASS_A}"
  export PATH="${SHIM_BIN}:${PATH}"
  export TOFU_ARGV_LOG="${ARGV_LOG}"
  export TOFU=tofu-1.13.1
  "${TOFU}" init -input=false >/dev/null
  "${TOFU}" apply -input=false -auto-approve >/dev/null
)

: >"${ARGV_LOG}"
export PATH="${SHIM_BIN}:${PATH}"
export TOFU_ARGV_LOG="${ARGV_LOG}"
export TOFU=tofu-1.13.1
TOFU_ROTATE_OLD_PASSPHRASE="${PASS_A}" TOFU_ROTATE_NEW_PASSPHRASE="${PASS_B}" \
  "${ROTATE}" "${WORKDIR}/stack" bootstrap

if grep -F "${PASS_A}" "${ARGV_LOG}"; then
  echo "FAIL: old passphrase appeared in child argv log" >&2
  exit 1
fi
if grep -F "${PASS_B}" "${ARGV_LOG}"; then
  echo "FAIL: new passphrase appeared in child argv log" >&2
  exit 1
fi

echo "OK: rotation passphrases absent from child process argv"
