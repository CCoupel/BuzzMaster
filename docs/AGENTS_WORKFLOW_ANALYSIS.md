# Analyse et Optimisation des Agents et Workflow Claude Code

**Date** : 2026-01-27
**Version analysée** : v2.45.x
**Branche** : `claude/optimize-agents-workflow-jIBZ9`

---

## Table des matières

1. [Architecture actuelle](#1-architecture-actuelle)
2. [Analyse des problèmes](#2-analyse-des-problèmes)
3. [Recommandation : Séparation DEV en Frontend/Backend](#3-recommandation--séparation-dev-en-frontendbackend)
4. [Optimisations des modèles](#4-optimisations-des-modèles)
5. [Améliorations du workflow](#5-améliorations-du-workflow)
6. [Nouvelles commandes proposées](#6-nouvelles-commandes-proposées)
7. [Plan d'implémentation](#7-plan-dimplémentation)

---

## 1. Architecture actuelle

### 1.1 Agents existants (8)

| Agent | Modèle | Couleur | Rôle |
|-------|--------|---------|------|
| `implementation-planner` | opus | red | Planification de features |
| `dev-feature-implementation` | opus | green | Implémentation code (Go + React) |
| `code-reviewer` | sonnet | yellow | Revue de code |
| `QA` | sonnet | purple | Tests et validation |
| `doc-updater` | sonnet | cyan | Mise à jour documentation |
| `deploy` | sonnet | red | Déploiement QUALIF/PROD |
| `git-squash-merge` | haiku | red | Workflow Git |
| `marketing-release` | sonnet | cyan | Communication marketing |

### 1.2 Commandes existantes (10)

| Commande | Workflow | Agents appelés |
|----------|----------|----------------|
| `/backlog` | Consultation/Ajout | Aucun (exécution directe) |
| `/plan` | Planification | `implementation-planner` |
| `/dev` | Implémentation | `dev-feature-implementation` |
| `/review` | Revue complète | `dev-feature-implementation` (analyse) → `QA` |
| `/qa` | Tests | `QA` |
| `/doc` | Documentation | `doc-updater` |
| `/deploy` | Déploiement | `deploy` |
| `/feature` | Workflow complet | PLAN → DEV → QA → DOC → DEPLOY |
| `/bugfix` | Workflow bugfix | DEV → QA → DOC → DEPLOY |
| `/marketing` | Communication | `marketing-release` |

### 1.3 Workflow actuel

```
┌─────────┐    ┌─────────┐    ┌─────────┐    ┌─────────┐    ┌─────────┐    ┌─────────┐
│  PLAN   │ →  │   DEV   │ →  │ REVIEW  │ →  │   QA    │ →  │   DOC   │ →  │ DEPLOY  │
│ (opus)  │    │ (opus)  │    │(sonnet) │    │(sonnet) │    │(sonnet) │    │(sonnet) │
└─────────┘    └─────────┘    └─────────┘    └─────────┘    └─────────┘    └─────────┘
```

---

## 2. Analyse des problèmes

### 2.1 Agent DEV monolithique

**Problème critique** : Un seul agent gère à la fois :
- Backend Go (models, engine, protocol, handlers, tests)
- Frontend React (pages, components, hooks, CSS)
- Tests E2E

**Impacts** :
- ❌ Contexte très large → prompts longs → coûts élevés
- ❌ Spécialisation impossible → qualité moyenne sur tout
- ❌ Pas de parallélisation possible
- ❌ Utilisation d'Opus (modèle le plus cher) pour des tâches variées

### 2.2 Modèles mal assignés

| Agent | Modèle actuel | Problème |
|-------|---------------|----------|
| `implementation-planner` | opus | Planification ne nécessite pas le modèle le plus puissant |
| `dev-feature-implementation` | opus | Trop cher pour du code standard |
| `QA` | sonnet | Pourrait utiliser haiku pour tests automatisés |
| `deploy` | sonnet | Scripts bash simples → haiku suffisant |

**Coût estimé par session de développement complète** :
- Actuel (2x opus + 4x sonnet) : ~$15-25
- Optimisé (6x sonnet/haiku) : ~$3-8

### 2.3 Commande /review mal conçue

La commande `/review` actuelle :
1. Lance `dev-feature-implementation` pour **analyser** le code
2. Puis lance `QA` pour les tests

**Problème** : L'agent `code-reviewer` existe mais n'est **jamais appelé** par `/review` !

### 2.4 Pas d'orchestration automatique

- Chaque transition entre agents nécessite une intervention manuelle
- Pas de détection automatique des dépendances frontend/backend
- Pas de parallélisation des tâches indépendantes

### 2.5 Redondance dans les prompts

Chaque commande répète :
```
**Contexte projet :**
- Répertoire : C:\Users\cyril\Documents\VScode\buzzcontrol
- Serveur Go : server-go/
- Frontend React : server-go/web/src/
- Config version : server-go/config.json
```

→ Devrait être centralisé dans un fichier de configuration.

---

## 3. Recommandation : Séparation DEV en Frontend/Backend

### 3.1 Décision

**OUI, il faut séparer l'agent DEV en deux agents spécialisés.**

### 3.2 Justification

| Critère | Agent unique | Agents séparés |
|---------|--------------|----------------|
| Spécialisation | ❌ Généraliste | ✅ Expert par domaine |
| Qualité code | ⚠️ Variable | ✅ Optimale |
| Coût | ❌ Opus obligatoire | ✅ Sonnet suffisant |
| Parallélisation | ❌ Impossible | ✅ Possible |
| Maintenance prompts | ❌ Prompt géant | ✅ Prompts ciblés |
| Debugging | ❌ Difficile | ✅ Localisé |

### 3.3 Nouveaux agents proposés

#### Agent `dev-backend`

```yaml
name: dev-backend
model: sonnet
color: green
description: "Agent spécialisé Go backend"
```

**Responsabilités** :
- `internal/game/models.go` - Structures de données
- `internal/game/engine.go` - Logique métier
- `internal/game/engine_test.go` - Tests unitaires
- `internal/protocol/messages.go` - Protocol WebSocket
- `internal/server/*.go` - Serveurs HTTP/TCP/WebSocket
- `cmd/server/main.go` - Handlers et orchestration

**Expertise** :
- Go idiomatique (error handling, defer, goroutines)
- Patterns backend (repository, service, handler)
- Tests table-driven
- Concurrence et thread-safety

#### Agent `dev-frontend`

```yaml
name: dev-frontend
model: sonnet
color: blue
description: "Agent spécialisé React frontend"
```

**Responsabilités** :
- `web/src/pages/*.jsx` - Pages React
- `web/src/components/*.jsx` - Composants réutilisables
- `web/src/hooks/*.js` - Custom hooks
- `web/src/*.css` - Styles CSS

**Expertise** :
- React patterns (hooks, memo, context)
- CSS moderne (flexbox, grid, variables)
- Accessibilité (ARIA, keyboard navigation)
- Responsive design
- Contrainte TV statique (pas de scroll)

### 3.4 Nouveau workflow avec agents séparés

```
                          ┌─────────────┐
                    ┌────→│ DEV-BACKEND │────┐
                    │     │  (sonnet)   │    │
┌─────────┐    ┌────┴───┐ └─────────────┘    │    ┌─────────┐    ┌─────────┐
│  PLAN   │ →  │DISPATCH│                    ├───→│ REVIEW  │ →  │   QA    │ → ...
│ (sonnet)│    │        │ ┌─────────────┐    │    │(sonnet) │    │ (haiku) │
└─────────┘    └────┬───┘ │DEV-FRONTEND │    │    └─────────┘    └─────────┘
                    │     │  (sonnet)   │    │
                    └────→└─────────────┘────┘
                          (parallèle si possible)
```

**Avantages** :
1. **Parallélisation** : Backend et frontend peuvent être développés simultanément si pas de dépendances
2. **Spécialisation** : Chaque agent maîtrise son domaine
3. **Coût réduit** : Sonnet au lieu d'Opus = -80% de coût
4. **Meilleure qualité** : Prompts ciblés = meilleurs résultats

---

## 4. Optimisations des modèles

### 4.1 Tableau comparatif

| Agent | Modèle actuel | Modèle proposé | Économie | Justification |
|-------|---------------|----------------|----------|---------------|
| `implementation-planner` | opus | **sonnet** | -70% | Analyse et structuration, pas de création complexe |
| `dev-backend` | (nouveau) | **sonnet** | N/A | Go est bien documenté, sonnet suffit |
| `dev-frontend` | (nouveau) | **sonnet** | N/A | React patterns standards |
| `code-reviewer` | sonnet | sonnet | 0% | Analyse nécessite réflexion |
| `QA` | sonnet | **haiku** | -50% | Exécution de commandes et parsing de résultats |
| `doc-updater` | sonnet | sonnet | 0% | Rédaction technique |
| `deploy` | sonnet | **haiku** | -50% | Scripts bash prédéfinis |
| `git-squash-merge` | haiku | haiku | 0% | OK |
| `marketing-release` | sonnet | sonnet | 0% | Création de contenu |

### 4.2 Économie estimée par feature

**Avant (workflow actuel)** :
- PLAN (opus) + DEV (opus) + REVIEW (sonnet) + QA (sonnet) + DOC (sonnet) + DEPLOY (sonnet)
- Coût moyen : $15-25

**Après (workflow optimisé)** :
- PLAN (sonnet) + DEV-BACKEND (sonnet) + DEV-FRONTEND (sonnet) + REVIEW (sonnet) + QA (haiku) + DOC (sonnet) + DEPLOY (haiku)
- Coût moyen : $5-12

**Économie : 40-60%**

---

## 5. Améliorations du workflow

### 5.1 Workflow `/feature` amélioré

```
/feature "Description"
    │
    ▼
┌─────────────────────────────────────────────────────────────┐
│ Phase 0: BACKLOG (auto)                                     │
│ - Recherche entrée correspondante                           │
│ - Confirmation utilisateur                                  │
└─────────────────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────────────┐
│ Phase 1: PLAN (sonnet)                                      │
│ - Analyse backlog                                           │
│ - Création branche + version                                │
│ - Plan structuré avec dépendances frontend/backend          │
│ ⏸️ VALIDATION UTILISATEUR                                   │
└─────────────────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────────────┐
│ Phase 2: DEV (parallélisable)                               │
│                                                             │
│ Si dépendances:  BACKEND → FRONTEND (séquentiel)            │
│ Si indépendant:  BACKEND ║ FRONTEND (parallèle)             │
│                                                             │
│ ┌─────────────────┐    ┌─────────────────┐                  │
│ │  DEV-BACKEND    │    │  DEV-FRONTEND   │                  │
│ │  (sonnet)       │    │  (sonnet)       │                  │
│ │                 │    │                 │                  │
│ │  models.go      │    │  Pages JSX      │                  │
│ │  engine.go      │    │  Components     │                  │
│ │  protocol.go    │    │  Hooks          │                  │
│ │  handlers       │    │  CSS            │                  │
│ │  tests Go       │    │                 │                  │
│ └─────────────────┘    └─────────────────┘                  │
└─────────────────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────────────┐
│ Phase 3: REVIEW (sonnet)                                    │
│ - Analyse qualité code                                      │
│ - Détection duplications                                    │
│ - Vérification sécurité                                     │
│ - Rapport structuré                                         │
│                                                             │
│ Si ❌ REJECTED → Retour Phase 2 avec corrections            │
└─────────────────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────────────┐
│ Phase 4: QA (haiku)                                         │
│ - Build production                                          │
│ - Tests unitaires Go                                        │
│ - Tests E2E                                                 │
│ - Rapport QA                                                │
│                                                             │
│ Si ❌ NOT VALIDATED → Retour Phase 2 avec erreurs           │
│ ⏸️ VALIDATION UTILISATEUR (tests manuels)                   │
└─────────────────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────────────┐
│ Phase 5: DOC (sonnet)                                       │
│ - CHANGELOG.md                                              │
│ - CLAUDE.md                                                 │
│ - ADMIN_GUIDE.md                                            │
│ - Backlog update                                            │
│ - Version finalization                                      │
└─────────────────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────────────┐
│ Phase 6: DEPLOY QUALIF (haiku)                              │
│ - Build Windows + ARM64                                     │
│ - Tests post-build                                          │
│ - Archive QUALIF                                            │
│ ⏸️ FIN WORKFLOW /feature                                    │
└─────────────────────────────────────────────────────────────┘
```

### 5.2 Détection automatique des dépendances

Le PLAN agent doit identifier :

```markdown
## Analyse des dépendances

### Backend → Frontend (séquentiel obligatoire)
- [ ] Nouvelles actions WebSocket utilisées par le frontend
- [ ] Nouveaux champs GameState consommés par React
- [ ] Nouveaux endpoints HTTP appelés par le frontend

### Frontend → Backend (rare)
- [ ] Nouveaux formats de données envoyés au backend

### Indépendant (parallélisable)
- [ ] Refactoring backend uniquement
- [ ] Styling CSS uniquement
- [ ] Tests unitaires isolés
```

### 5.3 Gestion des cycles DEV ↔ QA

```
MAX_CYCLES = 3

cycle = 0
while cycle < MAX_CYCLES:
    run_dev()
    result = run_qa()

    if result == "VALIDATED":
        break
    elif result == "VALIDATED_WITH_RESERVATIONS":
        user_confirm = ask_user("Continuer malgré réserves ?")
        if user_confirm:
            break
    else:  # NOT_VALIDATED
        cycle += 1
        if cycle == MAX_CYCLES:
            escalate_to_user("Maximum de cycles atteint")
```

---

## 6. Nouvelles commandes proposées

### 6.1 `/code-review` (nouvelle)

Commande dédiée pour lancer l'agent `code-reviewer` directement.

```markdown
# Commande /code-review

Lance l'agent code-reviewer pour analyser le code récemment modifié.

## Arguments
- `/code-review` : Analyse les fichiers modifiés depuis main
- `/code-review <fichier>` : Analyse un fichier spécifique
- `/code-review --security` : Focus sécurité
- `/code-review --performance` : Focus performance
```

### 6.2 `/dev-backend` et `/dev-frontend` (nouvelles)

Commandes pour lancer les agents séparément si nécessaire.

```markdown
# Commande /dev-backend

Lance uniquement l'agent dev-backend pour du développement Go.

## Arguments
- `/dev-backend <plan>` : Implémente la partie backend du plan
- `/dev-backend fix "bug"` : Corrige un bug backend
```

### 6.3 `/parallel` (nouvelle)

Commande pour lancer des tâches en parallèle.

```markdown
# Commande /parallel

Lance plusieurs agents en parallèle.

## Exemple
/parallel "backend: ajouter champ X" "frontend: afficher champ X"

→ Lance dev-backend ET dev-frontend simultanément
```

---

## 7. Plan d'implémentation

### Phase 1 : Créer les nouveaux agents (priorité haute)

1. **Créer `dev-backend.md`**
   - Copier la structure de `dev-feature-implementation.md`
   - Spécialiser pour Go
   - Changer le modèle en `sonnet`

2. **Créer `dev-frontend.md`**
   - Copier la structure de `dev-feature-implementation.md`
   - Spécialiser pour React
   - Changer le modèle en `sonnet`

3. **Mettre à jour `implementation-planner.md`**
   - Changer le modèle en `sonnet`
   - Ajouter la détection des dépendances frontend/backend

4. **Mettre à jour `qa.md`**
   - Changer le modèle en `haiku`
   - Simplifier le prompt

5. **Mettre à jour `deploy.md`**
   - Changer le modèle en `haiku`
   - Simplifier le prompt

### Phase 2 : Créer les nouvelles commandes (priorité moyenne)

1. **Créer `/code-review.md`**
   - Appelle directement `code-reviewer`

2. **Créer `/dev-backend.md` et `/dev-frontend.md`**
   - Commandes séparées pour chaque agent

3. **Mettre à jour `/feature.md`**
   - Ajouter logique de dispatch backend/frontend
   - Ajouter support parallélisation

4. **Mettre à jour `/review.md`**
   - Corriger pour appeler `code-reviewer`

### Phase 3 : Optimisations avancées (priorité basse)

1. **Créer un fichier de contexte partagé**
   - `.claude/context/project.md`
   - Évite la répétition dans chaque commande

2. **Ajouter des métriques**
   - Tracking du temps par agent
   - Tracking des cycles DEV ↔ QA

---

## Annexe : Structure de fichiers proposée

```
.claude/
├── agents/
│   ├── dev-backend.md          # NOUVEAU
│   ├── dev-frontend.md         # NOUVEAU
│   ├── implementation-planner.md  # MODIFIÉ (sonnet)
│   ├── code-reviewer.md        # INCHANGÉ
│   ├── qa.md                   # MODIFIÉ (haiku)
│   ├── doc-updater.md          # INCHANGÉ
│   ├── deploy.md               # MODIFIÉ (haiku)
│   ├── git-squash-merge.md     # INCHANGÉ
│   └── marketing-release.md    # INCHANGÉ
│
├── commands/
│   ├── backlog.md              # INCHANGÉ
│   ├── plan.md                 # INCHANGÉ
│   ├── dev.md                  # MODIFIÉ (dispatch backend/frontend)
│   ├── dev-backend.md          # NOUVEAU
│   ├── dev-frontend.md         # NOUVEAU
│   ├── code-review.md          # NOUVEAU
│   ├── review.md               # MODIFIÉ (utilise code-reviewer)
│   ├── qa.md                   # INCHANGÉ
│   ├── doc.md                  # INCHANGÉ
│   ├── deploy.md               # INCHANGÉ
│   ├── feature.md              # MODIFIÉ (dispatch + parallélisation)
│   ├── bugfix.md               # INCHANGÉ
│   └── marketing.md            # INCHANGÉ
│
└── context/
    └── project.md              # NOUVEAU - Contexte partagé
```

---

## Conclusion

### Recommandations prioritaires

1. **Séparer l'agent DEV** → Impact majeur sur qualité et coût
2. **Corriger la commande /review** → Utiliser l'agent code-reviewer existant
3. **Optimiser les modèles** → Économie de 40-60%
4. **Ajouter la parallélisation** → Gain de temps significatif

### ROI estimé

- **Temps** : -20% par feature (parallélisation)
- **Coût** : -50% par feature (modèles optimisés)
- **Qualité** : +30% (spécialisation des agents)

---

*Document généré le 2026-01-27*
