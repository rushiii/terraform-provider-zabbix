---
page_title: "zabbix Provider"
subcategory: ""
description: |-
  Terraform/OpenTofu provider for managing Zabbix objects via the JSON-RPC API.
---

# zabbix Provider

The `zabbix` provider lets you manage Zabbix resources (hosts, host groups, templates, triggers)
from Terraform/OpenTofu.

## Supported resources

- `zabbix_host`
- `zabbix_host_group`
- `zabbix_template`
- `zabbix_trigger`
- `zabbix_item`
- `zabbix_action`
- `zabbix_host_trigger_status` (enable/disable a trigger inherited from a template on a host)

## Data sources

This provider currently does not expose any data sources.

## Example Usage

```terraform
terraform {
  required_providers {
    zabbix = {
      source  = "rushiii/zabbix"
      version = "1.3.3"
    }
  }
}

provider "zabbix" {
  url       = "https://zabbix.example.com/api_jsonrpc.php"
  api_token = var.zabbix_api_token
}
```

Example with username/password authentication:

```terraform
provider "zabbix" {
  url      = "https://zabbix.example.com/api_jsonrpc.php"
  username = var.zabbix_username
  password = var.zabbix_password
}
```

## One configuration, many files

OpenTofu/Terraform loads every `.tf` file of a directory as a single configuration, so one
`tofu apply` can create host groups, templates, hosts, items, triggers, users and actions
together. A complete example split over several files is in `examples/complete/`.

OpenTofu creates resources in parallel, in the order given by the references between them.
To make one apply work every time:

- **Reference objects instead of writing their IDs or names**:
  `host_group_ids = [zabbix_host_group.x.id]`, `template_ids = [zabbix_template.t.id]`,
  `host_id = zabbix_host.h.id`, `user_group_ids = [zabbix_user_group.g.id]`.
- **Trigger expressions** are plain strings: build them from references so the trigger is
  created after its host and item, e.g.
  `expression = "last(/${zabbix_host.h.name}/${zabbix_item.i.key})>100"`.
- **Names** (`host_group_names`, `template_names`) of objects created in the same
  configuration need `depends_on = [zabbix_host_group.x]`: a string does not create a
  dependency. Names of objects that already exist in Zabbix need nothing.
- **Templates** must be in template groups (`host_group_ids` of `zabbix_template` are
  template group IDs on Zabbix 6.2+, e.g. `"1"` for "Templates").

The provider retries a request when Zabbix reports a transient database error caused by
concurrent requests, so the default parallelism of OpenTofu can be kept.

## Schema

### Required

- `url` (String) Zabbix API URL, for example `https://zabbix.example.com/api_jsonrpc.php`.

### Optional

- `api_token` (String, Sensitive) Zabbix API token. Takes priority if provided.
- `username` (String) Zabbix username (used when `api_token` is not set).
- `password` (String, Sensitive) Zabbix password (used when `api_token` is not set).
- `timeout_seconds` (Number) HTTP timeout in seconds. Default: `30`.
- `insecure_skip_tls` (Boolean) Skip TLS certificate validation (for lab/testing only).

## Authentication behavior

- If `api_token` is set, it is used first.
- If `api_token` is empty, `username` and `password` are required.
- If both `api_token` and `username/password` are set, the token still takes priority.

## Zabbix compatibility

Supported Zabbix versions: 6.0 LTS, 6.4, 7.0 LTS, 7.2, 7.4 and 8.0. The provider detects the server version and
uses the matching API conventions (the `Authorization: Bearer` header from 6.4, `selectHostGroups` /
`selectTemplateGroups` / `hostgroup_rights` from 6.2). Resource behavior is identical on every version.
Zabbix 8.0 requires provider 1.3.3 or later for hosts with only SNMP interfaces.

## Configure-time validation

During provider initialization:

1. The HTTP/API client is initialized.
2. An `apiinfo.version` ping is executed to verify API reachability and detect the Zabbix version.
3. If the check fails, planning/apply fails immediately.
