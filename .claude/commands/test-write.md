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

## Instructions

Lance le sous-agent **test-writer** via Task tool :

```
subagent_type: "test-writer"
description: "Écrire les tests"
prompt: voir ci-dessous
```

### Prompt à transmettre au sous-agent

```
Écris les tests pour BuzzControl.

**Contexte projet :** Voir `context/COMMON.md` section 1
**Framework Qualité :** Voir `context/QUALITY.md`
- Matrice qualité : section 2 (test-writer ÉCRIT, QA EXÉCUTE)
- Commandes test : section 7
- Règles : section 10

**Input utilisateur :** $ARGUMENTS

**Rappel :** Tu ÉCRIS les tests, tu ne les EXÉCUTES PAS.
```

## Intégration workflow

Voir `context/QUALITY.md` section 11 pour le workflow.

## Action immédiate

**Détecter le mode** — Lire `~/.claude/teams/myTEAM/config.json` :
- **Fichier trouvé → Mode TEAM** : transmettre au CDP pour dispatch
- **Fichier absent → Mode SOLO** : spawner un sous-agent jetable

### Mode TEAM
```
SendMessage(
  type: "message",
  recipient: "cdp",
  content: "Écriture de tests demandée: $ARGUMENTS",
  summary: "Tests: $ARGUMENTS"
)
```

### Mode SOLO
Lance le sous-agent `test-writer` avec le Task tool.
