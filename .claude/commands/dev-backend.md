# Commande /dev-backend - Implémentation Backend Go

Lance le sous-agent dev-backend pour implémenter du code Go selon un plan.

## Argument reçu

$ARGUMENTS

## Mot-clé help

`/dev-backend help` → Affiche :

```
## /dev-backend - Aide

**Description** : Implémenter du code backend Go

**Usage** :
  /dev-backend help                 Afficher cette aide
  /dev-backend [plan]               Plan d'implémentation backend
  /dev-backend fix "description"    Correction de bug backend
  /dev-backend review "corrections" Corrections post-review

**Ordre** : models → engine → tests → protocol → handlers
```

**Formats possibles** :
- `/dev-backend [plan]` : Plan d'implémentation backend
- `/dev-backend fix "description"` : Correction de bug backend
- `/dev-backend review "corrections"` : Corrections post-review

## Action immédiate

**Détecter le mode** — Lire `~/.claude/teams/myTEAM/config.json` :
- **Fichier trouvé → Mode TEAM** : transmettre au CDP existant
- **Fichier absent** : spawner le CDP — il orchestrera les agents nécessaires

### Mode TEAM
```
SendMessage(
  type: "message",
  recipient: "cdp",
  content: "Développement backend demandé: $ARGUMENTS",
  summary: "Dev backend: $ARGUMENTS"
)
```

### Sans TEAM
```
subagent_type: "cdp"
description: "Dev backend"
prompt: Développement backend demandé: $ARGUMENTS
```
