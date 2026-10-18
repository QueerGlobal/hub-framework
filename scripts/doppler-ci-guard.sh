#!/usr/bin/env bash
# Fail closed when CI DOPPLER_TOKEN can read bootstrap-admin (used at workflow start).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=scripts/lib/doppler-configs.sh
source "${ROOT}/scripts/lib/doppler-configs.sh"
# shellcheck source=scripts/lib/doppler-ci-token.sh
source "${ROOT}/scripts/lib/doppler-ci-token.sh"

doppler_assert_ci_token_cannot_read_bootstrap_admin
