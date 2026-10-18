provider "github" {
  owner = var.github_owner
}

locals {
  repository = var.github_repository
}

resource "github_repository_environment" "release" {
  environment = var.environment_name
  repository  = local.repository

  dynamic "reviewers" {
    for_each = length(var.reviewer_usernames) > 0 || var.reviewer_team_slug != "" ? [1] : []
    content {
      users = var.reviewer_usernames
      teams = var.reviewer_team_slug != "" ? [data.github_team.release_reviewers[0].id] : []
    }
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
