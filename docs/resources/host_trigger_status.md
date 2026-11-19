---
page_title: "zabbix_host_trigger_status Resource"
subcategory: ""
description: |-
  Enables or disables an existing trigger of a host, such as a trigger inherited from a linked template.
---

# zabbix_host_trigger_status (Resource)

Enables or disables a trigger that already exists on a host, typically a trigger
inherited from a template linked to the host (for example the triggers of the
`ICMP Ping` template).

The trigger is neither created nor deleted by this resource: to create your own
triggers, use `zabbix_trigger`.

## Example Usage

```terraform
resource "zabbix_host" "tv" {
  name             = "fst-tv-atela.example.fr"
  host_group_names = ["Linux servers"]
  template_names   = ["ICMP Ping"]

  interfaces {
    type = 1
    ip   = "172.17.32.22"
  }
}

# Disable one of the triggers inherited from "ICMP Ping" on this host only.
resource "zabbix_host_trigger_status" "tv_icmp_response_time" {
  host_id = zabbix_host.tv.id
  name    = "ICMP Ping: High ICMP ping response time"
  enabled = false
}
```

## Schema

### Required

- `enabled` (Boolean) Whether the trigger is enabled on this host.
- `host_id` (String) ID of the host the trigger belongs to, e.g. `zabbix_host.tv.id`. Changing it replaces the resource.
- `name` (String) Trigger name, as defined in the template (e.g. `ICMP Ping: Unavailable by ICMP ping`) or as displayed on the host with macros expanded. Changing it replaces the resource.

### Read-Only

- `id` (String) Trigger ID on the host.

## Notes

- Use `host_id = zabbix_host.x.id` rather than a hard-coded ID: the dependency makes sure the
  host and its templates exist before the trigger is looked up.
- On destroy, a trigger inherited from a template gets back the status of its template
  trigger (usually enabled). A trigger that is not inherited is left as it is.
- If the trigger disappears (host deleted, template unlinked), the resource is removed
  from the state.

## Import

The import ID is the trigger ID on the host:

```bash
tofu import zabbix_host_trigger_status.tv_icmp_response_time 32564
```
