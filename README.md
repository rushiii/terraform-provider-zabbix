# Terraform/OpenTofu Zabbix Provider Project

This repository is a **Zabbix provider project** for Terraform/OpenTofu, implemented with the Terraform Plugin Framework.
It lets you manage Zabbix objects through the Zabbix JSON-RPC API.

## What this project provides

Supported resources:

- `zabbix_host`
- `zabbix_host_group`
- `zabbix_template`
- `zabbix_trigger`
- `zabbix_item`
- `zabbix_action`
- `zabbix_host_trigger_status` (enable/disable a trigger inherited from a template on a host)

Key capabilities:

- Manage hosts, host groups, templates, and triggers from Terraform/OpenTofu
- Use either IDs or names for host group/template links on `zabbix_host`
- Support SNMP v2 host interfaces on `zabbix_host.interfaces`

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

## Authentication

The provider supports two authentication methods:

- API token: `api_token`
- Username/password: `username` + `password`

Behavior:

- If `api_token` is set, it has priority
- If `api_token` is empty, `username` and `password` are required

## Zabbix compatibility

The provider detects the Zabbix API version at startup (`apiinfo.version`) and adapts its requests:

| Zabbix version | Authentication | Host/template groups, user group permissions |
|---|---|---|
| < 6.2 | `auth` field in the JSON-RPC body | `selectGroups`, `rights` |
| 6.2 | `auth` field in the JSON-RPC body | `selectHostGroups` / `selectTemplateGroups`, `hostgroup_rights` |
| 6.4, 7.0 LTS, 7.2, 7.4, 8.0 | `Authorization: Bearer` header | `selectHostGroups` / `selectTemplateGroups`, `hostgroup_rights` |

Zabbix 7.2 removed the `auth` request field and the `selectGroups` / `rights` parameters, which is why
older provider versions fail on recent Zabbix servers with `unexpected parameter "auth"`.
Zabbix 8.0 removed the `hostinterface.create` / `update` / `delete` methods: provider versions before
1.3.3 fail on 8.0 when creating a host with only SNMP interfaces (`Method not found`).
Resource behavior is the same on every supported version.

## Tests

```bash
go test ./...
```

Acceptance tests run against a real Zabbix server and are skipped unless `TF_ACC` is set:

```bash
TF_ACC=1 ZABBIX_URL=http://localhost:8080/api_jsonrpc.php ZABBIX_USERNAME=Admin ZABBIX_PASSWORD=zabbix \
  go test ./internal/provider/ -run TestAcc -v
```

With OpenTofu, also set `TF_ACC_TERRAFORM_PATH=$(which tofu)` and `TF_ACC_PROVIDER_HOST=registry.opentofu.org`.

## Build

```bash
go mod tidy
go build ./...
```

## Local development test

Build the provider:

```bash
go build -o dist/terraform-provider-zabbix .
```

Create a local Terraform CLI config (for dev override), then point Terraform/OpenTofu to the local binary:

```hcl
provider_installation {
  dev_overrides {
    "registry.terraform.io/rushiii/zabbix" = "/absolute/path/to/terraform-provider-zabbix/dist"
  }
  direct {}
}
```

Then run:

```bash
TF_CLI_CONFIG_FILE=/absolute/path/to/.terraformrc.local terraform init -reconfigure
terraform plan
```

## Registry source address

Use this source address in your Terraform/OpenTofu configuration:

```hcl
terraform {
  required_providers {
    zabbix = {
      source  = "rushiii/zabbix"
      version = "1.3.3"
    }
  }
}
```