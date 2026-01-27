# Commande /code-review - Revue de Code

Lance le sous-agent code-reviewer pour analyser le code récemment modifié.

## Argument reçu (optionnel)

$ARGUMENTS

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

**Contexte projet :**
- Répertoire : /home/user/BuzzMaster
- Serveur Go : server-go/
- Frontend React : server-go/web/src/
- Architecture : CLAUDE.md

**Input utilisateur :** $ARGUMENTS

**Étapes à exécuter :**

1. **Identifier les fichiers à analyser**
   - Si aucun argument : `git diff main --name-only`
   - Si fichier spécifié : analyser ce fichier uniquement
   - Si focus spécifié : adapter l'analyse au focus

2. **Analyser selon le framework de review**

   | Catégorie | Vérifications |
   |-----------|---------------|
   | Qualité | Naming, fonctions courtes, comments, errors |
   | Sécurité | Injection, XSS, secrets, validation |
   | Performance | Boucles, re-renders, structures |
   | Architecture | CLAUDE.md conformité, patterns |
   | Rationalisation | Duplications, patterns répétés |

3. **Produire le rapport structuré**

   - 📊 Overview : Fichiers, lignes, statut global
   - ✅ Points positifs
   - ⚠️ Issues détectées (Critical / Warning / Rationalization / Suggestion)
   - 🔒 Analyse sécurité
   - 📈 Analyse performance
   - 🏗️ Conformité architecture
   - 🔄 Analyse rationalisation (duplications)
   - 📝 Qualité des tests
   - 🎯 Recommandations
   - ✅ Décision finale : APPROVED / APPROVED WITH RESERVATIONS / REJECTED

**Niveaux de sévérité :**
- 🔴 Critical : Bloquant (sécurité, bug majeur, ne compile pas)
- 🟡 Warning : Important mais non-bloquant
- 🟠 Rationalization : Duplication > 70% ou pattern 3+ occurrences
- 🔵 Suggestion : Amélioration optionnelle

**Règles critiques :**
- NE PAS approuver si issue critique de sécurité
- NE PAS être trop laxiste (mieux vaut signaler un doute)
- NE PAS corriger le code (tu ne fais que reviewer)
- NE PAS oublier d'analyser les tests
- NE PAS ignorer les duplications de code
```

## Action immédiate

Lance maintenant le sous-agent code-reviewer avec le Task tool.
