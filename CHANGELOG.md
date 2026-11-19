# Changelog

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
