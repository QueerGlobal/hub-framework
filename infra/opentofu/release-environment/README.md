# Release GitHub environment (OpenTofu)

This stack creates the GitHub Actions **release** environment for `QueerGlobal/hub-framework` and configures required reviewers before the Release workflow can push tags or create GitHub releases.

## Prerequisites

- OpenTofu or Terraform 1.6+
- A GitHub personal access token or GitHub App token with `administration:write` on the repository (to manage environments and deployment policies)

Set `GITHUB_TOKEN` or configure the GitHub provider token.

## Apply

```bash
cd infra/opentofu/release-environment
tofu init
tofu plan
tofu apply
```

## Variables

| Variable | Default | Purpose |
| --- | --- | --- |
| `github_owner` | `QueerGlobal` | Repository owner |
| `github_repository` | `hub-framework` | Repository name |
| `environment_name` | `release` | Must match `environment:` in `.github/workflows/release.yml` |
| `reviewer_usernames` | `["mnyon-grandkru"]` | Users who must approve release deployments |
| `reviewer_team_slug` | `""` | Optional org team slug to require as reviewer |
| `release_branch_pattern` | `main` | Branch allowed to use the release environment |

Copy and edit `terraform.tfvars.example` when you need non-default reviewers.
