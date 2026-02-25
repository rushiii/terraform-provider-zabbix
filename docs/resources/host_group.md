---
page_title: "zabbix_host_group Resource"
subcategory: ""
description: |-
  Gere un groupe d'hotes Zabbix.
---

# zabbix_host_group (Resource)

Cree, lit, met a jour et supprime un host group Zabbix.

## Example Usage

```terraform
resource "zabbix_host_group" "linux" {
  name = "Linux servers"
}
```

## Schema

### Required

- `name` (String) Nom du groupe d'hotes.

### Optional

- `id` (String) ID interne de la ressource.

## Import

```bash
tofu import zabbix_host_group.linux 12
```
