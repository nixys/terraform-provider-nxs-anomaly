terraform {
  required_providers {
    anomaly = {
      source  = "nixys/nxs-anomaly"
      version = "~> 0.0"
    }
  }
}

provider "anomaly" {
  # URL of the nxs-anomaly API.
  # Can also be set via NXS_ANOMALY_URL environment variable.
  url = "http://localhost:8080"

  # API key for authentication.
  # Can also be set via NXS_ANOMALY_API_KEY environment variable.
  api_key = var.anomaly_api_key
}

variable "anomaly_api_key" {
  type      = string
  sensitive = true
  default   = ""
}
