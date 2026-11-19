resource "zabbix_item" "if_in" {
  host_id    = zabbix_host.switch.id
  name       = "Trafic entrant port 1"
  key        = "if.in.1"
  type       = 20
  value_type = 3
  snmp_oid   = "1.3.6.1.2.1.2.2.1.10.1"
  delay      = "1m"
  delay_flex = "30s/1-5,08:00-18:00"
}
