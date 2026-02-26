---
page_title: "zabbix_template Resource"
subcategory: ""
description: |-
  Manages a Zabbix template and its host group links.
---

# zabbix_template (Resource)

Manages a Zabbix template.

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

- `host` (String) Internal template name (Zabbix `host` field).
- `host_group_ids` (Set of String) IDs of host groups to attach to the template.

### Optional

- `id` (String) Resource ID.
- `name` (String) Visible template name.

## Behavior

- If `name` is omitted or empty, it defaults to the same value as `host`.
- `host_group_ids` must be existing IDs in Zabbix.

## Import

```bash
tofu import zabbix_template.custom_icmp 10001
```
