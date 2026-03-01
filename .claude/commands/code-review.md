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

## Action immédiate

**Détecter le mode** — Lire `~/.claude/teams/TEAM-Buzz/config.json` :
- **Fichier trouvé → Mode TEAM** : transmettre au CDP existant
- **Fichier absent** : spawner le CDP — il orchestrera les agents nécessaires

### Mode TEAM
```
SendMessage(
  type: "message",
  recipient: "cdp",
  content: "Revue de code demandée: $ARGUMENTS",
  summary: "Review: $ARGUMENTS"
)
```

### Sans TEAM
```
subagent_type: "cdp"
description: "Revue de code"
prompt: Revue de code demandée: $ARGUMENTS
```
