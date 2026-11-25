---
page_title: "zabbix_host_group_membership Resource"
subcategory: ""
description: |-
  Adds existing hosts to a host group without managing the hosts themselves.
---

# zabbix_host_group_membership (Resource)

Adds hosts that already exist in Zabbix (created in the Zabbix UI, by discovery, or by
another tool) to a host group. Only the membership of the listed hosts in this group is
managed: their other groups, interfaces, templates and settings are left as they are.

To manage everything about an existing host instead, import it into a `zabbix_host`
(see the `zabbix_host` page).

## Example Usage

```terraform
resource "zabbix_host_group" "visiosense" {
  name = "Visiosense"
}

data "zabbix_host" "tv1" {
  name = "fst-tv-atela"
}

data "zabbix_host" "tv2" {
  name = "fst-tv-salle-b"
}

resource "zabbix_host_group_membership" "visiosense" {
  host_group_id = zabbix_host_group.visiosense.id
  host_ids      = [data.zabbix_host.tv1.id, data.zabbix_host.tv2.id]
}
```

## Schema

### Required

- `host_group_id` (String) ID of the host group. Changing it replaces the resource.
- `host_ids` (Set of String) IDs of the hosts to put in the group (at least one).

### Read-Only

- `id` (String) Same as `host_group_id`.

## Behavior

- Adding a host to `host_ids` adds it to the group (`hostgroup.massadd`); removing it removes
  it from the group (`hostgroup.massremove`). Its other groups are not changed.
- Destroying the resource removes the listed hosts from the group.
- Other hosts of the group are ignored: several `zabbix_host_group_membership` resources, or
  hosts added in the UI, can share the same group.
- Zabbix refuses to remove a host from its last group: add it to another group first.
- Do not use this resource for a host managed by `zabbix_host`: `zabbix_host` manages all the
  groups of its host and would remove the group again. Add the group to its `host_group_ids`
  instead.

## Import

The import ID is the host group ID; all hosts of the group are imported:

```bash
tofu import zabbix_host_group_membership.visiosense 34
```
