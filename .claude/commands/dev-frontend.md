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

## Instructions

Utilise le Task tool pour lancer le sous-agent dev-frontend avec les paramètres suivants :

```
subagent_type: "dev-frontend"
description: "Implémenter frontend React"
prompt: voir ci-dessous
```

### Prompt à transmettre au sous-agent

```
Implémente le code frontend React pour BuzzControl.

**Contexte projet :** Voir `context/COMMON.md` section 1
**Workflow DEV :** Voir `context/DEVELOPMENT.md`
- Étapes : section 4 (Workflow Commun)
- Ordre frontend : section 6
- Standards React : section 6
- Contrainte TV : section 6 (TV STATIQUE)
- Build : section 4.4
- Règles : section 8

**Input utilisateur :** $ARGUMENTS

**Si backend terminé, intégrer :**
- Nouvelles actions WebSocket
- Nouveaux champs GameState
```

## Action immédiate

**Détecter le mode** — Lire `~/.claude/teams/myTEAM/config.json` :
- **Fichier trouvé → Mode TEAM** : envoyer le prompt au teammate `frontend-dev`
- **Fichier absent → Mode SOLO** : spawner un sous-agent jetable

### Mode TEAM
```
SendMessage(
  type: "message",
  recipient: "frontend-dev",
  content: [prompt "Prompt à transmettre au sous-agent" ci-dessus],
  summary: "Dev frontend: $ARGUMENTS"
)
```

### Mode SOLO
Lance le sous-agent `dev-frontend` avec le Task tool.
