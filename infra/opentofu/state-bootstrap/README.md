# OpenTofu state bootstrap (Cloudflare R2)

Creates the shared **R2** bucket and a **bucket-scoped** S3-compatible API token for OpenTofu remote state. After the first apply, bootstrap state is migrated to the same R2 bucket under its own key (see below).

Requires **OpenTofu 1.10.0+** (state/plan encryption and `use_lockfile` on the S3-compatible backend).

## Cloudflare bootstrap token permissions

Export `CLOUDFLARE_API_TOKEN` before running `scripts/bootstrap-release-infra.sh`. The token must be able to verify, list account permission groups, create R2 buckets, and mint scoped API tokens. In the Cloudflare dashboard this usually means:

- **API Tokens Read/Write** (the **Create additional tokens** template), or an account-owned token with **Account API Tokens Write**
- **R2** object/bucket edit at account scope (for example **Workers R2 Storage Write** or equivalent bucket create permission)

The script preflights `GET /user/tokens/verify` and `GET /accounts/{account_id}/tokens/permission_groups` before any infrastructure write.

## Manual, no API

These inputs cannot be created by the bootstrap script:

| Input | Reason |
| --- | --- |
| Cloudflare bootstrap API token | Cloudflare does not expose an API to mint the broad bootstrap token safely; you create it in the dashboard. |
| GitHub fine-grained `GITHUB_TOFU_RELEASE_ENV` | GitHub does not provide an API to create fine-grained personal access tokens; store the token in Doppler manually. |
| Doppler login | The Doppler CLI requires an interactive or prior `doppler login`; the script only creates the `hub-framework` project and `infra` config if they are missing. |

## Bootstrap state on R2

The first bootstrap apply uses a local backend (`tofu init -backend=false`). After the bucket and scoped credentials exist, the script runs `tofu init -migrate-state` with a generated `bootstrap-backend-config.hcl` (gitignored) so later runs use remote state at `hub-framework/state-bootstrap/terraform.tfstate` in the shared bucket, with the same encryption settings as in `versions.tf`.

## Run

Use the repo entry point (do not run this stack by hand unless debugging):

```bash
./scripts/bootstrap-release-infra.sh
```

Account ID is read from Doppler `CLOUDFLARE_ACCOUNT_ID` or `CLOUDFLARE_ACCOUNT_ID` in the environment. Encryption passphrases and backend settings are read from or written to **Doppler** (`hub-framework` / `infra`) by the script.
