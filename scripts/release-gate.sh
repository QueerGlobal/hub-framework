#!/usr/bin/env bash
# Release workflow gate. Fails closed unless the commit passed CI and the release environment requires approval.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=scripts/lib/release-gate.sh
source "${ROOT}/scripts/lib/release-gate.sh"

: "${GH_TOKEN:?GH_TOKEN must be set}"
release_gate "${GITHUB_REPOSITORY:?}" "${GITHUB_SHA:?}" "${RELEASE_ENVIRONMENT:-release}" "${RELEASE_JOB_NAME:-release}"
