# Commande /code-review - Revue de Code

Lance le sous-agent code-reviewer pour analyser le code récemment modifié.

## Argument reçu (optionnel)

$ARGUMENTS

## Mot-clé help

`/code-review help` → Affiche :

```
## /code-review - Aide

**Description** : Revue de code du code modifié

**Usage** :
  /code-review help             Afficher cette aide
  /code-review                  Analyser fichiers modifiés depuis main
  /code-review <fichier>        Analyser un fichier spécifique
  /code-review security         Focus sécurité OWASP
  /code-review performance      Focus performances
  /code-review rationalization  Focus duplications/rationalisation

**Verdicts** : APPROVED / APPROVED WITH RESERVATIONS / REJECTED
```

**Formats possibles** :
- `/code-review` : Analyse les fichiers modifiés depuis main
- `/code-review <fichier>` : Analyse un fichier spécifique
- `/code-review security` : Focus sur la sécurité OWASP
- `/code-review performance` : Focus sur les performances
- `/code-review rationalization` : Focus sur la rationalisation/duplications

## Instructions

Utilise le Task tool pour lancer le sous-agent code-reviewer avec les paramètres suivants :

```
subagent_type: "code-reviewer"
description: "Revue de code"
prompt: voir ci-dessous
```

### Prompt à transmettre au sous-agent

```
Effectue une revue de code pour BuzzControl.

**Contexte projet :** Voir `context/COMMON.md` section 1
**Framework Qualité :** Voir `context/QUALITY.md`
- Framework review : section 4
- Sévérités : section 3
- Verdicts : section 5
- Structure rapport : section 8
- Règles : section 10

**Input utilisateur :** $ARGUMENTS

**Mode** :
- Aucun argument : `git diff main --name-only`
- Fichier : analyser ce fichier
- Focus (security/performance/rationalization) : adapter l'analyse
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
  content: "Revue de code demandée: $ARGUMENTS",
  summary: "Review: $ARGUMENTS"
)
```

### Mode SOLO
Lance le sous-agent `code-reviewer` avec le Task tool.
