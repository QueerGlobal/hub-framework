# shellcheck shell=bash

# Doppler configs for hub-framework OpenTofu bootstrap.
# infra: release-environment secrets (safe for future read-only CI checks with mocks).
# bootstrap-admin: bootstrap stack secrets and local state backups (must never be readable from CI).

DOPPLER_PROJECT_NAME="${DOPPLER_PROJECT:-hub-framework}"
DOPPLER_INFRA_CONFIG="${DOPPLER_CONFIG:-infra}"
DOPPLER_BOOTSTRAP_ADMIN_CONFIG="${DOPPLER_BOOTSTRAP_CONFIG:-bootstrap-admin}"
