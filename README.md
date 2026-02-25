# Terraform/OpenTofu Provider Zabbix

Provider Zabbix base sur le Terraform Plugin Framework.

## Ressources incluses

- `zabbix_host`
- `zabbix_host_group`
- `zabbix_template`
- `zabbix_trigger`

`zabbix_host` supporte les IDs et les noms pour les liens :

- `host_group_ids` et/ou `host_group_names`
- `template_ids` et/ou `template_names`

SNMP v2 est supporte sur `zabbix_host.interfaces` avec :

- `type = 2`
- `snmp_details { version = 2, community = "..." }`

## Build

```bash
go mod tidy
go build ./...
```

## Import

```bash
tofu import zabbix_host.ubuntu01 12345
tofu import zabbix_host_group.linux 12
tofu import zabbix_template.custom_icmp 10001
tofu import zabbix_trigger.icmp_loss 40055
```
