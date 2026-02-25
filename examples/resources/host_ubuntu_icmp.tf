resource "zabbix_host" "ubuntu01" {
  name         = "ubuntu01"
  visible_name = "Ubuntu 22.04 - Prod"
  enabled      = true

  host_group_names = ["Linux servers"]

  template_names = ["Template Module ICMP Ping",]

  interfaces {
    type   = 1
    main   = true
    use_ip = true
    ip     = "10.20.30.40"
    port   = "10050"
  }
}
