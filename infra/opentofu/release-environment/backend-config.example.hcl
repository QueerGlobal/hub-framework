# Copy to backend-config.hcl (gitignored) and run: tofu init -backend-config=backend-config.hcl
bucket       = "your-org-opentofu-state"
key          = "hub-framework/release-environment/terraform.tfstate"
region       = "us-east-1"
encrypt      = true
use_lockfile = true
