# Commande /dev-frontend - Implémentation Frontend React

Lance le sous-agent dev-frontend pour implémenter du code React selon un plan.

## Argument reçu

$ARGUMENTS

## Mot-clé help

`/dev-frontend help` → Affiche :

```
## /dev-frontend - Aide

**Description** : Implémenter du code frontend React

**Usage** :
  /dev-frontend help                 Afficher cette aide
  /dev-frontend [plan]               Plan d'implémentation frontend
  /dev-frontend fix "description"    Correction de bug frontend
  /dev-frontend review "corrections" Corrections post-review

**Ordre** : hooks → components → pages → PlayerDisplay → CSS
**Contrainte** : TV STATIQUE (overflow: hidden, vh/vw)
```

**Formats possibles** :
- `/dev-frontend [plan]` : Plan d'implémentation frontend
- `/dev-frontend fix "description"` : Correction de bug frontend
- `/dev-frontend review "corrections"` : Corrections post-review

## Action immédiate

**Détecter le mode** — Lire `~/.claude/teams/myTEAM/config.json` :
- **Fichier trouvé → Mode TEAM** : transmettre au CDP existant
- **Fichier absent** : spawner le CDP — il orchestrera les agents nécessaires

### Mode TEAM
```
SendMessage(
  type: "message",
  recipient: "cdp",
  content: "Développement frontend demandé: $ARGUMENTS",
  summary: "Dev frontend: $ARGUMENTS"
)
```

### Sans TEAM
```
subagent_type: "cdp"
description: "Dev frontend"
prompt: Développement frontend demandé: $ARGUMENTS
```
