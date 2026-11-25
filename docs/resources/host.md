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
- host description

## Example Usage

### Host ICMP/Agent

```terraform
resource "zabbix_host" "name" {
  name         = "name"
  visible_name = "name"
  description  = "Main web server, room B"
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

### Host group created in the same configuration

```terraform
resource "zabbix_host_group" "visiosense" {
  name = "Visiosense"
}

resource "zabbix_host" "tv" {
  name           = "fst-tv-atela.example.fr"
  host_group_ids = [zabbix_host_group.visiosense.id]
  template_names = ["ICMP Ping"]

  interfaces {
    type = 3
    ip   = "172.17.32.22"
    port = "623"
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

- `description` (String) Host description (`description` in Zabbix). When omitted, the description set in Zabbix is left unchanged; set `description = ""` to clear it.
- `enabled` (Boolean) Whether the host is enabled. Default: `true`.
- `host_group_ids` (Set of String) Host group IDs. Computed from `host_group_names` when not set.
- `host_group_names` (Set of String) Host group names, resolved to IDs at apply time. Computed from `host_group_ids` when not set.
- `id` (String) Resource ID.
- `interfaces` (Block List) Host interfaces.
- `tags` (Map of String) Host tags as `tag => value`. When not set, the host's tags are left unchanged (tags added in the Zabbix UI or by discovery are kept); `tags = {}` removes them all.
- `template_ids` (Set of String) Template IDs to link. Computed from `template_names` when not set.
- `template_names` (Set of String) Template names to link (technical or visible name), resolved to IDs at apply time. Computed from `template_ids` when not set.
- `visible_name` (String) Display name (`name`) in Zabbix. When not set, Zabbix shows the technical name.

## Important rules

- You must provide at least `host_group_ids` or `host_group_names`.
- ID and name lists are merged and deduplicated.
- The attribute you do not set is computed from the other one, so you can use
  either form; both are always known after apply.
- Host groups and templates created in the same configuration can be referenced:
  - by ID, e.g. `host_group_ids = [zabbix_host_group.x.id]` (the dependency is implicit);
  - by name, with `depends_on = [zabbix_host_group.x]` so that the group exists when
    the name is resolved (a plain string does not create a dependency).
- Templates: when neither `template_ids` nor `template_names` is set, the templates
  linked to the host are left unchanged. Set `template_ids = []` to unlink them all.
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
- `port` (String) Destination port: a number or a user macro such as `"{$AGENT.PORT}"`. Default: `"10050"`.
- `snmp_details` (Block) SNMP details.

### Nested Schema for `interfaces.snmp_details`

Optional:

- `version` (Number) SNMP version. Default: `2` (only accepted value currently).
- `community` (String) SNMP community. Default: `"{$SNMP_COMMUNITY}"`.

## Existing hosts

`zabbix_host` creates the host. For a host that already exists in Zabbix (created in the UI or
by discovery), `tofu apply` fails with `Host with the same name "..." already exists`. Either:

- import it, so that `zabbix_host` manages it from then on (the configuration must describe its
  interfaces; the next apply aligns Zabbix with it):

  ```terraform
  import {
    to = zabbix_host.srv
    id = "10819" # host ID, e.g. from data.zabbix_host or the Zabbix UI
  }
  ```

- or only add it to groups with `zabbix_host_group_membership`, leaving the rest of the host as it is.

## Import

```bash
tofu import zabbix_host.name 12345
```

## Notes

- Interfaces are sent with `host.create` / `host.update` (existing interfaces are updated in
  place). Zabbix 8.0 no longer allows managing interfaces with `hostinterface.*` methods.
- In practice, Zabbix usually requires at least one valid interface
  for a monitorable host.
