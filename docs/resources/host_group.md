---
page_title: "zabbix_host_group Resource"
subcategory: ""
description: |-
  Manages a Zabbix host group.
---

# zabbix_host_group (Resource)

Creates, reads, updates, and deletes a Zabbix host group.

## Example Usage

```terraform
resource "zabbix_host_group" "linux" {
  name = "Linux servers"
}
```

## Schema

### Required

- `name` (String) Host group name.

### Optional

- `id` (String) Internal resource ID.

## Import

```bash
tofu import zabbix_host_group.linux 12
```
