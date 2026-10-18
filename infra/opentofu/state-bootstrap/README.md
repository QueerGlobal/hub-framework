# OpenTofu state bootstrap (Cloudflare R2)

Creates the shared **R2** bucket and a **bucket-scoped** S3-compatible API token used only by the **release-environment** stack. **Bootstrap stack state stays on disk** (local backend) and is backed up to Doppler config **`bootstrap-admin`**, which CI must never read.

Requires **OpenTofu 1.10.0+** (state/plan encryption with `enforced = true` and passphrase rotation via `bootstrap_primary` / fallback `bootstrap`).

## Cloudflare bootstrap token permissions

Export `CLOUDFLARE_API_TOKEN` before running `scripts/bootstrap-release-infra.sh`. Preflight uses capability probes with the token itself, then optional policy checks by permission group **id** (allow policies only; deny is ignored):

1. `GET /user/tokens/verify`, or `GET /accounts/{account_id}/tokens/verify` when user verify fails (account-owned tokens)
2. `GET /accounts/{account_id}/r2/buckets?per_page=1` (R2 account access)
3. `GET /accounts/{account_id}/tokens?per_page=1` (Account API Tokens Read/Write namespace)
4. `GET /accounts/{account_id}/tokens/permission_groups` (catalog; resolves **Write** group names such as **Workers R2 Storage Write** and **Account API Tokens Write**, not dashboard "Edit" labels)
5. When token details are readable (`GET /user/tokens/{id}` or `GET /accounts/{account_id}/tokens/{id}`), confirm allow policies include the catalog ids for R2 Storage Write and Account API Tokens Write. This step needs **User API Tokens Read** or **Account API Tokens Read** on the bootstrap token.

Dashboard templates: **Account API Tokens Write** plus **Workers R2 Storage Write** at account scope.

## Doppler configs

| Config | Purpose | CI |
| --- | --- | --- |
| `infra` | Release stack remote state, `GITHUB_TOFU_RELEASE_ENV`, release passphrases | Mock vars only in `.github/workflows/infra.yml` |
| `bootstrap-admin` | Bootstrap passphrases, `TOFU_BOOTSTRAP_STATE_LOCAL_BACKUP` | Must not be referenced from CI |

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

Encryption uses primary key provider **`bootstrap_primary`** and fallback **`bootstrap`** (original name) with `enforced = true` on state and plan. To rotate, set a new `TOFU_BOOTSTRAP_STATE_ENCRYPTION_PASSPHRASE` in `bootstrap-admin`, keep the previous value in `TOFU_BOOTSTRAP_STATE_ENCRYPTION_PASSPHRASE_LEGACY`, then run `tofu apply -refresh-only` in this stack. See `scripts/test/encryption-rotation-test.sh`.

## Run

```bash
./scripts/bootstrap-release-infra.sh
```

Account ID from `CLOUDFLARE_ACCOUNT_ID` or Doppler `infra` / `CLOUDFLARE_ACCOUNT_ID`.
