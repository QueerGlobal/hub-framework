#!/usr/bin/env bash
# Fault injection against the real rotation script, real OpenTofu and the Doppler mock.
# Each fault must stop the rotation before promotion, or resume it, with its own reason.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
FIXTURE="${ROOT}/infra/opentofu/encryption-rotation-fixture"
ROTATE="${ROOT}/scripts/tofu-rotate-encryption-passphrase.sh"
WORKDIR="$(mktemp -d)"
trap 'rm -rf "${WORKDIR}"' EXIT

# shellcheck source=scripts/test/lib/rotation-doppler-mock.sh
source "${ROOT}/scripts/test/lib/rotation-doppler-mock.sh"
REAL_TOFU="$(command -v tofu-1.13.1 || command -v tofu)"
STACK="${WORKDIR}/stack"
DB="${WORKDIR}/doppler.tsv"
CUR=TOFU_BOOTSTRAP_STATE_ENCRYPTION_PASSPHRASE
NEXT=TOFU_BOOTSTRAP_STATE_ENCRYPTION_PASSPHRASE_NEXT
PASS_A='rotation-fault-passphrase-a16'
STAGED='rotation-fault-staged-next-passphrase-b16'
mkdir -p "${WORKDIR}/bin"
rotation_doppler_mock_install "${WORKDIR}/bin" "${DB}"

# OpenTofu wrapper that injects one fault, selected by TOFU_FAULT, then runs the real binary.
cat >"${WORKDIR}/bin/tofu-fault" <<FAULT
#!/usr/bin/bash
set -euo pipefail
count_file="${WORKDIR}/tofu-calls"
n=1
if [[ -f "\${count_file}" ]]; then n=\$(( \$(cat "\${count_file}") + 1 )); fi
echo "\${n}" >"\${count_file}"
case "\${TOFU_FAULT:-}:\${1:-}" in
  skip-second-apply:apply)
    if [[ -f "${WORKDIR}/applies" ]]; then exit 0; fi
    echo apply >>"${WORKDIR}/applies"
    ;;
  crash-second-apply:apply)
    if [[ -f "${WORKDIR}/applies" ]]; then exit 1; fi
    echo apply >>"${WORKDIR}/applies"
    ;;
  old-decrypts:plan)
    if [[ "\${TF_VAR_passphrase:-}" == "${PASS_A}" ]]; then exit 0; fi
    ;;
  old-other-error:plan)
    if [[ "\${TF_VAR_passphrase:-}" == "${PASS_A}" ]]; then echo "Error: network unreachable" >&2; exit 1; fi
    ;;
  crash-verify:plan)
    if [[ \${n} -eq 4 ]]; then exit 1; fi
    ;;
esac
exec "${REAL_TOFU}" "\$@"
FAULT
chmod +x "${WORKDIR}/bin/tofu-fault"

export PATH="${WORKDIR}/bin:${PATH}" ROTATION_DOPPLER_DB="${DB}" DOPPLER_PROJECT=hub-framework DOPPLER_CONFIG=bootstrap-admin

prepare() {
  rm -rf "${STACK}" "${WORKDIR}/tofu-calls" "${WORKDIR}/applies"
  mkdir -p "${STACK}"
  : >"${DB}"
  : >"${DB}.writes"
  cp "${FIXTURE}/variables.tf" "${FIXTURE}/main.tf" "${FIXTURE}/versions.tf" "${STACK}/"
  rotation_doppler_seed "${DB}" "${CUR}" hub-framework bootstrap-admin "${PASS_A}"
  (
    cd "${STACK}"
    TF_VAR_passphrase="${PASS_A}" "${REAL_TOFU}" init -input=false >/dev/null
    TF_VAR_passphrase="${PASS_A}" "${REAL_TOFU}" apply -input=false -auto-approve >/dev/null
  )
}

# run_rotate <out_file> [env assignments...]: runs the rotation, returns its exit code.
run_rotate() {
  local out=$1
  shift
  local rc=0
  env TOFU="${WORKDIR}/bin/tofu-fault" "$@" bash "${ROTATE}" "${STACK}" bootstrap >"${out}" 2>&1 || rc=$?
  return "${rc}"
}

expect_stopped() {
  local label=$1 reason=$2 out=$3 rc=$4
  if [[ ${rc} -eq 0 ]]; then
    echo "FAIL: ${label}: rotation should have stopped" >&2
    exit 1
  fi
  if ! grep -Fq "${reason}" "${out}"; then
    echo "FAIL: ${label}: missing reason: ${reason}" >&2
    cat "${out}" >&2
    exit 1
  fi
  if [[ "$(rotation_doppler_value "${DB}" "${CUR}")" != "${PASS_A}" ]]; then
    echo "FAIL: ${label}: the current passphrase was changed" >&2
    exit 1
  fi
  if grep -q "^set ${CUR} \|^set TOFU_BOOTSTRAP_STATE_LOCAL_BACKUP " "${DB}.writes"; then
    echo "FAIL: ${label}: promoted or backed up after the fault" >&2
    exit 1
  fi
  echo "OK: ${label} stopped before promotion with its reason"
}

expect_finished() {
  local label=$1 out=$2 rc=$3 want=$4
  if [[ ${rc} -ne 0 ]]; then
    echo "FAIL: ${label}: rotation should have finished" >&2
    cat "${out}" >&2
    exit 1
  fi
  if [[ "$(rotation_doppler_value "${DB}" "${CUR}")" != "${want}" ]]; then
    echo "FAIL: ${label}: the promoted passphrase is not the staged one" >&2
    exit 1
  fi
  if [[ -n "$(rotation_doppler_value "${DB}" "${NEXT}")" ]]; then
    echo "FAIL: ${label}: ${NEXT} was left behind" >&2
    exit 1
  fi
  if ! (cd "${STACK}" && TF_VAR_passphrase="${want}" "${REAL_TOFU}" plan -input=false -lock=false >/dev/null 2>&1); then
    echo "FAIL: ${label}: the committed config cannot decrypt state with the promoted passphrase" >&2
    exit 1
  fi
  echo "OK: ${label} finished with the staged passphrase"
}

