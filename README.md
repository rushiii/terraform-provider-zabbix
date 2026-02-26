# Terraform/OpenTofu Zabbix Provider Project

This repository is a **Zabbix provider project** for Terraform/OpenTofu, implemented with the Terraform Plugin Framework.
It lets you manage Zabbix objects through the Zabbix JSON-RPC API.

## What this project provides

Supported resources:

- `zabbix_host`
- `zabbix_host_group`
- `zabbix_template`
- `zabbix_trigger`

Key capabilities:

- Manage hosts, host groups, templates, and triggers from Terraform/OpenTofu
- Use either IDs or names for host group/template links on `zabbix_host`
- Support SNMP v2 host interfaces on `zabbix_host.interfaces`

## Authentication

The provider supports two authentication methods:

- API token: `api_token`
- Username/password: `username` + `password`

Behavior:

- If `api_token` is set, it has priority
- If `api_token` is empty, `username` and `password` are required

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
      version = "0.1.1"
    }
  }
}
```

## Release assets required by OpenTofu/Terraform registries

Each GitHub release tag (for example `v0.1.1`) must include:

- `terraform-provider-zabbix_0.1.1_linux_amd64.zip`
- `terraform-provider-zabbix_0.1.1_linux_arm64.zip`
- `terraform-provider-zabbix_0.1.1_darwin_amd64.zip`
- `terraform-provider-zabbix_0.1.1_darwin_arm64.zip`
- `terraform-provider-zabbix_0.1.1_windows_amd64.zip`
- `terraform-provider-zabbix_0.1.1_SHA256SUMS`

This repository now includes a GoReleaser config and a GitHub Actions workflow to generate
and upload these assets automatically when a version tag is pushed.

## Import examples

```bash
tofu import zabbix_host.ubuntu01 12345
tofu import zabbix_host_group.linux 12
tofu import zabbix_template.custom_icmp 10001
tofu import zabbix_trigger.icmp_loss 40055
```
