---
page_title: "zabbix Provider"
subcategory: ""
description: |-
  Provider OpenTofu/Terraform pour gerer des objets Zabbix via l'API JSON-RPC.
---

# zabbix Provider

Le provider `zabbix` permet de gerer des ressources Zabbix (hosts, host groups, templates, triggers)
depuis Terraform/OpenTofu.

## Ressources supportees

- `zabbix_host`
- `zabbix_host_group`
- `zabbix_template`
- `zabbix_trigger`

## Data sources

Ce provider ne publie actuellement aucun data source.

## Example Usage

```terraform
terraform {
  required_providers {
    zabbix = {
      source  = "local/zabbix"
      version = "0.1.0"
    }
  }
}

provider "zabbix" {
  url       = "https://zabbix.example.com/api_jsonrpc.php"
  api_token = var.zabbix_api_token
}
```

Exemple avec authentification user/password:

```terraform
provider "zabbix" {
  url      = "https://zabbix.example.com/api_jsonrpc.php"
  username = var.zabbix_username
  password = var.zabbix_password
}
```

## Schema

### Required

- `url` (String) URL de l'API Zabbix, par exemple `https://zabbix.example.com/api_jsonrpc.php`.

### Optional

- `api_token` (String, Sensitive) Token API Zabbix. Prioritaire si defini.
- `username` (String) Nom utilisateur Zabbix (utilise si `api_token` est absent).
- `password` (String, Sensitive) Mot de passe Zabbix (utilise si `api_token` est absent).
- `timeout_seconds` (Number) Timeout HTTP en secondes. Defaut: `30`.
- `insecure_skip_tls` (Boolean) Ignore la validation TLS (utile en labo uniquement).

## Comportement d'authentification

- Si `api_token` est renseigne, il est utilise en priorite.
- Si `api_token` est vide, `username` et `password` sont obligatoires.
- Si `api_token` et `username/password` sont donnes en meme temps, le token reste prioritaire.

## Validation au configure

Au chargement du provider:

1. Le client HTTP/API est initialise.
2. Un ping `apiinfo.version` est execute pour verifier l'accessibilite de l'API.
3. En cas d'erreur, la planification/apply echoue immediatement.
