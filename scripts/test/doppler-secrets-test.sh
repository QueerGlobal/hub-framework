#!/usr/bin/env bash
# Verifies doppler_secret_set_stdin never puts secret values on the doppler argv.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
TESTDIR="$(mktemp -d)"
trap 'rm -rf "${TESTDIR}"' EXIT

FAKE_BIN="${TESTDIR}/bin"
mkdir -p "${FAKE_BIN}"
LOG="${TESTDIR}/doppler.log"
STDIN_CAPTURE="${TESTDIR}/stdin.txt"

cat > "${FAKE_BIN}/doppler" <<'EOF'
#!/usr/bin/env bash
echo "argv=$*" >> "$DOPPLER_TEST_LOG"
if [[ "$1" == "secrets" && "$2" == "set" ]]; then
  cat > "$DOPPLER_TEST_STDIN"
fi
exit 0
EOF
chmod +x "${FAKE_BIN}/doppler"

export DOPPLER_TEST_LOG="${LOG}"
export DOPPLER_TEST_STDIN="${STDIN_CAPTURE}"
export PATH="${FAKE_BIN}:${PATH}"
export DOPPLER_PROJECT=hub-framework
export DOPPLER_CONFIG=infra

# shellcheck source=scripts/lib/doppler-secrets.sh
source "${ROOT}/scripts/lib/doppler-secrets.sh"

SECRET='super-secret-not-on-argv-xyzzy'
doppler_secret_set_stdin TEST_SECRET "${SECRET}"

if grep -F "${SECRET}" "${LOG}"; then
  echo "FAIL: secret value appeared in doppler argv log" >&2
  exit 1
fi

if ! grep -Fx "${SECRET}" "${STDIN_CAPTURE}"; then
  echo "FAIL: secret value not passed via stdin" >&2
  exit 1
fi

export BOOTSTRAP_DOPPLER_SECRET_DRY_RUN=1
if ! doppler_secret_set_stdin DRY_RUN_KEY "${SECRET}" | grep -q 'dry-run: doppler secrets set DRY_RUN_KEY'; then
  echo "FAIL: dry-run mode did not emit expected message" >&2
  exit 1
fi

echo "OK: doppler_secret_set_stdin tests passed"
