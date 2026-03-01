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

## Instructions

Utilise le Task tool pour lancer le sous-agent QA avec les paramètres suivants :

```
subagent_type: "QA"
description: "Exécuter tests QA"
prompt: voir ci-dessous
```

### Prompt à transmettre au sous-agent

```
Exécute la procédure de tests QA complète pour BuzzControl.

**Contexte projet :** Voir `context/COMMON.md` section 1
**Build :** Voir `context/COMMON.md` section 2
**Framework Qualité :** Voir `context/QUALITY.md`
- Commandes test : section 7
- Critères validation : section 6
- Verdicts : section 5
- Structure rapport : section 9
- Règles : section 10

**Argument utilisateur (optionnel) :** $ARGUMENTS
- `unit` : Tests unitaires uniquement
- `e2e` : Tests E2E uniquement
- `full` ou vide : Suite complète (défaut)
- Nom de package : Tests d'un package spécifique
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
  content: "Tests QA demandés: $ARGUMENTS",
  summary: "QA: $ARGUMENTS"
)
```

### Mode SOLO
Lance le sous-agent `QA` avec le Task tool.
