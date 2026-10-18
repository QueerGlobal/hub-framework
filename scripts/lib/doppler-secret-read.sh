# shellcheck shell=bash
# Reads one Doppler secret so that a Doppler error is never mistaken for a missing secret.
# With --no-exit-on-missing-secret the CLI exits 0 and prints nothing for an absent secret,
# so any non-zero exit is a real error (network, auth, or a missing project or config).

# Prints the value, or nothing when the secret is absent or empty. Fails on any Doppler error.
doppler_secret_read() {
  local name=$1
  local project=$2
  local config=$3
  doppler secrets get "${name}" --plain --no-exit-on-missing-secret -p "${project}" -c "${config}"
}

# Prints "missing" (absent or empty) or "present". Fails with a reason on any Doppler error.
doppler_secret_state() {
  local name=$1
  local project=$2
  local config=$3
  local value
  if ! value="$(doppler_secret_read "${name}" "${project}" "${config}")"; then
    echo "Doppler error while reading ${name} from ${project}/${config}; refusing to treat it as missing." >&2
    return 1
  fi
  if [[ -z "${value}" ]]; then
    echo missing
  else
    echo present
  fi
}
