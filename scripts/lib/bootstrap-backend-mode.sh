# shellcheck shell=bash
# Bootstrap state stays on the local backend; this backs the encrypted state file up to Doppler.

bootstrap_backup_local_state_to_doppler() {
  local state_file=$1
  local project=$2
  local config=$3
  if [[ ! -f "${state_file}" ]]; then
    echo "No local bootstrap state at ${state_file} after apply; refusing to continue without a backup." >&2
    return 1
  fi
  doppler secrets set TOFU_BOOTSTRAP_STATE_LOCAL_BACKUP \
    --project "${project}" --config "${config}" \
    --silent --no-interactive <"${state_file}"
}
