# Release GitHub environment (OpenTofu)

This stack creates the GitHub Actions **release** environment for `QueerGlobal/hub-framework`, configures required reviewers, and restricts deployments to the branch pattern used by `.github/workflows/release.yml`.

Remote state lives in **Cloudflare R2** (S3-compatible). The bucket and scoped credentials are created by `infra/opentofu/state-bootstrap` and written to Doppler by `scripts/bootstrap-release-infra.sh`.

**Important:** The **Release** GitHub Actions workflow (`.github/workflows/release.yml`) does **not** run OpenTofu and does **not** read Doppler, R2, or `AWS_*` credentials. It uses `github.token` only to create tags and GitHub releases after a human approves the `release` environment. OpenTofu state and credentials exist **only** for repository admins running the bootstrap script locally (or in an admin CI job), not for routine releases.

## Prerequisites

- OpenTofu **1.8+** (variable-based state encryption), tested with **1.10.6**
- [Doppler CLI](https://docs.doppler.com/docs/cli) with access to project **`hub-framework`**, config **`infra`**
- After bootstrap: **`GITHUB_TOFU_RELEASE_ENV`** in Doppler (repo **Administration: Read and write**)

## One-time setup (before the first Release workflow run)

A repository **admin** runs the repo entry point once (idempotent; confirms before any write):

```bash
./scripts/bootstrap-release-infra.sh
```

That script:

1. Applies **state-bootstrap** (R2 bucket `queerglobal-hub-framework-tofu-state`, bucket-scoped S3 credentials).
2. Writes backend settings and credentials to **Doppler** (`hub-framework` / `infra`).
3. Inits/plans/applies **release-environment** using Doppler-backed `backend-config.hcl`.

### What Mark supplies

Only the **Cloudflare bootstrap API token** (`CLOUDFLARE_API_TOKEN`): **R2 Admin Read & Write** on the QueerGlobal account. Export it before running the script. Account ID is read from Doppler `CLOUDFLARE_ACCOUNT_ID` or `CLOUDFLARE_ACCOUNT_ID` in the environment.

All other secrets (bootstrap/release state passphrases, R2 state keys, GitHub token) are created or read via Doppler by the script.

After a successful apply, required reviewers (OpenTofu-managed GitHub user IDs) gate the Release workflow approval step (the workflow itself still only uses `github.token` for tagging).

## Variables

| Variable | Default | Purpose |
| --- | --- | --- |
| `github_owner` | `QueerGlobal` | Repository owner |
| `github_repository` | `hub-framework` | Repository name |
| `environment_name` | `release` | Must match `environment:` in `.github/workflows/release.yml` |
| `reviewer_usernames` | `["mnyon-grandkru"]` | GitHub usernames resolved to numeric user IDs for reviewers |
| `reviewer_team_slug` | `""` | Optional org team slug to require as reviewer |
| `release_branch_pattern` | `main` | Branch allowed to use the release environment |
| `state_encryption_passphrase` | (required) | PBKDF2 passphrase for encrypted remote state (`TOFU_STATE_ENCRYPTION_PASSPHRASE` in Doppler) |

Copy and edit `terraform.tfvars.example` when you need non-default reviewers.
