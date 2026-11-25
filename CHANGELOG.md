# Changelog

## 1.4.0

### New

- Data source `zabbix_host`: looks up an existing host by technical or visible name (e.g. a
  host created in the Zabbix UI or by discovery).
- Resource `zabbix_host_group_membership`: adds existing hosts to a host group
  (`hostgroup.massadd` / `massremove`) without managing anything else about them.

### Documentation

- `zabbix_host`: how to handle hosts that already exist in Zabbix (import, or group membership)
  instead of failing with `Host with the same name ... already exists`.

## 1.3.3

### Zabbix 8.0

- `zabbix_host`: creating a host with only SNMP interfaces failed on Zabbix 8.0 with
  `hostinterface.delete (temp): Method not found`. Zabbix 8.0 removed the
  `hostinterface.create` / `update` / `delete` methods, which the provider used to work around
  an old `host.create` limitation. The host is now created with its SNMP interfaces and
  templates in a single `host.create`, which works on Zabbix 6.0 to 8.0.

### Tests

- Acceptance test for a host with only an SNMP interface linked to a template with SNMP items.
  The whole acceptance suite (15 tests) passes on Zabbix 7.0, 7.4 and 8.0.

## 1.3.2

### Bug fixes

- Requests failing with a transient "Database error occurred" are retried. Zabbix reports it
  when concurrent requests collide while allocating object IDs (e.g. a template and a host
  created in parallel by one `apply` on a new Zabbix); the failed transaction is rolled back,
  so retrying is safe.

### Documentation

- "One configuration, many files": how to write a configuration that one `apply` creates
  entirely, and a complete multi-file example in `examples/complete/`.

## 1.3.1

### Bug fixes

- `zabbix_host`: no perpetual diff on `visible_name` when it is not set (Zabbix reports the
  technical name as the visible name).
- `zabbix_host`: tags are only managed when `tags` is set. Tags added outside Terraform (UI,
  discovery) are no longer removed by an apply on a host without `tags`; `tags = {}` removes
  all tags.
- `zabbix_host`: an interface `port` can be a user macro (`"{$AGENT.PORT}"`); it used to be
  replaced by 10050. An invalid port is now an error instead of being silently replaced.
- `zabbix_host`: several interfaces with the same type and main flag (e.g. two secondary agent
  interfaces) are updated correctly; one of them used to be dropped.
- `zabbix_host`: if a host with only SNMP interfaces cannot be fully set up, it is deleted
  instead of being left in Zabbix outside of the state.
- `zabbix_action`: `subject` and `message` are now sent to Zabbix and used in notifications
  (they were ignored, and the plan always showed a diff). Existing actions show one change
  on the next plan.
- `zabbix_item`: `delay_flex` is applied (it was ignored). The Zabbix format
  (`50s/1-5,09:00-18:00;md1-5`) and the older `50s;1-7,00:00-24:00` form are accepted.
- `zabbix_template`: `macros = {}` removes all macros (the last macro used to stay).
- Username/password authentication: an expired session is renewed automatically instead of
  failing every remaining call.

### Documentation

- New `zabbix_action` page, including how conditions are combined (OR).

## 1.3.0

### New resources

- `zabbix_host_trigger_status`: enables or disables an existing trigger of a host, such as a
  trigger inherited from a linked template (e.g. `ICMP Ping`), without creating or deleting
  it. On destroy, an inherited trigger gets back the status of its template trigger.

## 1.2.1

### Bug fixes

- `zabbix_host`: a host group created in the same configuration can be referenced by ID
  (`host_group_ids = [zabbix_host_group.x.id]`). The plan no longer fails with
  "Provide at least `host_group_ids` or `host_group_names`" while the ID is not known yet.
- `zabbix_host`: `host_group_names`, `host_group_ids`, `template_names` and `template_ids`
  are always known after apply. Creating a host configured with IDs only, or without
  templates, no longer fails with "Provider returned invalid result object after apply"
  and no longer leaves the host tainted.
- `zabbix_host`: host group and template names are resolved to IDs at apply time instead
  of plan time, so a group or template created in the same apply can be referenced by name
  (with `depends_on`).
- `zabbix_host`: no perpetual diff on `template_names` when a template is referenced by its
  technical name, nor on the computed `*_names` / `*_ids` attribute.
- `zabbix_host`: `template_ids = []` (or `template_names = []`) now unlinks all templates.
  When neither attribute is set, linked templates are left unchanged.

### Tests

- Acceptance tests for `zabbix_host` (`TF_ACC=1`, see `internal/provider/resource_host_acc_test.go`).

## 1.2.0

- `zabbix_host`: new `description` attribute.
- CI workflow: build, vet and tests on pull requests.

## 1.1.0

- Support for Zabbix 7.2, 7.4 and 8.0 (`Authorization: Bearer` header, `selectHostGroups` /
  `selectTemplateGroups`, `hostgroup_rights`), detected from the server version.
