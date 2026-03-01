# Commande /qa - Lancer les Tests QA

Lance le sous-agent QA pour exécuter les tests et générer un rapport de qualité.

## Argument reçu (optionnel)

$ARGUMENTS

## Mot-clé help

`/qa help` → Affiche :

```
## /qa - Aide

**Description** : Exécuter les tests et générer un rapport de qualité

**Usage** :
  /qa help           Afficher cette aide
  /qa                Suite complète (défaut)
  /qa unit           Tests unitaires uniquement
  /qa e2e            Tests E2E uniquement
  /qa full           Suite complète explicite
  /qa <package>      Tests d'un package spécifique

**Différence** : /qa EXÉCUTE, /test-write ÉCRIT
**Verdicts** : VALIDATED / VALIDATED WITH RESERVATIONS / NOT VALIDATED
```

## Action immédiate

**Détecter le mode** — Lire `~/.claude/teams/TEAM-Buzz/config.json` :
- **Fichier trouvé → Mode TEAM** : transmettre au CDP existant
- **Fichier absent** : spawner le CDP — il orchestrera les agents nécessaires

### Mode TEAM
```
SendMessage(
  type: "message",
  recipient: "cdp",
  content: "Tests QA demandés: $ARGUMENTS",
  summary: "QA: $ARGUMENTS"
)
```

### Sans TEAM
```
subagent_type: "cdp"
description: "Tests QA"
prompt: Tests QA demandés: $ARGUMENTS
```
