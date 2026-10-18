# shellcheck shell=bash

# Assert encrypted state metadata is keyed under the original key_provider name (not rotation / _primary).
tofu_assert_state_method_metadata() {
  local state_file=$1
  local provider_name=$2
  local expected_key_provider="key_provider.pbkdf2.${provider_name}"
  local wrong_primary="key_provider.pbkdf2.${provider_name}_primary"
  local rotation_key_provider="key_provider.pbkdf2.rotation"

  if [[ ! -f "${state_file}" ]]; then
    echo "FAIL: missing state file ${state_file}" >&2
    return 1
  fi

  if strings "${state_file}" | grep -Fq "${wrong_primary}"; then
    echo "FAIL: state metadata still references ${wrong_primary}" >&2
    return 1
  fi

  if strings "${state_file}" | grep -Fq "${rotation_key_provider}"; then
    echo "FAIL: state metadata still on ${rotation_key_provider}, expected ${expected_key_provider}" >&2
    return 1
  fi

  if strings "${state_file}" | grep -Fq "${expected_key_provider}"; then
    return 0
  fi

  echo "FAIL: could not find ${expected_key_provider} in state metadata" >&2
  return 1
}
