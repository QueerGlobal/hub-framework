variable "github_owner" {
  type        = string
  description = "GitHub organization or user that owns the repository."
  default     = "QueerGlobal"
}

variable "github_repository" {
  type        = string
  description = "Repository name (without owner)."
  default     = "hub-framework"
}

variable "environment_name" {
  type        = string
  description = "GitHub Actions environment name used by the Release workflow."
  default     = "release"
}

variable "reviewer_usernames" {
  type        = list(string)
  description = "GitHub usernames who must approve deployments to the release environment."
  default     = ["mnyon-grandkru"]
}

variable "reviewer_team_slug" {
  type        = string
  description = "Optional GitHub team slug (within github_owner) that must approve release deployments. Leave empty to skip team reviewers."
  default     = ""
}

variable "release_branch_pattern" {
  type        = string
  description = "Branch pattern allowed to deploy to the release environment (for example main)."
  default     = "main"
}
