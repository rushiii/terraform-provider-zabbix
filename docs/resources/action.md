---
page_title: "zabbix_action Resource"
subcategory: ""
description: |-
  Manages a Zabbix trigger action that sends a message to user groups and users.
---

# zabbix_action (Resource)

Manages a trigger action (event source "triggers") that sends a message to user groups
and, optionally, to specific users when a problem occurs.

## Example Usage

```terraform
data "zabbix_user_group" "admins" {
  name = "Zabbix administrators"
}

resource "zabbix_action" "lamp" {
  name              = "Videoprojecteurs : lampe"
  user_group_ids    = [data.zabbix_user_group.admins.id]
  host_group_ids    = [zabbix_host_group.lampe.id]
  trigger_name_like = ["Lampe"]

  subject = "Problème : {TRIGGER.NAME} sur {HOST.NAME}"
  message = <<-EOT
    Trigger: {TRIGGER.NAME}
    Host: {HOST.NAME}
    Severity: {TRIGGER.SEVERITY}
  EOT
}
```

## Schema

### Required

- `message` (String) Message body. Supports Zabbix macros.
- `name` (String) Action name.
- `subject` (String) Message subject. Supports Zabbix macros.
- `user_group_ids` (Set of String) IDs of the user groups to notify.

### Optional

- `enabled` (Boolean) Whether the action is enabled. Default: `true`.
- `esc_period` (String) Default operation step duration (e.g. `"1h"`, `"60s"`). Default: `"1h"`.
- `host_group_ids` (Set of String) Host group conditions.
- `trigger_name_like` (Set of String) Trigger name conditions ("contains").
- `user_ids` (Set of String) IDs of users to notify in addition to the user groups.

### Read-Only

- `id` (String) Action ID.

## Conditions

Each host group and each trigger name pattern is one condition. When there is more than
one condition, they are combined with **OR**: the action fires for any problem on a host of
one of the groups, **or** for any trigger whose name contains one of the patterns. Without
any condition, the action fires for every trigger problem.

## Message

The message is sent with the configured `subject` and `message`, not the defaults of the
media type. Actions created before version 1.3.1 used the media type defaults: the next
`apply` shows a change on `subject`/`message` and sets the configured text.

## Import

```bash
tofu import zabbix_action.lamp 7
```
