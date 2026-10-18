# shellcheck shell=bash

PLAN_FILES=()

register_plan_file() {
  PLAN_FILES+=("$1")
}

cleanup_plan_files() {
  local f
  for f in "${PLAN_FILES[@]:-}"; do
    rm -f "${f}"
  done
}

install_plan_file_trap() {
  trap cleanup_plan_files EXIT
}
