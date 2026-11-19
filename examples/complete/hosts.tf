resource "zabbix_host" "tv" {
  name           = "fst-tv-atela.example.fr"
  visible_name   = "fst-tv-atela"
  description    = "TV salle Atela"
  host_group_ids = [zabbix_host_group.visiosense.id]
  template_names = ["ICMP Ping"]
  tags           = { site = "atela" }
  interfaces {
    type = 1
    ip   = "172.17.32.22"
  }
}
resource "zabbix_host" "switch" {
  name           = "sw-core-01"
  visible_name   = "Switch coeur"
  host_group_ids = [zabbix_host_group.network.id]
  template_ids   = [zabbix_template.snmp_switch.id]
  interfaces {
    type = 2
    ip   = "172.17.32.1"
    port = "161"
    snmp_details {
      version   = 2
      community = "{$SNMP_COMMUNITY}"
    }
  }
}
