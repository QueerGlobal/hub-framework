# OpenTofu state bootstrap (Cloudflare R2)

Creates the shared **R2** bucket and a **bucket-scoped** S3-compatible API token used only by the **release-environment** stack. **Bootstrap stack state stays on disk** (local backend) and is backed up to Doppler config **`bootstrap-admin`**, which CI must never read.

Requires **OpenTofu 1.10.0+** for day-to-day use (**1.13.1+** for passphrase rotation). State and plan encryption use key provider **`bootstrap`** with `enforced = true`.

## Cloudflare bootstrap token permissions

Export `CLOUDFLARE_API_TOKEN` before running `scripts/bootstrap-release-infra.sh`. Preflight is **capability-only** (no token policy reads and no account token list API):

1. **Verify** the token is **active**: `GET /accounts/{account_id}/tokens/verify`, then `GET /user/tokens/verify` if needed.
2. **R2 list buckets**: `GET /accounts/{account_id}/r2/buckets?per_page=1` (proves account-scoped R2 access; does not prove Write).
3. **Permission groups read**: `GET /accounts/{account_id}/tokens/permission_groups?per_page=1` (same read the bootstrap stack uses via `data.cloudflare_api_token_permission_groups`).

**R2 Write** cannot be proven with a list call. If the token only has R2 Read, preflight may still pass; the **first OpenTofu apply** fails closed when creating the bucket (no durable resource change). See `scripts/test/cloudflare-r2-write-fails-closed-test.sh`.

Dashboard: **Workers R2 Storage Write** and **Account API Tokens Write** at account scope (API names use **Write**, not dashboard "Edit").

## Doppler configs

| Config | Purpose | CI |
| --- | --- | --- |
| `infra` | Release stack remote state, `GITHUB_TOFU_RELEASE_ENV`, release passphrases, `DOPPLER_CI_INFRA_SERVICE_TOKEN` | `DOPPLER_TOKEN` must be a service token scoped to **infra** only |
| `bootstrap-admin` | Bootstrap passphrases, `TOFU_BOOTSTRAP_STATE_LOCAL_BACKUP` | Must not be readable from CI (`scripts/test/doppler-ci-runtime-guard.sh`) |

Release R2 credentials in `infra` cannot overwrite bootstrap state because bootstrap state is **not** stored under those credentials on R2.

## Manual, no API

| Input | Reason |
| --- | --- |
| Cloudflare bootstrap API token | No API to mint the broad bootstrap token safely. |
| GitHub fine-grained `GITHUB_TOFU_RELEASE_ENV` | No API to create fine-grained PATs; export or store in Doppler before bootstrap. |
| Doppler login | Requires `doppler login`; script creates project/configs idempotently after read-only preflight. |

## Bootstrap state and incident response

- Primary copy: encrypted **`terraform.tfstate`** locally (`backend_local.tf`).
- Backup: encrypted state blob in Doppler **`bootstrap-admin`** (`TOFU_BOOTSTRAP_STATE_LOCAL_BACKUP`).
- **Do not revoke** the only R2 token that still protects release state without minting a replacement and updating Doppler `infra` first. Revoking R2 credentials does **not** delete local bootstrap state or the Doppler backup.

## Passphrase rotation

Committed config keeps a **single** key provider **`bootstrap`** (no fallback). Rotation is two refresh-only applies via `TF_ENCRYPTION` so tracked `.tf` files stay unchanged; state metadata remains under **`bootstrap`**.

1. Store the next passphrase in Doppler (for example `TOFU_BOOTSTRAP_STATE_ENCRYPTION_PASSPHRASE_NEXT`) and export the current value from `TOFU_BOOTSTRAP_STATE_ENCRYPTION_PASSPHRASE`.
2. From the repo root:

```bash
./scripts/tofu-rotate-encryption-passphrase.sh infra/opentofu/state-bootstrap bootstrap \
  "${TOFU_BOOTSTRAP_STATE_ENCRYPTION_PASSPHRASE}" "${TOFU_BOOTSTRAP_STATE_ENCRYPTION_PASSPHRASE_NEXT}"
```

3. Update `TOFU_BOOTSTRAP_STATE_ENCRYPTION_PASSPHRASE` in `bootstrap-admin` to the new value and remove `_NEXT`.

Requires **OpenTofu 1.13.1+**. Proof: `scripts/test/encryption-rotation-test.sh`.

## Run

```bash
./scripts/bootstrap-release-infra.sh
```

Account ID from `CLOUDFLARE_ACCOUNT_ID` or Doppler `infra` / `CLOUDFLARE_ACCOUNT_ID`.
