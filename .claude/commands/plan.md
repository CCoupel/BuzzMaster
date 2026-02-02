# Commande /plan - Planification d'Implémentation

Lance le sous-agent implementation-planner pour créer un plan d'implémentation détaillé AVANT tout développement.

## Argument reçu

$ARGUMENTS

**Formats possibles** :
- `/plan backlog/memory-game.md Phase 6` : Depuis un backlog
- `/plan "Description de la feature"` : Description libre
- `/plan bugfix "Description du bug"` : Planification de bugfix

## Instructions

Utilise le Task tool pour lancer le sous-agent implementation-planner avec les paramètres suivants :

```
subagent_type: "implementation-planner"
description: "Créer plan d'implémentation"
prompt: voir ci-dessous
```

### Prompt à transmettre au sous-agent

```
Crée un plan d'implémentation détaillé pour BuzzControl.

**Contexte projet :** Voir `context/COMMON.md` section 1
**Versionnement :** Voir `context/COMMON.md` section 5
**Workflows :** Voir `context/COMMON.md` section 9

**Références additionnelles :**
- Backlog : backlog/*.md
- Procédure : docs/DEV_PROCEDURE.md

**Demande utilisateur :** $ARGUMENTS

**Étapes à exécuter :**

1. **Analyser la demande**
   - Si backlog spécifié : lire backlog/<nom>.md
   - Consulter CLAUDE.md pour l'architecture actuelle
   - Consulter server-go/internal/game/models.go pour les modèles
   - Consulter CHANGELOG.md pour l'historique des versions

2. **Créer la branche et incrémenter la version**
   Voir `context/COMMON.md` section 6.1 pour la procédure complète.
   # Incrémenter Y (minor) pour nouvelle feature

3. **Produire le plan structuré**

   | Section | Contenu |
   |---------|---------|
   | 📊 Analyse | Branche, version cible, complexité, risques |
   | 🎯 Objectif | Description claire de la feature |
   | 📝 Tâches | Liste ordonnée Backend → Frontend → Tests → Docs |
   | 🧪 Tests | Stratégie tests unitaires et E2E |
   | 🔗 Dépendances | Ce qui doit exister avant |
   | ⚠️ Risques | Problèmes potentiels et mitigations |
   | ✅ Validation | Checklist de conformité |

4. **Structure des tâches**
   - 1. Backend (Go) : models.go, engine.go, tests, protocol, handlers
   - 2. Frontend (React) : Admin UI, TV display, CSS
   - 3. Tests E2E : e2e_test.go
   - 4. Documentation : CLAUDE.md, CHANGELOG.md

**Critères de qualité :**
- Exhaustif : Toutes les tâches listées
- Ordonné : Backend → Frontend → Tests → Docs
- Précis : Chemins de fichiers, noms de fonctions
- Actionnable : L'agent DEV peut suivre sans ambiguïté
- Rétrocompatible : Valeurs par défaut pour nouveaux champs

**Règles critiques :**
- NE PAS implémenter de code (rôle de l'agent DEV)
- NE PAS oublier les tests et la documentation
- NE PAS créer de breaking changes sans migration
- L'affichage TV (/tv) est STATIQUE - pas de scroll

**Versionnement :** Voir `context/COMMON.md` section 5.2
- Y (minor) : Nouvelles features ← TU INCRÉMENTES CELUI-CI

**Attendre validation utilisateur avant de passer au développement.**
```

## Action immédiate

Lance maintenant le sous-agent implementation-planner avec le Task tool.
