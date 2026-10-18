provider "github" {
  owner = var.github_owner
}

locals {
  repository = var.github_repository
}

data "github_user" "reviewer" {
  for_each = toset(var.reviewer_usernames)
  username = each.value
}

resource "github_repository_environment" "release" {
  environment         = var.environment_name
  repository          = local.repository
  can_admins_bypass   = false
  prevent_self_review = false

  dynamic "reviewers" {
    for_each = length(var.reviewer_usernames) > 0 || var.reviewer_team_slug != "" ? [1] : []
    content {
      users = [for name in var.reviewer_usernames : data.github_user.reviewer[name].id]
      teams = var.reviewer_team_slug != "" ? [data.github_team.release_reviewers[0].id] : []
    }
  }

  deployment_branch_policy {
    protected_branches     = false
    custom_branch_policies = true
  }
}

data "github_team" "release_reviewers" {
  count = var.reviewer_team_slug != "" ? 1 : 0
  slug  = var.reviewer_team_slug
}

resource "github_repository_environment_deployment_policy" "release_branch" {
  repository     = local.repository
  environment    = github_repository_environment.release.environment
  branch_pattern = var.release_branch_pattern
}
