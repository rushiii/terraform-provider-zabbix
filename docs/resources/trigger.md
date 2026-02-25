---
page_title: "zabbix_trigger Resource"
subcategory: ""
description: |-
  Gere un trigger Zabbix (description, expression, priorite, activation).
---

# zabbix_trigger (Resource)

Permet de gerer un trigger Zabbix.

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

- `description` (String) Description du trigger.
- `expression` (String) Expression du trigger au format Zabbix.

### Optional

- `enabled` (Boolean) Active/desactive le trigger. Defaut: `true`.
- `id` (String) ID de la ressource.
- `priority` (String) Severite de `0` a `5`. Defaut: `"3"`.

## Notes

- `priority` est exposee en string (valeurs attendues: `0..5`).
- Le provider n'impose pas de validation stricte sur la plage de `priority`;
  la validation finale est donc faite par l'API Zabbix.

## Import

```bash
tofu import zabbix_trigger.icmp_loss 40055
```
