---
page_title: "zabbix_item Resource"
subcategory: ""
description: |-
  Manages a Zabbix item (élément de collecte). Supports SNMP items (type 2).
---

# zabbix_item (Resource)

Creates, reads, updates, and deletes a Zabbix item. Typically used on a template (host_id = template id).

## Schema

### Required

- `host_id` (String) ID of the host or template to attach the item to.
- `name` (String) Item name.
- `key` (String) Item key (e.g. epson.lamp.hours).
- `snmp_oid` (String) SNMP OID.

### Optional

- `type` (Number) Item type: 0=Zabbix agent, 1=SNMPv1, 2=SNMPv2c, 3=SNMPv3. Default: 2.
- `value_type` (Number) Value type: 0=float, 1=string, 2=log, 3=unsigned, 4=text. Default: 3.
- `units` (String) Display units (e.g. !h for hours).
- `delay` (String) Update interval (e.g. 10m, 60s). Default: 10m.
- `history` (String) History storage period (e.g. 90d). Default: 90d.
- `trends` (String) Trends storage period (e.g. 365d). Default: 365d.
- `delay_flex` (String) Flexible interval, e.g. 50s;1-7,00:00-24:00.
- `enabled` (Bool) Whether the item is enabled. Default: true.

### Read-only

- `id` (String) Item ID.
