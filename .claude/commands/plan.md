# Commande /plan - Planification d'Implémentation

Lance le sous-agent implementation-planner pour créer un plan d'implémentation détaillé AVANT tout développement.

## Argument reçu

$ARGUMENTS

## Mot-clé help

`/plan help` → Affiche l'aide ci-dessous :

```
## /plan - Aide

**Description** : Créer un plan d'implémentation détaillé AVANT développement

**Usage** :
  /plan help                           Afficher cette aide
  /plan backlog/<nom>.md               Depuis un backlog
  /plan backlog/<nom>.md Phase X       Phase spécifique d'un backlog
  /plan "Description de la feature"    Description libre
  /plan bugfix "Description du bug"    Planification de bugfix

**Output** : Plan structuré avec tâches Backend → Frontend → Tests → Docs
```

**Formats possibles** :
- `/plan backlog/memory-game.md Phase 6` : Depuis un backlog
- `/plan "Description de la feature"` : Description libre
- `/plan bugfix "Description du bug"` : Planification de bugfix

## Action immédiate

**Détecter le mode** — Lire `~/.claude/teams/myTEAM/config.json` :
- **Fichier trouvé → Mode TEAM** : transmettre au CDP existant
- **Fichier absent** : spawner le CDP — il orchestrera les agents nécessaires

### Mode TEAM
```
SendMessage(
  type: "message",
  recipient: "cdp",
  content: "Planification demandée: $ARGUMENTS",
  summary: "Plan: $ARGUMENTS"
)
```

### Sans TEAM
```
subagent_type: "cdp"
description: "Planification"
prompt: Planification demandée: $ARGUMENTS
```
