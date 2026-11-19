resource "zabbix_host_group" "visiosense" {
  name = "Visiosense"
}
resource "zabbix_host_group" "network" {
  name = "Reseau"
}
resource "zabbix_user_group" "techs" {
  name                = "Techniciens"
  host_group_read_ids = [zabbix_host_group.visiosense.id, zabbix_host_group.network.id]
}
