resource "zabbix_user" "tech" {
  username       = "jdupont"
  name           = "Jean Dupont"
  password       = var.user_password
  user_group_ids = [zabbix_user_group.techs.id]
  email          = "jdupont@example.fr"
}
