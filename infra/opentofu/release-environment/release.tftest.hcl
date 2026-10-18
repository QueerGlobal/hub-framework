mock_provider "github" {
  alias = "github"

  mock_data "github_user" {
    defaults = {
      id = 5750239
    }
  }

  mock_resource "github_repository_environment" {}

  mock_resource "github_repository_environment_deployment_policy" {}
}

run "release_environment_apply" {
  command = apply

  providers = {
    github = github.github
  }

  variables {
    state_encryption_passphrase = "mock-release-passphrase-16"
    github_owner                       = "QueerGlobal"
    github_repository                  = "hub-framework"
    environment_name                   = "release"
    reviewer_usernames                 = ["mnyon-grandkru"]
    reviewer_team_slug                 = ""
    release_branch_pattern             = "main"
  }

  assert {
    condition     = github_repository_environment.release.can_admins_bypass == false
    error_message = "can_admins_bypass must be false"
  }

  assert {
    condition     = length(tolist(github_repository_environment.release.reviewers[0].users)) == 1
    error_message = "expected one user reviewer"
  }

  assert {
    condition     = contains(github_repository_environment.release.reviewers[0].users, 5750239)
    error_message = "reviewer user id must be 5750239"
  }

  assert {
    condition     = github_repository_environment.release.deployment_branch_policy[0].custom_branch_policies == true
    error_message = "custom_branch_policies must be true"
  }

  assert {
    condition     = github_repository_environment.release.deployment_branch_policy[0].protected_branches == false
    error_message = "protected_branches must be false"
  }

  assert {
    condition     = var.environment_name == "release"
    error_message = "environment name must be release"
  }

  assert {
    condition     = var.release_branch_pattern == "main"
    error_message = "branch pattern must be main"
  }

  assert {
    condition     = var.github_repository == "hub-framework"
    error_message = "repository name must be hub-framework"
  }
}
