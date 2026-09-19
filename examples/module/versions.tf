terraform {
  required_version = ">= 1.3"

  required_providers {
    # The provider's local name is `anomaly`; resources are named `anomaly_*`.
    # The Terraform Registry address must match the one in the root module.
    anomaly = {
      source  = "nixys/nxs-anomaly"
      version = ">= 1.1.2, < 2.0.0"
    }
  }
}
