# Commande /feature - Workflow Feature Complet

Orchestre le workflow complet de développement d'une feature via les sous-agents spécialisés.

## Argument reçu

$ARGUMENTS

## Workflow complet

```
[Git] → PLAN → DEV → REVIEW → QA → DOC → DEPLOY(QUALIF) → [FIN]
```

**Note** : Le déploiement en PROD se fait via `/deploy PROD` après validation de la QUALIF.

## Instructions

Cette commande orchestre le workflow jusqu'à la QUALIF. Elle lance les sous-agents dans l'ordre avec le point de validation utilisateur.

### Phase 0 : Préparation Git (via PLAN)

L'agent PLAN s'occupe de la préparation Git :

```bash
git checkout main && git pull origin main
git checkout -b feature/<nom-court>
```

Puis incrémente y dans `server-go/config.json` : `2.39.0` → `2.40.0`

```bash
git add server-go/config.json
git commit -m "chore(version): Start v2.40.0 - <feature name>"
git push -u origin feature/<nom-court>
```

### Phase 1 : Planification (PLAN)

Lance le sous-agent **implementation-planner** via Task tool :

```
subagent_type: "implementation-planner"
description: "Créer plan d'implémentation"
prompt: "Crée un plan d'implémentation détaillé pour BuzzControl.

**Contexte projet :**
- Répertoire : C:\Users\cyril\Documents\VScode\buzzcontrol
- Serveur Go : server-go/
- Config version : server-go/config.json
- Backlog : backlog/*.md

**Demande utilisateur :** $ARGUMENTS

**Actions :**
1. Analyser le backlog ou la description
2. Créer la branche feature/<nom-court>
3. Incrémenter la version mineure (y) dans config.json
4. Commit et push initial
5. Produire le plan d'implémentation structuré

**IMPORTANT :** Attendre validation utilisateur avant de passer à DEV."
```

**⏸️ POINT DE VALIDATION : Attendre que l'utilisateur valide le plan**

### Phase 2 : Implémentation (DEV)

Après validation du plan, lance le sous-agent **dev-feature-implementation** via Task tool :

```
subagent_type: "dev-feature-implementation"
description: "Implémenter feature"
prompt: "Implémente le code pour BuzzControl selon le plan validé.

**Contexte projet :**
- Répertoire : C:\Users\cyril\Documents\VScode\buzzcontrol
- Serveur Go : server-go/
- Frontend React : server-go/web/src/
- Config version : server-go/config.json

**Plan :** [Plan validé de la phase précédente]

**Actions :**
1. Incrémenter z dans config.json
2. Implémenter Backend → Frontend → Tests
3. Commits atomiques par tâche
4. Push en fin de cycle"
```

### Phase 3 : Revue de code (REVIEW)

Lance le sous-agent **code-reviewer** via Task tool :

```
subagent_type: "code-reviewer"
description: "Revue de code"
prompt: "Effectue une revue de code complète pour BuzzControl.

**Contexte projet :**
- Répertoire : C:\Users\cyril\Documents\VScode\buzzcontrol
- Architecture : CLAUDE.md

**Actions :**
1. Analyser qualité (Go, React)
2. Analyser sécurité (OWASP)
3. Vérifier conformité architecture
4. Détecter duplications de code
5. Produire rapport de review

**Si problèmes critiques :** Retour à DEV avec feedback."
```

### Phase 4 : Tests QA (QA)

Lance le sous-agent **QA** via Task tool :

```
subagent_type: "QA"
description: "Exécuter tests QA"
prompt: "Exécute la procédure de tests QA complète pour BuzzControl.

**Contexte projet :**
- Répertoire : C:\Users\cyril\Documents\VScode\buzzcontrol
- Serveur Go : server-go/

**Actions :**
1. Build de production
2. Tests unitaires
3. Tests E2E
4. Rapport de qualité

**Si échecs :** Retour à DEV avec erreurs."
```

### Phase 5 : Documentation (DOC)

Lance le sous-agent **doc-updater** via Task tool :

```
subagent_type: "doc-updater"
description: "Mettre à jour documentation"
prompt: "Mets à jour la documentation pour BuzzControl.

**Contexte projet :**
- Répertoire : C:\Users\cyril\Documents\VScode\buzzcontrol
- Config version : server-go/config.json

**Type :** feature

**Actions :**
1. CHANGELOG.md : Ajouter entrée version
2. CLAUDE.md : Mettre à jour sections impactées
3. ADMIN_GUIDE.md : Documenter fonctionnalités user-facing
4. Finaliser version (reset z à 0)
5. Commit et push"
```

### Phase 6 : Déploiement QUALIF (DEPLOY)

Lance le sous-agent **deploy** via Task tool :

```
subagent_type: "deploy"
description: "Déploiement QUALIF"
prompt: "Déploie le serveur BuzzControl vers l'environnement QUALIF.

**Contexte projet :**
- Répertoire : C:\Users\cyril\Documents\VScode\buzzcontrol
- Serveur Go : server-go/
- Config version : server-go/config.json

**Environnement :** QUALIF

**Actions :**
1. Build Windows + ARM64
2. Tests post-build
3. Créer archive QUALIF"
```

## Fin du workflow /feature

**✅ Le workflow /feature s'arrête ici.**

Pour continuer vers la production :
1. **Valider la QUALIF** manuellement
2. **Lancer** `/deploy PROD` pour le déploiement en production
3. **Lancer** `/marketing` pour la communication (optionnel)

## Gestion des erreurs

| Situation | Action |
|-----------|--------|
| REVIEW trouve problèmes critiques | Retour à Phase 2 (DEV) avec feedback |
| QA échoue | Retour à Phase 2 (DEV) avec tests en échec |
| Build échoue | Retour à Phase 2 (DEV) pour correction |
| Maximum 3 cycles DEV ↔ REVIEW/QA | Escalade vers utilisateur |

## Point de validation obligatoire

- **Après PLAN** : L'utilisateur doit valider le plan avant implémentation

## Action immédiate

Lance maintenant la **Phase 1 (PLAN)** avec le Task tool :

```
Task tool:
- subagent_type: "implementation-planner"
- description: "Créer plan d'implémentation"
- prompt: [prompt de la Phase 1 avec $ARGUMENTS]
```
