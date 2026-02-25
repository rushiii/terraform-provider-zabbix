resource "zabbix_host" "switch_snmp_v2" {
  name         = "sw-core-01"
  visible_name = "Switch Core 01"
  enabled      = true

  host_group_names = ["Network devices"]
  template_names   = ["Template Module Interfaces SNMP"]

  interfaces {
    type   = 2
    main   = true
    use_ip = true
    ip     = "10.20.30.50"
    port   = "161"

    snmp_details {
      version   = 2
      community = "{$SNMP_COMMUNITY}"
    }
  }
}
