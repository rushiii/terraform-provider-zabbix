---
page_title: "zabbix_host Resource"
subcategory: ""
description: |-
  Manages a Zabbix host, including interfaces, group/template links, and tags.
---

# zabbix_host (Resource)

Resource to create and manage a Zabbix host.

This resource supports:

- linking host groups by IDs or names
- linking templates by IDs or names
- agent/SNMP/IPMI/JMX interfaces
- SNMP v2 details for interfaces with `type = 2`
- host tags

## Example Usage

### Host ICMP/Agent

```terraform
resource "zabbix_host" "name" {
  name         = "name"
  visible_name = "name"
  enabled      = true

  host_group_names = ["Linux servers"]
  template_names   = ["Template Module ICMP Ping"]

  interfaces {
    type   = 1
    main   = true
    use_ip = true
    ip     = "10.20.30.40"
    port   = "10050"
  }
}
```

### Host SNMP v2

```terraform
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
```

## Schema

### Required

- `name` (String) Technical host name (`host` in Zabbix).

### Optional

- `enabled` (Boolean) Whether the host is enabled. Default: `true`.
- `host_group_ids` (Set of String) Host group IDs.
- `host_group_names` (Set of String) Host group names (resolved to IDs).
- `id` (String) Resource ID.
- `interfaces` (Block List) Host interfaces.
- `tags` (Map of String) Host tags as `tag => value`.
- `template_ids` (Set of String) Template IDs to link.
- `template_names` (Set of String) Template names to link (resolved to IDs).
- `visible_name` (String) Display name (`name`) in Zabbix.

## Important rules

- You must provide at least `host_group_ids` or `host_group_names`.
- ID and name lists are merged and deduplicated.
- When `interfaces.use_ip = true`, `ip` is required.
- When `interfaces.use_ip = false`, `dns` is required.
- If `interfaces.type = 2` (SNMP), only version `2` is currently supported.
- If `snmp_details` is omitted on an SNMP interface, defaults are:
  - `version = 2`
  - `community = "{$SNMP_COMMUNITY}"`

### Nested Schema for `interfaces`

Required:

- `type` (Number) Interface type: `1=Agent`, `2=SNMP`, `3=IPMI`, `4=JMX`.

Optional:

- `main` (Boolean) Main interface for this type. Default: `true`.
- `use_ip` (Boolean) Use IP (`true`) or DNS (`false`). Default: `true`.
- `ip` (String) IP address (required when `use_ip = true`).
- `dns` (String) DNS name (required when `use_ip = false`).
- `port` (String) Destination port. Default: `"10050"`.
- `snmp_details` (Block) SNMP details.

### Nested Schema for `interfaces.snmp_details`

Optional:

- `version` (Number) SNMP version. Default: `2` (only accepted value currently).
- `community` (String) SNMP community. Default: `"{$SNMP_COMMUNITY}"`.

## Import

```bash
tofu import zabbix_host.name 12345
```

## Notes

- During update, interfaces are replaced as a full set via
  `hostinterface.replacehostinterfaces`.
- In practice, Zabbix usually requires at least one valid interface
  for a monitorable host.
