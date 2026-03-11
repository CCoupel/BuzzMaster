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
  /plan #N                             Depuis une issue GitHub
  /plan #N Phase X                     Phase spécifique d'une issue
  /plan backlog/<nom>.md               Depuis un fichier de spec
  /plan backlog/<nom>.md Phase X       Phase spécifique d'un fichier de spec
  /plan "Description de la feature"    Description libre
  /plan bugfix "Description du bug"    Planification de bugfix

**Output** : Plan structuré avec tâches Backend → Frontend → Tests → Docs
```

**Formats possibles** :
- `/plan #5` : Depuis une issue GitHub
- `/plan #5 Phase 7` : Phase spécifique d'une issue GitHub
- `/plan backlog/memory-game.md Phase 6` : Depuis un fichier de spec
- `/plan "Description de la feature"` : Description libre
- `/plan bugfix "Description du bug"` : Planification de bugfix

## Action immédiate

**Détecter le mode** — Lire `~/.claude/teams/TEAM-Buzz/config.json` :
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
