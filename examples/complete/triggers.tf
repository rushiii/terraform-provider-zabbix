# The expression uses references (zabbix_host.switch.name, zabbix_item.if_in.key) rather than
# "sw-core-01" and "if.in.1": the trigger is then created after the host and the item.
resource "zabbix_trigger" "if_in_high" {
  description = "Trafic entrant élevé sur sw-core-01"
  expression  = "last(/${zabbix_host.switch.name}/${zabbix_item.if_in.key})>1000000"
  priority    = "3"
}
resource "zabbix_host_trigger_status" "tv_latency" {
  host_id = zabbix_host.tv.id
  name    = "ICMP Ping: High ICMP ping response time"
  enabled = false
}
