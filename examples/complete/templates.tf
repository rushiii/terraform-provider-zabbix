# host_group_ids of a template are template group IDs ("1" = "Templates" on Zabbix 7.x).
resource "zabbix_template" "snmp_switch" {
  host           = "tpl-switch-snmp"
  name           = "Switch SNMP maison"
  host_group_ids = ["1"]
  macros         = { "{$SNMP_COMMUNITY}" = "public" }
}
resource "zabbix_item" "tpl_uptime" {
  host_id    = zabbix_template.snmp_switch.id
  name       = "Uptime"
  key        = "sysUpTime"
  type       = 20
  value_type = 3
  snmp_oid   = "1.3.6.1.2.1.1.3.0"
  delay      = "5m"
}
