output "environment_name" {
  description = "GitHub Actions environment name for release jobs."
  value       = github_repository_environment.release.environment
}

output "reviewer_usernames" {
  description = "Configured user reviewers for the release environment."
  value       = var.reviewer_usernames
}

output "reviewer_team_slug" {
  description = "Configured team reviewer slug, if any."
  value       = var.reviewer_team_slug
}