OUT="${WORKDIR}/out"
rc=0

prepare
run_rotate "${OUT}" ROTATION_DOPPLER_FAIL_GET="${CUR}" || rc=$?
expect_stopped "auth error reading the current passphrase" "Doppler error while reading ${CUR} from" "${OUT}" "${rc}"
if [[ -s "${DB}.writes" ]]; then
  echo "FAIL: auth error reading the current passphrase: Doppler was written" >&2
  exit 1
fi

prepare
rotation_doppler_seed "${DB}" "${NEXT}" hub-framework bootstrap-admin "${STAGED}"
rc=0; run_rotate "${OUT}" ROTATION_DOPPLER_FAIL_GET="${NEXT}" || rc=$?
expect_stopped "auth error reading the staged _NEXT" "Doppler error while reading ${NEXT} from" "${OUT}" "${rc}"
if [[ -s "${DB}.writes" || "$(rotation_doppler_value "${DB}" "${NEXT}")" != "${STAGED}" ]]; then
  echo "FAIL: auth error reading the staged _NEXT: the staged passphrase was overwritten" >&2
  exit 1
fi

prepare
rc=0
env TOFU="${WORKDIR}/bin/tofu-fault" bash -x "${ROTATE}" "${STACK}" bootstrap >"${OUT}" 2>&1 || rc=$?
expect_stopped "xtrace on" "Refusing to run with xtrace on" "${OUT}" "${rc}"
if grep -Fq "${PASS_A}" "${OUT}" || [[ -s "${DB}.writes" ]]; then
  echo "FAIL: xtrace on: a passphrase was printed or Doppler was written" >&2
  exit 1
fi

prepare
rc=0; run_rotate "${OUT}" TOFU_FAULT=skip-second-apply || rc=$?
expect_stopped "second hop skipped" "The committed config cannot decrypt state with the new passphrase; not promoting." "${OUT}" "${rc}"

prepare
rc=0; run_rotate "${OUT}" TOFU_FAULT=old-decrypts || rc=$?
expect_stopped "old passphrase still decrypts" "Old passphrase must not decrypt state after rotation; not promoting." "${OUT}" "${rc}"

prepare
rc=0; run_rotate "${OUT}" TOFU_FAULT=old-other-error || rc=$?
expect_stopped "old passphrase fails for another reason" "The old passphrase failed for a reason other than decryption; not promoting." "${OUT}" "${rc}"

prepare
rc=0; run_rotate "${OUT}" TOFU_FAULT=crash-second-apply || rc=$?
expect_stopped "crash after hop 1" "" "${OUT}" "${rc}"
STAGED_RUN="$(rotation_doppler_value "${DB}" "${NEXT}")"
if [[ ${#STAGED_RUN} -lt 32 ]]; then
  echo "FAIL: crash after hop 1: no generated _NEXT was staged" >&2
  exit 1
fi
rc=0; run_rotate "${OUT}" || rc=$?
if ! grep -Fq "Resuming the rotation staged in ${NEXT}." "${OUT}"; then
  echo "FAIL: crash after hop 1: the rerun did not resume from _NEXT" >&2
  exit 1
fi
expect_finished "resume after a crash after hop 1" "${OUT}" "${rc}" "${STAGED_RUN}"

prepare
rc=0; run_rotate "${OUT}" TOFU_FAULT=crash-verify || rc=$?
expect_stopped "crash after hop 2" "not promoting" "${OUT}" "${rc}"
STAGED_RUN="$(rotation_doppler_value "${DB}" "${NEXT}")"
rm -f "${WORKDIR}/tofu-calls"
rc=0; run_rotate "${OUT}" || rc=$?
if ! grep -Fq "State already decrypts with ${NEXT}; skipping the two hops." "${OUT}"; then
  echo "FAIL: crash after hop 2: the rerun did not skip the finished hops" >&2
  exit 1
fi
expect_finished "resume after a crash after hop 2" "${OUT}" "${rc}" "${STAGED_RUN}"

prepare
rotation_doppler_seed "${DB}" "${NEXT}" hub-framework bootstrap-admin "${PASS_A}"
rc=0; run_rotate "${OUT}" || rc=$?
if [[ ${rc} -ne 0 ]] || ! grep -Fq "${CUR} was already promoted; removed the leftover ${NEXT}." "${OUT}" \
  || [[ -n "$(rotation_doppler_value "${DB}" "${NEXT}")" ]] || grep -q '^set ' "${DB}.writes"; then
  echo "FAIL: leftover _NEXT after promotion was not cleaned up alone" >&2
  exit 1
fi
echo "OK: leftover _NEXT after promotion cleaned up without a rotation"

prepare
: >"${DB}"
rotation_doppler_seed "${DB}" "${CUR}" hub-framework bootstrap-admin ""
rc=0; run_rotate "${OUT}" || rc=$?
if [[ ${rc} -eq 0 ]] || ! grep -Fq "Missing current passphrase ${CUR}" "${OUT}"; then
  echo "FAIL: empty current passphrase was not refused" >&2
  exit 1
fi
echo "OK: empty current passphrase refused"

echo "OK: encryption rotation fault checks passed"
