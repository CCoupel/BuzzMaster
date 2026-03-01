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

## Instructions

Utilise le Task tool pour lancer le sous-agent dev-backend avec les paramètres suivants :

```
subagent_type: "dev-backend"
description: "Implémenter backend Go"
prompt: voir ci-dessous
```

### Prompt à transmettre au sous-agent

```
Implémente le code backend Go pour BuzzControl.

**Contexte projet :** Voir `context/COMMON.md` section 1
**Workflow DEV :** Voir `context/DEVELOPMENT.md`
- Étapes : section 4 (Workflow Commun)
- Ordre backend : section 5
- Standards Go : section 5
- Build : section 4.4
- Règles : section 8

**Input utilisateur :** $ARGUMENTS

**Résumé à fournir (pour DEV-FRONTEND si applicable) :**
- Nouvelles actions WebSocket
- Nouveaux champs GameState
```

## Action immédiate

**Détecter le mode** — Lire `~/.claude/teams/myTEAM/config.json` :
- **Fichier trouvé → Mode TEAM** : transmettre au CDP pour dispatch
- **Fichier absent → Mode SOLO** : spawner un sous-agent jetable

### Mode TEAM
```
SendMessage(
  type: "message",
  recipient: "cdp",
  content: "Développement backend demandé: $ARGUMENTS",
  summary: "Dev backend: $ARGUMENTS"
)
```

### Mode SOLO
Lance le sous-agent `dev-backend` avec le Task tool.
