# Release GitHub environment (OpenTofu)

This stack creates the GitHub Actions **release** environment for `QueerGlobal/hub-framework`, configures required reviewers, and restricts deployments to the branch pattern used by `.github/workflows/release.yml`.

Remote state lives in **Cloudflare R2** (S3-compatible). The bucket and scoped credentials are created by `infra/opentofu/state-bootstrap` and written to Doppler by `scripts/bootstrap-release-infra.sh`.

**Important:** The **Release** GitHub Actions workflow (`.github/workflows/release.yml`) does **not** run OpenTofu and does **not** read Doppler, R2, or `AWS_*` credentials. It uses `github.token` only to create tags and GitHub releases after a human approves the `release` environment. OpenTofu state and credentials exist **only** for repository admins running the bootstrap script locally (or in an admin CI job), not for routine releases.

## Prerequisites

- OpenTofu **1.10.0+** (state/plan encryption and `use_lockfile` on the S3-compatible backend), tested with **1.10.6**
- [Doppler CLI](https://docs.doppler.com/docs/cli) logged in, with access to project **`hub-framework`**, config **`infra`**
- **`GITHUB_TOFU_RELEASE_ENV`** in Doppler config **`bootstrap-admin`** before bootstrap (repo **Administration: Read and write**)

## Manual, no API

These inputs cannot be created by the bootstrap script:

| Input | Reason |
| --- | --- |
| Cloudflare bootstrap API token | Cloudflare does not expose an API to mint the broad bootstrap token safely; you create it in the dashboard. |
| GitHub fine-grained `GITHUB_TOFU_RELEASE_ENV` | GitHub does not provide an API to create fine-grained personal access tokens; store the token in Doppler manually. |
| Doppler login | The Doppler CLI requires an interactive or prior `doppler login`; the script creates the `hub-framework` project, **`infra`**, and **`bootstrap-admin`** configs if they are missing. |

## One-time setup (before the first Release workflow run)

A repository **admin** runs the repo entry point once (idempotent; confirms before any write):

```bash
./scripts/bootstrap-release-infra.sh
```

That script:

1. Preflights Doppler, Cloudflare token permissions, account ID, and `GITHUB_TOFU_RELEASE_ENV`.
2. Applies **state-bootstrap** (R2 bucket `queerglobal-hub-framework-tofu-state`, bucket-scoped S3 credentials). **Bootstrap stack state stays local**; an encrypted backup is stored in Doppler config **`bootstrap-admin`**, which CI must never read.
3. Writes release-environment R2 backend settings and credentials to **Doppler** (`hub-framework` / `infra` only).
4. Inits/plans/applies **release-environment** using Doppler-backed `backend-config.hcl`.

### Cloudflare bootstrap token

See `infra/opentofu/state-bootstrap/README.md` for required Cloudflare permissions (API token administration plus R2 edit).

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
| `state_encryption_passphrase` | (required) | PBKDF2 passphrase for key provider **`state`** (`TOFU_STATE_ENCRYPTION_PASSPHRASE` in Doppler `infra`) |

State and plan encryption use key provider **`state`** with `enforced = true`.

## Passphrase rotation

Same two-hop `TF_ENCRYPTION` flow as bootstrap; provider name is **`state`**:

```bash
TOFU_ROTATE_OLD_PASSPHRASE="${TOFU_STATE_ENCRYPTION_PASSPHRASE}" \
TOFU_ROTATE_NEW_PASSPHRASE="${TOFU_STATE_ENCRYPTION_PASSPHRASE_NEXT}" \
  ./scripts/tofu-rotate-encryption-passphrase.sh infra/opentofu/release-environment state
```

Then update `TOFU_STATE_ENCRYPTION_PASSPHRASE` in Doppler `infra` and clear `_NEXT`. Requires **OpenTofu 1.13.1+**. See `scripts/test/encryption-rotation-test.sh`.

Copy and edit `terraform.tfvars.example` when you need non-default reviewers.
