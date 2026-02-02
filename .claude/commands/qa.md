# Commande /qa - Lancer les Tests QA

Lance le sous-agent QA pour exécuter les tests et générer un rapport de qualité.

## Argument reçu (optionnel)

$ARGUMENTS

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

Lance maintenant le sous-agent QA avec le Task tool.
