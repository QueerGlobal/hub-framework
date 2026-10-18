variable "plan_leak_sensitive_marker" {
  type        = string
  default     = ""
  description = "Test-only marker for plan encryption leak checks."
  sensitive   = true
}

resource "terraform_data" "plan_leak_probe" {
  count = var.plan_leak_sensitive_marker != "" ? 1 : 0

  input = var.plan_leak_sensitive_marker
}
