# shellcheck shell=bash
# Guards for scripts that hold secrets in shell variables.

# Exits when xtrace is on (bash -x, set -x or SHELLOPTS=xtrace), which would print every secret.
secret_shell_refuse_xtrace() {
  if [[ $- == *x* ]]; then
    set +x
    echo "Refusing to run with xtrace on (bash -x, set -x or SHELLOPTS): it would print secrets." >&2
    exit 1
  fi
}

# Prints a new random passphrase for OpenTofu state encryption.
secret_shell_generate_passphrase() {
  local pass
  pass="$(openssl rand -base64 32)" || return 1
  if [[ ${#pass} -lt 32 ]]; then
    echo "Generated passphrase was too short." >&2
    return 1
  fi
  printf '%s' "${pass}"
}
