# A complete configuration split over several files: OpenTofu/Terraform loads every .tf file
# of the directory as one configuration, so a single `tofu apply` creates everything.

terraform {
  required_providers {
    zabbix = {
      source  = "rushiii/zabbix"
      version = "~> 1.3"
    }
  }
}

variable "zabbix_url" {
  type = string
}

variable "zabbix_api_token" {
  type      = string
  sensitive = true
}

variable "user_password" {
  type      = string
  sensitive = true
}

provider "zabbix" {
  url       = var.zabbix_url
  api_token = var.zabbix_api_token
}
