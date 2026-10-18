#!/usr/bin/env bash
# Ensures OpenTofu plan files for real stacks do not contain encryption passphrases in plain text.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
TRAP_PLAN_FILES=()

cleanup() {
  local f
  for f in "${TRAP_PLAN_FILES[@]:-}"; do
    rm -f "${f}"
  done
}
trap cleanup EXIT

umask 077

run_mocked_plan_test() {
  local stack_name=$1
  local stack_dir=$2
  shift 2
  local -a exports=("$@")

  (
    cd "${stack_dir}"
    rm -rf .terraform terraform.tfstate terraform.tfstate.backup
    local init_args=(-input=false)
    if [[ "${stack_name}" == "release-environment" ]]; then
      init_args+=(-backend=false)
    fi
    tofu init "${init_args[@]}" >/dev/null
    for kv in "${exports[@]}"; do
      export "${kv?}"
    done
    tofu test -filter=plan-leak >/dev/null
  )
  echo "OK: ${stack_name} mocked tofu test plan (plan-leak.tftest.hcl)"
}

assert_plan_out_no_passphrase() {
  local stack_name=$1
  local stack_dir=$2
  local pass=$3
  local plan_file=$4
  shift 4
  local -a exports=("$@")

  local workdir
  workdir="$(mktemp -d)"
  cp "${stack_dir}/versions.tf" "${stack_dir}/variables.tf" "${workdir}/"
  if [[ "${stack_name}" == "release-environment" ]]; then
    sed -i 's/backend "s3" {}/backend "local" {\n    path = "terraform.tfstate"\n  }/' "${workdir}/versions.tf"
  fi
  cat > "${workdir}/main.tf" <<'EOF'
resource "terraform_data" "plan_leak_probe" {
  input = "plan-leak"
}
EOF

  TRAP_PLAN_FILES+=("${plan_file}")

  (
    cd "${workdir}"
    tofu init -input=false >/dev/null
    for kv in "${exports[@]}"; do
      export "${kv?}"
    done
    tofu plan -input=false -out="${plan_file}" >/dev/null
  )

  rm -rf "${workdir}"

  if grep -F "${pass}" "${plan_file}" >/dev/null 2>&1; then
    echo "FAIL: ${stack_name} plan file contains plaintext encryption passphrase" >&2
    return 1
  fi

  echo "OK: ${stack_name} plan file has no plaintext passphrase (${plan_file})"
}

BOOTSTRAP_PASS='plan-leak-bootstrap-passphrase-xyzzy'
RELEASE_PASS='plan-leak-release-passphrase-xyzzy'
BOOTSTRAP_DIR="${ROOT}/infra/opentofu/state-bootstrap"
RELEASE_DIR="${ROOT}/infra/opentofu/release-environment"

run_mocked_plan_test "state-bootstrap" "${BOOTSTRAP_DIR}" \
  "TF_VAR_bootstrap_state_encryption_passphrase=${BOOTSTRAP_PASS}" \
  "TF_VAR_cloudflare_account_id=00000000000000000000000000000001" \
  "TF_VAR_state_bucket_name=queerglobal-hub-framework-tofu-state" \
  "CLOUDFLARE_API_TOKEN=plan-leak-mock-token"

assert_plan_out_no_passphrase "state-bootstrap" "${BOOTSTRAP_DIR}" "${BOOTSTRAP_PASS}" \
  "${BOOTSTRAP_DIR}/ci-plan-leak.tfplan" \
  "TF_VAR_bootstrap_state_encryption_passphrase=${BOOTSTRAP_PASS}"

run_mocked_plan_test "release-environment" "${RELEASE_DIR}" \
  "TF_VAR_state_encryption_passphrase=${RELEASE_PASS}"

assert_plan_out_no_passphrase "release-environment" "${RELEASE_DIR}" "${RELEASE_PASS}" \
  "${RELEASE_DIR}/ci-plan-leak.tfplan" \
  "TF_VAR_state_encryption_passphrase=${RELEASE_PASS}"

echo "OK: plan encryption checks passed for state-bootstrap and release-environment"
