#!/usr/bin/env bash
# Proves plan encryption hides a sensitive marker in real stack plan files.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
TRAP_PLAN_FILES=()
TRAP_WORKDIRS=()

cleanup() {
  local f d
  for f in "${TRAP_PLAN_FILES[@]:-}"; do
    rm -f "${f}"
  done
  for d in "${TRAP_WORKDIRS[@]:-}"; do
    rm -rf "${d}"
  done
}
trap cleanup EXIT

umask 077

plan_file_contains_marker() {
  local plan_file=$1
  local marker=$2
  if unzip -t "${plan_file}" >/dev/null 2>&1; then
    unzip -p "${plan_file}" 2>/dev/null | grep -aF "${marker}" >/dev/null 2>&1
    return $?
  fi
  grep -aF "${marker}" "${plan_file}" >/dev/null 2>&1
}

assert_versions_plan_block() {
  local stack_name=$1
  local versions_tf=$2
  if ! grep -qE '[[:space:]]plan[[:space:]]*\{' "${versions_tf}"; then
    echo "FAIL: ${stack_name} versions.tf is missing a plan { encryption } block" >&2
    return 1
  fi
  echo "OK: ${stack_name} versions.tf defines plan encryption"
}

copy_stack_to_workdir() {
  local stack_dir=$1
  local workdir=$2
  local fixture_dir=$3
  shopt -s nullglob
  local f
  for f in "${stack_dir}"/*.tf "${stack_dir}"/.terraform.lock.hcl; do
    [[ -e "${f}" ]] || continue
    cp "${f}" "${workdir}/"
  done
  if [[ -n "${fixture_dir}" ]]; then
    cp "${fixture_dir}/plan_leak_probe.tf" "${fixture_dir}/plan-leak.ci.tfvars" "${workdir}/"
  fi
  shopt -u nullglob
  rm -rf "${workdir}/.terraform" "${workdir}/terraform.tfstate" "${workdir}/terraform.tfstate.backup"
}

prepare_release_workdir_backend() {
  local workdir=$1
  rm -f "${workdir}/backend_s3.tf"
  cat > "${workdir}/backend_local.tf" <<'EOF'
terraform {
  backend "local" {
    path = "terraform.tfstate"
  }
}
EOF
}

plan_leak_stack() {
  local stack_name=$1
  local stack_dir=$2
  local marker=$3
  local plan_file=$4
  local tfvars_file=$5
  shift 5
  local -a extra_env=("$@")

  assert_versions_plan_block "${stack_name}" "${stack_dir}/versions.tf"

  local workdir
  workdir="$(mktemp -d)"
  TRAP_WORKDIRS+=("${workdir}")

  local fixture_dir="${ROOT}/infra/opentofu/fixtures/plan-leak/${stack_name}"
  copy_stack_to_workdir "${stack_dir}" "${workdir}" "${fixture_dir}"
  if [[ "${stack_name}" == "release-environment" ]]; then
    prepare_release_workdir_backend "${workdir}"
  fi

  TRAP_PLAN_FILES+=("${plan_file}")

  (
    cd "${workdir}"
    for kv in "${extra_env[@]}"; do
      export "${kv?}"
    done
    tofu init -input=false >/dev/null
    tofu plan -input=false \
      -var-file="${tfvars_file}" \
      -target='terraform_data.plan_leak_probe[0]' \
      -out="${plan_file}" >/dev/null
  )

  if plan_file_contains_marker "${plan_file}" "${marker}"; then
    echo "FAIL: ${stack_name} encrypted plan file contains sensitive marker (plan encryption ineffective)" >&2
    return 1
  fi

  echo "OK: ${stack_name} encrypted plan hides sensitive marker (${plan_file})"
}

# Sanity check: without plan {}, the same marker must appear in the plan file bytes.
assert_unencrypted_plan_exposes_marker() {
  local stack_name=$1
  local stack_dir=$2
  local marker=$3
  local tfvars_file=$4
  shift 4
  local -a extra_env=("$@")

  local workdir plan_file
  workdir="$(mktemp -d)"
  TRAP_WORKDIRS+=("${workdir}")
  plan_file="${workdir}/unencrypted.tfplan"

  local fixture_dir="${ROOT}/infra/opentofu/fixtures/plan-leak/${stack_name}"
  copy_stack_to_workdir "${stack_dir}" "${workdir}" "${fixture_dir}"
  if [[ "${stack_name}" == "release-environment" ]]; then
    prepare_release_workdir_backend "${workdir}"
  fi

  python3 - "${workdir}/versions.tf" <<'PY'
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
text = path.read_text()
needle = "plan {"
start = text.find(needle)
if start < 0:
    sys.exit("plan block not found")
depth = 0
idx = start
while idx < len(text):
    ch = text[idx]
    if ch == "{":
        depth += 1
    elif ch == "}":
        depth -= 1
        if depth == 0:
            text = text[:start] + text[idx + 1 :]
            break
    idx += 1
else:
    sys.exit("unbalanced plan block")
path.write_text(text)
PY

  (
    cd "${workdir}"
    for kv in "${extra_env[@]}"; do
      export "${kv?}"
    done
    tofu init -input=false >/dev/null
    tofu plan -input=false \
      -var-file="${tfvars_file}" \
      -target='terraform_data.plan_leak_probe[0]' \
      -out="${plan_file}" >/dev/null
  )

  if ! plan_file_contains_marker "${plan_file}" "${marker}"; then
    echo "FAIL: ${stack_name} plan without plan {} block did not expose marker (encryption test is not meaningful)" >&2
    return 1
  fi

  echo "OK: ${stack_name} unencrypted plan exposes marker (control case)"
}

BOOTSTRAP_DIR="${ROOT}/infra/opentofu/state-bootstrap"
RELEASE_DIR="${ROOT}/infra/opentofu/release-environment"
BOOTSTRAP_MARKER='PLAN-LEAK-MARKER-STATE-BOOTSTRAP-xyzzy-0123456789'
RELEASE_MARKER='PLAN-LEAK-MARKER-RELEASE-ENV-xyzzy-0123456789'
BOOTSTRAP_TFVARS="${ROOT}/infra/opentofu/fixtures/plan-leak/state-bootstrap/plan-leak.ci.tfvars"
RELEASE_TFVARS="${ROOT}/infra/opentofu/fixtures/plan-leak/release-environment/plan-leak.ci.tfvars"

BOOTSTRAP_ENV=(
  "TF_VAR_bootstrap_state_encryption_passphrase=plan-leak-bootstrap-passphrase-16chars"
  "TF_VAR_cloudflare_account_id=00000000000000000000000000000001"
  "TF_VAR_state_bucket_name=queerglobal-hub-framework-tofu-state"
  "CLOUDFLARE_API_TOKEN=plan-leak-mock-token"
)
RELEASE_ENV=(
  "TF_VAR_state_encryption_passphrase=plan-leak-release-passphrase-16chars"
  "GITHUB_TOKEN=plan-leak-mock-token"
)

plan_leak_stack "state-bootstrap" "${BOOTSTRAP_DIR}" "${BOOTSTRAP_MARKER}" \
  "${BOOTSTRAP_DIR}/ci-plan-leak.tfplan" "${BOOTSTRAP_TFVARS}" "${BOOTSTRAP_ENV[@]}"

plan_leak_stack "release-environment" "${RELEASE_DIR}" "${RELEASE_MARKER}" \
  "${RELEASE_DIR}/ci-plan-leak.tfplan" "${RELEASE_TFVARS}" "${RELEASE_ENV[@]}"

assert_unencrypted_plan_exposes_marker "state-bootstrap" "${BOOTSTRAP_DIR}" "${BOOTSTRAP_MARKER}" \
  "${BOOTSTRAP_TFVARS}" "${BOOTSTRAP_ENV[@]}"

assert_unencrypted_plan_exposes_marker "release-environment" "${RELEASE_DIR}" "${RELEASE_MARKER}" \
  "${RELEASE_TFVARS}" "${RELEASE_ENV[@]}"

echo "OK: plan encryption checks passed for state-bootstrap and release-environment"
