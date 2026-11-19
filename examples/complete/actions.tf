data "zabbix_user_group" "admins" {
  name = "Zabbix administrators"
}
resource "zabbix_action" "visiosense" {
  name           = "Alertes Visiosense"
  user_group_ids = [zabbix_user_group.techs.id, data.zabbix_user_group.admins.id]
  user_ids       = [zabbix_user.tech.id]
  host_group_ids = [zabbix_host_group.visiosense.id]
  subject        = "Problème : {TRIGGER.NAME} sur {HOST.NAME}"
  message        = "Sévérité : {TRIGGER.SEVERITY}"
}
