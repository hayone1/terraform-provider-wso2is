terraform {
  required_version = ">= 1.6"

  required_providers {
    hcloud = {
      source  = "hetznercloud/hcloud"
      version = "~> 1.70"
    }
  }
}

# The provider reads the API token from the HCLOUD_TOKEN environment variable
# (set it in the repository's .env file; Task loads it).
provider "hcloud" {}
