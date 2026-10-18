#!/usr/bin/env bash
# The release gate must refuse to tag unless CI passed on the commit and the release environment requires a reviewer.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
WORKDIR="$(mktemp -d)"
trap 'rm -rf "${WORKDIR}"' EXIT
mkdir -p "${WORKDIR}/bin"
cat >"${WORKDIR}/bin/gh" <<'MOCK'
#!/usr/bin/bash
set -euo pipefail
[[ "${1:-}" == api ]] || { echo "unexpected gh: $*" >&2; exit 1; }
path=$2 filter=""
[[ "${3:-}" == --jq ]] && filter=$4
case "${path}" in
  repos/QueerGlobal/hub-framework/environments/release) body=${MOCK_GH_ENV:?} ;;
  repos/QueerGlobal/hub-framework/commits/*/check-runs*) body=${MOCK_GH_CHECKS:?} ;;
  *) echo "unexpected gh api path: ${path}" >&2; exit 1 ;;
esac
[[ "${body}" == HTTP404 ]] && { echo "gh: Not Found (HTTP 404)" >&2; exit 1; }
jq -r "${filter:-.}" <<<"${body}"
MOCK
chmod +x "${WORKDIR}/bin/gh"

REVIEWERS='{"protection_rules":[{"type":"branch_policy"},{"type":"required_reviewers"}]}'
NO_REVIEWERS='{"protection_rules":[{"type":"branch_policy"}]}'
checks() { jq -cn --args '{check_runs: [$ARGS.positional[] | split(":") | {name: .[0], status: .[1], conclusion: (.[2] // null)}]}' "$@"; }
GREEN="$(checks test:completed:success opentofu-and-shell:completed:success release:in_progress)"

# Values are exported in a subshell, so the fake token never lands on any argv.
run_gate() {
  (
    export PATH="${WORKDIR}/bin:${PATH}" GH_TOKEN=fake-gh-token-for-gate-test
    export GITHUB_REPOSITORY=QueerGlobal/hub-framework GITHUB_SHA=0123456789abcdef0123456789abcdef01234567
    local kv
    for kv in "$@"; do
      export "${kv?}"
    done
    bash "${ROOT}/scripts/release-gate.sh" 2>&1
  )
}

expect_fail() {
  local label=$1 needle=$2 out rc=0
  shift 2
  out="$(run_gate "$@")" || rc=$?
  [[ ${rc} -ne 0 ]] || { echo "FAIL: release gate passed with ${label}" >&2; exit 1; }
  grep -Fq "${needle}" <<<"${out}" || { echo "FAIL: release gate failed with ${label} without reason: ${needle}" >&2; echo "${out}" >&2; exit 1; }
  echo "OK: release gate refuses ${label}"
}

out="$(run_gate MOCK_GH_ENV="${REVIEWERS}" MOCK_GH_CHECKS="${GREEN}")" || { echo "FAIL: release gate refused a green, approved commit" >&2; echo "${out}" >&2; exit 1; }
echo "OK: release gate passes a green commit with required reviewers"

expect_fail "an environment without reviewers" "has no required reviewers" MOCK_GH_ENV="${NO_REVIEWERS}" MOCK_GH_CHECKS="${GREEN}"
expect_fail "a missing environment" "could not read the release environment" MOCK_GH_ENV=HTTP404 MOCK_GH_CHECKS="${GREEN}"
expect_fail "no Go test check" "the Go test check has not passed" MOCK_GH_ENV="${REVIEWERS}" \
  MOCK_GH_CHECKS="$(checks opentofu-and-shell:completed:success)"
expect_fail "a failed Go test check" "the Go test check has not passed" MOCK_GH_ENV="${REVIEWERS}" \
  MOCK_GH_CHECKS="$(checks test:completed:failure)"
expect_fail "a check still running" "is still running" MOCK_GH_ENV="${REVIEWERS}" \
  MOCK_GH_CHECKS="$(checks test:completed:success opentofu-and-shell:in_progress)"
expect_fail "a failed infra check" "did not pass: opentofu-and-shell" MOCK_GH_ENV="${REVIEWERS}" \
  MOCK_GH_CHECKS="$(checks test:completed:success opentofu-and-shell:completed:failure)"
expect_fail "unreadable check runs" "could not read check runs" MOCK_GH_ENV="${REVIEWERS}" MOCK_GH_CHECKS=HTTP404
expect_fail "no GH_TOKEN" "GH_TOKEN must be set" GH_TOKEN= MOCK_GH_ENV="${REVIEWERS}" MOCK_GH_CHECKS="${GREEN}"
# The Release workflow must run the gate before it creates the tag.
gate_line="$(grep -n 'run: scripts/release-gate.sh' "${ROOT}/.github/workflows/release.yml" | cut -d: -f1 | head -1 || true)"
tag_line="$(grep -n 'git tag -a' "${ROOT}/.github/workflows/release.yml" | cut -d: -f1 | head -1 || true)"
if [[ -z "${gate_line}" || -z "${tag_line}" || "${gate_line}" -ge "${tag_line}" ]]; then
  echo "FAIL: the Release workflow does not run the release gate before tagging" >&2
  exit 1
fi
echo "OK: the Release workflow runs the gate before tagging"
echo "OK: release gate checks passed"
