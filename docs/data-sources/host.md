---
page_title: "zabbix_host Data Source"
subcategory: ""
description: |-
  Looks up an existing Zabbix host by name.
---

# zabbix_host (Data Source)

Looks up a host that already exists in Zabbix, for example a host created in the Zabbix UI or
by network discovery, so that it can be referenced without being managed by Terraform.

## Example Usage

```terraform
data "zabbix_host" "srv" {
  name = "srv-manuel"
}

output "srv_id" {
  value = data.zabbix_host.srv.id
}
```

## Schema

### Required

- `name` (String) Technical name (`host`) of the host, or its visible name. The technical name
  is tried first.

### Read-Only

- `id` (String) Host ID.
- `host` (String) Technical name of the host.
- `visible_name` (String) Visible name of the host.
- `host_group_ids` (Set of String) IDs of the host groups the host belongs to.

An error is raised when no host matches, or when several hosts share the visible name.
