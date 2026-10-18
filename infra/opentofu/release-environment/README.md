# Release GitHub environment (OpenTofu)

This stack creates the GitHub Actions **release** environment for `QueerGlobal/hub-framework`, configures required reviewers, and restricts deployments to the branch pattern used by `.github/workflows/release.yml`.

## Prerequisites

- OpenTofu **1.7+** (state encryption) — tested with **1.10.6**
- Remote state: S3 bucket (see `backend-config.example.hcl`)
- GitHub token with **Administration: Read and write** on the repository (manage environments and deployment policies)

### GitHub token (Doppler)

Use the **`GITHUB_TOFU_RELEASE_ENV`** secret from Doppler project **`hub-framework`**, config **`infra`**. That token must include **repo → Administration: Read and write** (classic PAT) or equivalent fine-grained **Administration** access on `QueerGlobal/hub-framework`.

Export it before plan/apply:

```bash
export GITHUB_TOKEN="$(doppler secrets get GITHUB_TOFU_RELEASE_ENV --plain -p hub-framework -c infra)"
export TF_VAR_state_encryption_passphrase="$(doppler secrets get TOFU_STATE_ENCRYPTION_PASSPHRASE --plain -p hub-framework -c infra)"
```

## One-time setup (before the first Release workflow run)

A repository **admin** (for example the QueerGlobal infra owner) applies this stack once so the `release` environment, reviewers, and `main`-only deployment policy exist before anyone runs **Actions → Release**.

Entry point:

```bash
cd infra/opentofu/release-environment
cp backend-config.example.hcl backend-config.hcl   # edit bucket/region
tofu init -backend-config=backend-config.hcl
tofu plan
tofu apply
```

After a successful apply, required reviewers (OpenTofu-managed GitHub user IDs) gate the Release workflow.

## Variables

| Variable | Default | Purpose |
| --- | --- | --- |
| `github_owner` | `QueerGlobal` | Repository owner |
| `github_repository` | `hub-framework` | Repository name |
| `environment_name` | `release` | Must match `environment:` in `.github/workflows/release.yml` |
| `reviewer_usernames` | `["mnyon-grandkru"]` | GitHub usernames resolved to numeric user IDs for reviewers |
| `reviewer_team_slug` | `""` | Optional org team slug to require as reviewer |
| `release_branch_pattern` | `main` | Branch allowed to use the release environment |
| `state_encryption_passphrase` | (required) | PBKDF2 passphrase for encrypted remote state |

Copy and edit `terraform.tfvars.example` when you need non-default reviewers.
