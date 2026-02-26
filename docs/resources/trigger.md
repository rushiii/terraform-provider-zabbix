---
page_title: "zabbix_trigger Resource"
subcategory: ""
description: |-
  Manages a Zabbix trigger (description, expression, priority, status).
---

# zabbix_trigger (Resource)

Manages a Zabbix trigger.

## Example Usage

```terraform
resource "zabbix_trigger" "icmp_loss" {
  description = "ICMP ping loss on ubuntu01"
  expression  = "{ubuntu01:icmpping.max(5m)}=0"
  priority    = "4"
  enabled     = true
}
```

## Schema

### Required

- `description` (String) Trigger description.
- `expression` (String) Trigger expression in Zabbix format.

### Optional

- `enabled` (Boolean) Enable/disable trigger. Default: `true`.
- `id` (String) Resource ID.
- `priority` (String) Severity from `0` to `5`. Default: `"3"`.

## Notes

- `priority` is exposed as a string (expected values: `0..5`).
- The provider does not enforce strict range validation for `priority`;
  final validation is performed by the Zabbix API.

## Import

```bash
tofu import zabbix_trigger.icmp_loss 40055
```
