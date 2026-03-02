terraform {
  required_providers {
    zabbix = {
      source  = "rushiii/zabbix"
      version = "0.1.16"
    }
  }
}

provider "zabbix" {
  url       = "https://zabbix.example.com/api_jsonrpc.php"
  api_token = var.zabbix_api_token
  # Alternative authentication:
  # username = var.zabbix_username
  # password = var.zabbix_password
}
