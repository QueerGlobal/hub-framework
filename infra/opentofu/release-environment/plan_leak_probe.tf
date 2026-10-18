variable "plan_leak_sensitive_marker" {
  type        = string
  sensitive   = true
  default     = ""
  description = "CI-only sensitive marker for plan encryption tests (leave empty for normal runs)."
}

resource "terraform_data" "plan_leak_probe" {
  count = var.plan_leak_sensitive_marker != "" ? 1 : 0

  input = var.plan_leak_sensitive_marker
}
