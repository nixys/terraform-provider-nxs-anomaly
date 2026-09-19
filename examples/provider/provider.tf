terraform {
  required_providers {
    anomaly = {
      source  = "nixys/nxs-anomaly"
      version = "~> 1.1"
    }
  }
}

provider "anomaly" {
  # URL of the nxs-anomaly API. A value set here wins over the
  # NXS_ANOMALY_URL environment variable; omit the argument to use it.
  url = "http://localhost:8080"

  # API key for authentication. Left null, NXS_ANOMALY_API_KEY is used: any
  # other value, including "", wins over the environment variable.
  api_key = var.anomaly_api_key
}

variable "anomaly_api_key" {
  description = "API key. Left null, the provider reads NXS_ANOMALY_API_KEY."
  type        = string
  sensitive   = true
  default     = null
}
