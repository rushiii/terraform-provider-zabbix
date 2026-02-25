---
page_title: "zabbix_host Resource"
subcategory: ""
description: |-
  Gere un host Zabbix, ses interfaces, ses liens de groupes, templates et tags.
---

# zabbix_host (Resource)

Ressource pour creer et maintenir un host Zabbix.

Cette ressource supporte:

- association aux host groups par IDs ou par noms
- association aux templates par IDs ou par noms
- interfaces agent/SNMP/IPMI/JMX
- details SNMP v2 pour les interfaces `type = 2`
- tags du host

## Example Usage

### Host ICMP/Agent

```terraform
resource "zabbix_host" "name" {
  name         = "name"
  visible_name = "name"
  enabled      = true

  host_group_names = ["Linux servers"]
  template_names   = ["Template Module ICMP Ping"]

  interfaces {
    type   = 1
    main   = true
    use_ip = true
    ip     = "10.20.30.40"
    port   = "10050"
  }
}
```

### Host SNMP v2

```terraform
resource "zabbix_host" "switch_snmp_v2" {
  name         = "sw-core-01"
  visible_name = "Switch Core 01"
  enabled      = true

  host_group_names = ["Network devices"]
  template_names   = ["Template Module Interfaces SNMP"]

  interfaces {
    type   = 2
    main   = true
    use_ip = true
    ip     = "10.20.30.50"
    port   = "161"

    snmp_details {
      version   = 2
      community = "{$SNMP_COMMUNITY}"
    }
  }
}
```

## Schema

### Required

- `name` (String) Nom technique du host (`host` dans Zabbix).

### Optional

- `enabled` (Boolean) Host active/desactive. Defaut: `true`.
- `host_group_ids` (Set of String) IDs de groupes d'hotes.
- `host_group_names` (Set of String) Noms de groupes d'hotes (resolus en IDs).
- `id` (String) ID de la ressource.
- `interfaces` (Block List) Interfaces du host.
- `tags` (Map of String) Tags du host sous forme `tag => value`.
- `template_ids` (Set of String) IDs de templates a lier.
- `template_names` (Set of String) Noms de templates a lier (resolus en IDs).
- `visible_name` (String) Nom visible (`name`) du host dans Zabbix.

## Regles importantes

- Il faut fournir au moins `host_group_ids` ou `host_group_names`.
- Les listes IDs/noms sont fusionnees sans doublons.
- Pour `interfaces.use_ip = true`, le champ `ip` est obligatoire.
- Pour `interfaces.use_ip = false`, le champ `dns` est obligatoire.
- Si `interfaces.type = 2` (SNMP), seule la version `2` est actuellement supportee.
- Si `snmp_details` est omis sur une interface SNMP, les valeurs par defaut sont:
  - `version = 2`
  - `community = "{$SNMP_COMMUNITY}"`

### Nested Schema for `interfaces`

Required:

- `type` (Number) Type interface: `1=Agent`, `2=SNMP`, `3=IPMI`, `4=JMX`.

Optional:

- `main` (Boolean) Interface principale du type. Defaut: `true`.
- `use_ip` (Boolean) Utilise IP (`true`) ou DNS (`false`). Defaut: `true`.
- `ip` (String) Adresse IP (obligatoire si `use_ip = true`).
- `dns` (String) Nom DNS (obligatoire si `use_ip = false`).
- `port` (String) Port de destination. Defaut: `"10050"`.
- `snmp_details` (Block) Details SNMP.

### Nested Schema for `interfaces.snmp_details`

Optional:

- `version` (Number) Version SNMP. Defaut: `2` (seule valeur acceptee actuellement).
- `community` (String) Community SNMP. Defaut: `"{$SNMP_COMMUNITY}"`.

## Import

```bash
tofu import zabbix_host.name 12345
```

## Notes

- Lors d'un update, les interfaces sont remplacees en bloc via
  `hostinterface.replacehostinterfaces`.
- En pratique, Zabbix exige generalement au moins une interface valide
  pour un host monitorable.
