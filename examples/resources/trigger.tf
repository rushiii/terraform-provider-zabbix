resource "zabbix_trigger" "icmp_loss" {
  description = "ICMP ping loss on ubuntu01"
  expression  = "{ubuntu01:icmpping.max(5m)}=0"
  priority    = "4" # high
  enabled     = true
}
