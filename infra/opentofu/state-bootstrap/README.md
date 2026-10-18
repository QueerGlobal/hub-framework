# OpenTofu state bootstrap (Cloudflare R2)

Creates the shared **R2** bucket and a **bucket-scoped** S3-compatible API token for OpenTofu remote state. Bootstrap state stays **local** with OpenTofu encryption.

## What Mark supplies

Only the **Cloudflare bootstrap API token** (export as `CLOUDFLARE_API_TOKEN`):

- Permission: **R2 Admin Read & Write** on the QueerGlobal account (create buckets and create scoped R2 API tokens).

Everything else (account ID default in `terraform.tfvars.example`, encryption passphrases, GitHub token, generated backend config) is read from or written to **Doppler** (`hub-framework` / `infra`) by `scripts/bootstrap-release-infra.sh`.

## Run

Use the repo entry point (do not run this stack by hand unless debugging):

```bash
./scripts/bootstrap-release-infra.sh
```
