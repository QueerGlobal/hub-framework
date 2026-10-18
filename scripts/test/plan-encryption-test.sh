#!/usr/bin/env bash
# Ensures OpenTofu plan files do not contain encryption passphrases in plain text.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
STACK="${ROOT}/infra/opentofu/plan-encryption-fixture"
PLAN_FILE="${STACK}/ci-plan-encryption-test.tfplan"
PASS='plan-encryption-test-passphrase-xyzzy'

cleanup() {
  rm -f "${PLAN_FILE}"
}
trap cleanup EXIT

umask 077

cd "${STACK}"
rm -rf .terraform
tofu init -input=false >/dev/null

export TF_VAR_bootstrap_state_encryption_passphrase="${PASS}"

tofu plan -input=false -out="${PLAN_FILE}" >/dev/null

if grep -F "${PASS}" "${PLAN_FILE}" >/dev/null 2>&1; then
  echo "FAIL: plan file contains plaintext encryption passphrase" >&2
  exit 1
fi

echo "OK: plan file has no plaintext passphrase (${PLAN_FILE})"
