terraform {
  required_providers {
    zabbix = {
      source  = "RuShIII/terraform-provider-zabbix"
      version = "0.1.0"
    }
  }
}

provider "zabbix" {
  url       = "https://zabbix.example.com/api_jsonrpc.php"
  api_token = var.zabbix_api_token
  # Alternative:
  # username = var.zabbix_username
  # password = var.zabbix_password
}
