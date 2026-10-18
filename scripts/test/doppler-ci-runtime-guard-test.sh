#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=scripts/lib/doppler-configs.sh
source "${ROOT}/scripts/lib/doppler-configs.sh"
# shellcheck source=scripts/lib/doppler-ci-token.sh
source "${ROOT}/scripts/lib/doppler-ci-token.sh"

if env -u DOPPLER_TOKEN DOPPLER_CI_ALLOW_UNSET_TOKEN= bash -c \
  'source "'"${ROOT}"'/scripts/lib/doppler-configs.sh"; source "'"${ROOT}"'/scripts/lib/doppler-ci-token.sh"; doppler_assert_ci_token_cannot_read_bootstrap_admin'; then
  echo "FAIL: guard should fail closed when DOPPLER_TOKEN is unset" >&2
  exit 1
fi
echo "OK: guard fails closed without DOPPLER_TOKEN"

if ! env -u DOPPLER_TOKEN DOPPLER_CI_ALLOW_UNSET_TOKEN=1 bash -c \
  'source "'"${ROOT}"'/scripts/lib/doppler-configs.sh"; source "'"${ROOT}"'/scripts/lib/doppler-ci-token.sh"; doppler_assert_ci_token_cannot_read_bootstrap_admin'; then
  echo "FAIL: guard should allow unset token when DOPPLER_CI_ALLOW_UNSET_TOKEN=1" >&2
  exit 1
fi
echo "OK: guard allows unset token only with DOPPLER_CI_ALLOW_UNSET_TOKEN=1"
