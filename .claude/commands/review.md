# Commande /review - Revue de Code

Lance le sous-agent REVIEW pour analyser le code implémenté et détecter les problèmes.

## Argument reçu (optionnel)

$ARGUMENTS

**Formats possibles** :
- `/review` : Auto-détecte depuis git diff main
- `/review internal/game/engine.go` : Fichier spécifique
- `/review "Feature QCM hints"` : Avec contexte
- `/review HEAD~5..HEAD` : Range de commits

## Instructions

Utilise le Task tool pour lancer le sous-agent code-reviewer avec les paramètres suivants :

```
subagent_type: "code-reviewer"
description: "Revue de code"
prompt: voir ci-dessous
```

### Prompt à transmettre au sous-agent

```
Effectue une revue de code complète pour BuzzControl.

**Contexte projet :**
- Répertoire : C:\Users\cyril\Documents\VScode\buzzcontrol
- Serveur Go : server-go/
- Frontend React : server-go/web/src/
- Architecture : CLAUDE.md

**Input utilisateur :** $ARGUMENTS

**Étapes à exécuter :**

1. **Collecter le contexte**
   - Branche : git branch --show-current
   - Fichiers modifiés : git diff main --name-only
   - Diff complet : git diff main
   - Commits récents : git log main..HEAD --oneline

2. **Analyser le code selon les catégories**

   | Catégorie | Points vérifiés |
   |-----------|-----------------|
   | Qualité Go | Naming, fonctions courtes, error handling, idiomatic Go |
   | Qualité React | Hooks, props, state minimal, useEffect deps, memoization |
   | Sécurité OWASP | Injection, XSS, auth, secrets, config |
   | Performance | Boucles infinies, re-renders, structures de données |
   | Architecture | Conformité CLAUDE.md, patterns existants, rétrocompat |
   | Tests | Présence, couverture, qualité |
   | Rationalisation | Duplications, patterns répétés, code consolidable |

3. **Classifier les problèmes**

   | Niveau | Signification | Action |
   |--------|---------------|--------|
   | 🔴 Critical | Faille sécurité, bug majeur, ne compile pas | DOIT être corrigé |
   | 🟡 Warning | Mauvaise pratique, perf suboptimale, tests insuffisants | Devrait être corrigé |
   | 🟠 Rationalization | Duplications 70%+, patterns 3+ occurrences | Devrait être consolidé |
   | 🔵 Suggestion | Optimisation possible, refactoring suggéré | Optionnel |

4. **Produire le rapport structuré**

   ```markdown
   # Review Report: [Feature Name]

   ## 📊 Overview
   - Files analyzed: X
   - Lines added/removed: +Y / -Z
   - Overall status: ✅/⚠️/❌

   ## ✅ Positive Points
   ## ⚠️ Issues Detected (Critical / Warning / Suggestion)
   ## 🔒 Security Analysis
   ## 📈 Performance Analysis
   ## 🏗️ Architecture Conformity
   ## 🔄 Code Rationalization Analysis
   ## 📝 Test Quality
   ## 🎯 Recommendations
   ## ✅ Final Decision
   ```

**Critères de décision :**
| Décision | Critères |
|----------|----------|
| ✅ APPROVED | Aucun critique, qualité OK, tests présents, sécurité OK |
| ⚠️ APPROVED WITH RESERVATIONS | Pas de bloquant, quelques warnings |
| ❌ REJECTED | Critique détecté, faille sécurité, bug majeur, tests manquants |

**Checklists d'analyse :**

Backend Go :
- Naming clair et cohérent
- Fonctions < 50 lignes
- Erreurs gérées (pas ignorées)
- Pas de code dupliqué
- Go idiomatique (defer, error patterns)

Frontend React :
- Composants fonctionnels + hooks
- Props bien définies
- State minimal
- useEffect deps corrects
- Memoization appropriée

Sécurité OWASP :
- Pas d'injection
- Pas de XSS
- Pas de secrets hardcodés
- Auth/permissions vérifiées

**Règles critiques :**
- JAMAIS approuver avec faille sécurité critique
- JAMAIS être trop indulgent (signaler les doutes)
- JAMAIS corriger le code (review seulement)
- JAMAIS oublier d'analyser les tests
- TOUJOURS analyser la logique, pas juste la syntaxe
- TOUJOURS chercher les duplications de code
```

## Action immédiate

Lance maintenant le sous-agent code-reviewer avec le Task tool.
