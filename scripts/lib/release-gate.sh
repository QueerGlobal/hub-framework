# shellcheck shell=bash
# Read-only checks that a release may be tagged: the commit passed CI and the
# release environment requires a reviewer. Uses gh with GH_TOKEN from the environment.

release_gate_environment_has_reviewers() {
  local repo=$1 environment=$2 count
  count="$(gh api "repos/${repo}/environments/${environment}" \
    --jq '[.protection_rules[]? | select(.type == "required_reviewers")] | length' 2>/dev/null)" || {
    echo "FAIL: could not read the ${environment} environment for ${repo}" >&2
    return 1
  }
  if [[ "${count}" != [1-9]* ]]; then
    echo "FAIL: the ${environment} environment has no required reviewers, so this run was not approved" >&2
    return 1
  fi
}

release_gate_ci_green() {
  local repo=$1 sha=$2 self_job=$3 runs
  runs="$(gh api "repos/${repo}/commits/${sha}/check-runs?per_page=100" \
    --jq "[.check_runs[] | select(.name != \"${self_job}\") | {name, status, conclusion}]" 2>/dev/null)" || {
    echo "FAIL: could not read check runs for ${sha}" >&2
    return 1
  }
  if ! jq -e 'any(.[]; .name == "test" and .conclusion == "success")' >/dev/null <<<"${runs}"; then
    echo "FAIL: the Go test check has not passed on ${sha}" >&2
    return 1
  fi
  if jq -e 'any(.[]; .status != "completed")' >/dev/null <<<"${runs}"; then
    echo "FAIL: a check on ${sha} is still running" >&2
    return 1
  fi
  if jq -e 'any(.[]; (.conclusion | IN("success", "skipped", "neutral")) | not)' >/dev/null <<<"${runs}"; then
    echo "FAIL: a check on ${sha} did not pass: $(jq -r '[.[] | select((.conclusion | IN("success", "skipped", "neutral")) | not) | .name] | join(", ")' <<<"${runs}")" >&2
    return 1
  fi
}

release_gate() {
  local repo=$1 sha=$2 environment=$3 self_job=$4
  release_gate_environment_has_reviewers "${repo}" "${environment}" || return 1
  release_gate_ci_green "${repo}" "${sha}" "${self_job}" || return 1
  echo "OK: ${sha} passed CI and the ${environment} environment requires a reviewer"
}
