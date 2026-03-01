# Commande /test-write - Définition et Rédaction des Tests

Lance l'agent **test-writer** pour écrire les tests (unitaires, E2E) sans les exécuter.

## Argument reçu

$ARGUMENTS

## Mot-clé help

`/test-write help` → Affiche :

```
## /test-write - Aide

**Description** : Écrire les tests (unitaires, E2E) SANS les exécuter

**Usage** :
  /test-write help       Afficher cette aide
  /test-write            Écrire tests pour fichiers modifiés
  /test-write <fichier>  Écrire tests pour un fichier spécifique
  /test-write unit       Tests unitaires uniquement
  /test-write e2e        Scénarios E2E Chrome uniquement

**Différence** : /test-write ÉCRIT, /qa EXÉCUTE
```

**Formats possibles** :
- `/test-write` : Analyse les fichiers modifiés et écrit les tests manquants
- `/test-write <fichier>` : Écrit les tests pour un fichier spécifique
- `/test-write e2e` : Définit uniquement les scénarios E2E Chrome
- `/test-write unit` : Écrit uniquement les tests unitaires

## Différence avec /qa

| Commande | Action | Agent |
|----------|--------|-------|
| `/test-write` | **Écrit** les fichiers de tests | `test-writer` |
| `/qa` | **Exécute** les tests existants | `QA` |

## Action immédiate

**Détecter le mode** — Lire `~/.claude/teams/TEAM-Buzz/config.json` :
- **Fichier trouvé → Mode TEAM** : transmettre au CDP existant
- **Fichier absent** : spawner le CDP — il orchestrera les agents nécessaires

### Mode TEAM
```
SendMessage(
  type: "message",
  recipient: "cdp",
  content: "Écriture de tests demandée: $ARGUMENTS",
  summary: "Tests: $ARGUMENTS"
)
```

### Sans TEAM
```
subagent_type: "cdp"
description: "Écriture tests"
prompt: Écriture de tests demandée: $ARGUMENTS
```
