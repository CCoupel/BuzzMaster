# Agents Spécialisés BuzzMaster

Ce dossier contient les instructions pour les **6 agents spécialisés** du projet BuzzMaster.

---

## 🎭 Concept

L'orchestrateur principal (Claude Code) délègue des tâches spécifiques à des agents spécialisés en utilisant le **Task tool**.

Chaque agent a un rôle clair et des responsabilités précises.

---

## 📋 Les 6 agents

| Agent | Rôle | Appelé quand | Input | Output |
|-------|------|--------------|-------|--------|
| **PLAN** | Planification | En premier, avant toute implémentation | Spécification backlog | Plan d'implémentation détaillé |
| **DEV** | Développement | Après validation du plan | Plan d'implémentation | Code + tests + commits |
| **REVIEW** | Revue de code | Après développement | Code modifié | Rapport de review (qualité, sécurité) |
| **QA** | Tests & Qualité | Après review | Code à tester | Rapport de tests (PASS/FAIL) |
| **DOC** | Documentation | Après validation QA | Feature implémentée | Documentation mise à jour |
| **DEPLOY** | Déploiement | En dernier | Version à déployer | Déploiement QUALIF/PROD |

---

## 🔄 Workflow complet

```
Utilisateur : "Implémente Memory Phase 6"
      │
      ▼
┌──────────────────────────────────────┐
│  Orchestrateur (Claude Code)        │
│  Analyse la demande et orchestre     │
└──────────────────────────────────────┘
      │
      │ 1️⃣ Planification
      ▼
┌──────────┐
│   PLAN   │ Lit backlog/memory-game.md, crée plan détaillé
└────┬─────┘
     │ Plan validé par utilisateur ✓
     │
     │ 2️⃣ Développement
     ▼
┌──────────┐
│   DEV    │ Implémente selon le plan, crée tests, commit
└────┬─────┘
     │ Code + tests ✓
     │
     │ 3️⃣ Revue de code
     ▼
┌──────────┐
│  REVIEW  │ Analyse qualité, sécurité, standards
└────┬─────┘
     │ Review OK ✓
     │
     │ 4️⃣ Tests
     ▼
┌──────────┐
│    QA    │ Exécute tests unitaires + E2E, build
└────┬─────┘
     │ Tests PASS ✓
     │
     │ 5️⃣ Documentation
     ▼
┌──────────┐
│   DOC    │ Met à jour CHANGELOG, CLAUDE.md, etc.
└────┬─────┘
     │ Docs à jour ✓
     │
     │ 6️⃣ Déploiement
     ▼
┌──────────┐
│  DEPLOY  │ Build + déploiement QUALIF ou PROD
└────┬─────┘
     │
     ▼
   ✅ Feature complète et déployée
```

---

## 🚀 Comment utiliser

### En tant qu'utilisateur

Vous dites simplement :
```
"Implémente Memory Phase 6"
```

L'orchestrateur (Claude Code) :
1. Comprend que c'est une nouvelle feature
2. Appelle automatiquement les agents dans l'ordre
3. Vous présente les résultats à chaque étape
4. Demande validation avant de continuer

### En tant qu'orchestrateur (Claude Code)

Quand je reçois une demande, j'utilise le Task tool :

```javascript
// Exemple : Lancer l'agent PLAN
Task({
  subagent_type: "general-purpose",
  description: "Plan Memory Phase 6",
  prompt: `Tu es l'Agent PLAN. Lis les instructions dans .claude/agents/plan.md
           et crée un plan d'implémentation pour backlog/memory-game.md Phase 6.`
})

// Puis après validation, lancer l'agent DEV
Task({
  subagent_type: "general-purpose",
  description: "Développe Memory Phase 6",
  prompt: `Tu es l'Agent DEV. Lis les instructions dans .claude/agents/dev.md
           et implémente selon le plan dans plans/memory-phase6-plan.md.`
})

