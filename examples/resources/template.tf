resource "zabbix_template" "custom_icmp" {
  host = "Template Custom ICMP"
  name = "Template Custom ICMP"

  host_group_ids = [
    "2", # Templates
  ]
}
