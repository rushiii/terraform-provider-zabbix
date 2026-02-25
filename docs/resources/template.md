---
page_title: "zabbix_template Resource"
subcategory: ""
description: |-
  Gere un template Zabbix et son rattachement aux host groups.
---

# zabbix_template (Resource)

Permet de gerer un template Zabbix.

## Example Usage

```terraform
resource "zabbix_template" "custom_icmp" {
  host = "Template Custom ICMP"
  name = "Template Custom ICMP"

  host_group_ids = [
    "2",
  ]
}
```

## Schema

### Required

- `host` (String) Nom interne du template (champ `host` Zabbix).
- `host_group_ids` (Set of String) IDs des groupes auxquels rattacher le template.

### Optional

- `id` (String) ID de la ressource.
- `name` (String) Nom visible du template.

## Comportement

- Si `name` est omis ou vide, il prend la meme valeur que `host`.
- Les `host_group_ids` doivent etre des IDs existants cote Zabbix.

## Import

```bash
tofu import zabbix_template.custom_icmp 10001
```