// Et ainsi de suite...
```

---

## 📁 Structure des fichiers

```
.claude/agents/
├── README.md           # Ce fichier
├── plan.md            # Agent PLAN
├── dev.md             # Agent DEV
├── review.md          # Agent REVIEW
├── qa.md              # Agent QA
├── doc.md             # Agent DOC
└── deploy.md          # Agent DEPLOY
```

---

## ✅ Avantages du système

| Avantage | Description |
|----------|-------------|
| **Séparation des responsabilités** | Chaque agent a un rôle clair |
| **Qualité garantie** | Chaque étape est validée (review, tests) |
| **Traçabilité** | Chaque agent génère un rapport |
| **Flexibilité** | L'orchestrateur peut sauter des étapes si besoin |
| **Scalabilité** | Facile d'ajouter de nouveaux agents |
| **Automatisation** | L'orchestrateur gère tout le workflow |

---

## 🔧 Personnalisation

### Ajouter un nouvel agent

1. Créer un fichier `.claude/agents/nouvel-agent.md`
2. Documenter son rôle, input, output
3. Préciser quand il est appelé dans le workflow
4. Mettre à jour ce README

### Modifier un agent existant

1. Éditer le fichier `.claude/agents/[agent].md`
2. Les changements s'appliquent immédiatement
3. L'orchestrateur utilise toujours la dernière version

---

## 📊 Rapports générés

Chaque agent génère un rapport structuré :

| Agent | Format du rapport |
|-------|-------------------|
| PLAN | Plan d'implémentation (Markdown) |
| DEV | Résumé d'implémentation (Markdown) |
| REVIEW | Rapport de review (Markdown) |
| QA | Rapport de tests (Markdown) |
| DOC | Résumé de documentation (Markdown) |
| DEPLOY | Rapport de déploiement (Markdown) |

Ces rapports sont présentés à l'utilisateur pour validation/information.

---

## ⚡ Workflows prédéfinis

### Feature complète
```
PLAN → DEV → REVIEW → QA → DOC → DEPLOY (QUALIF) → DEPLOY (PROD)
```

### Hotfix urgent
```
DEV → QA → DEPLOY (PROD)
```

### Documentation seule
```
DOC
```

### Tests seuls
```
QA
```

L'orchestrateur décide automatiquement du workflow selon la demande de l'utilisateur.

---

## 🎯 Exemples d'utilisation

### Exemple 1 : Nouvelle feature

**Utilisateur** : "Implémente Memory Phase 6 - Mode CHACUN_SON_TOUR"

**Orchestrateur** :
1. Lance PLAN → Présente le plan
2. Utilisateur valide → Lance DEV
3. Lance REVIEW → Rapport OK
4. Lance QA → Tests PASS
5. Lance DOC → Docs mises à jour
6. Utilisateur : "Déploie en QUALIF" → Lance DEPLOY (QUALIF)

### Exemple 2 : Correction de bug

**Utilisateur** : "Corrige le bug de calcul de score en mode Memory"

**Orchestrateur** :
1. Lance DEV (avec le fix)
2. Lance QA (vérifier que c'est corrigé)
3. Lance DOC (CHANGELOG.md : version patch)
4. Lance DEPLOY (QUALIF puis PROD)

### Exemple 3 : Documentation uniquement

**Utilisateur** : "Mets à jour la documentation pour la feature Memory"

**Orchestrateur** :
1. Lance DOC uniquement
2. Présente le résumé

---

## 🛠️ Maintenance

### Vérifier qu'un agent fonctionne

L'orchestrateur peut tester un agent isolément :

```
"Lance l'agent QA sur le code actuel"
```

### Mettre à jour les instructions

Éditer directement le fichier `.md` de l'agent concerné.

---

## 📚 Références

- **CLAUDE.md** : Architecture complète du projet
- **docs/DEV_PROCEDURE.md** : Procédure de développement
- **docs/TEST_PROCEDURE.md** : Procédure de tests
- **docs/QUALIF_PROCEDURE.md** : Procédure de qualification
- **docs/RELEASE_PROCEDURE.md** : Procédure de release

---

## 🤝 Contribution

Pour améliorer un agent :
1. Identifier le problème (agent fait des erreurs, oublie des étapes, etc.)
2. Éditer le fichier `.md` de l'agent
3. Préciser les instructions manquantes
4. Tester avec un cas réel
5. Documenter les améliorations dans ce README

---

**Version** : 1.0.0
**Date** : 2026-01-22
**Auteur** : Équipe BuzzMaster
