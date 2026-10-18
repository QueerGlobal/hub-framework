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

run "plan_leak" {
  command = plan

  providers = {
    github = github.github
  }

  variables {
    state_encryption_passphrase        = "plan-leak-release-passphrase-xyzzy"
    state_encryption_passphrase_legacy = "plan-leak-release-passphrase-xyzzy"
    github_owner                       = "QueerGlobal"
    github_repository                  = "hub-framework"
    environment_name                   = "release"
    reviewer_usernames                 = ["mnyon-grandkru"]
    reviewer_team_slug                 = ""
    release_branch_pattern             = "main"
  }
}
