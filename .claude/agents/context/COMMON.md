# Règles Communes à Tous les Agents

> **Ce fichier contient les règles obligatoires pour TOUS les agents BuzzMaster.**
> Chaque agent doit référencer ce fichier : `@import COMMON.md`
>
> **Contexte projet** : Voir `context/PROJECT_CONTEXT.md` pour le stack technique, la structure et les commandes.

---

## Gestion de la Todo List (OBLIGATOIRE)

Vous DEVEZ utiliser le tool `TodoWrite` pour suivre votre progression de manière visible.

### Au Démarrage

1. **Créer une todo list** avec toutes les étapes de votre travail
2. Chaque tâche doit avoir :
   - `content` : Description de la tâche (forme impérative)
   - `status` : `pending` (ou `in_progress` pour la première)
   - `activeForm` : Description en forme progressive (anglais)

### Structure d'une Todo List

```json
[
  {"content": "Description en français", "status": "in_progress", "activeForm": "English description"},
  {"content": "Deuxième tâche", "status": "pending", "activeForm": "Second task"},
  ...
]
```

### Affichage Attendu

Pendant l'exécution, afficher visuellement la progression :

```
📍 Tâche [N]/[Total] : [Description]
   └── [Détail de ce qui est en cours...]

✅ Tâche [N]/[Total] terminée
   └── [Résumé du résultat]
```

### Règles Strictes

| Règle | Description |
|-------|-------------|
| **Une seule tâche `in_progress`** | Jamais plus d'une tâche en cours à la fois |
| **Mise à jour immédiate** | Mettre à jour la todo list après CHAQUE changement |
| **Affichage visuel** | Toujours afficher la progression à l'utilisateur |
| **Jamais continuer sans MAJ** | Ne jamais passer à la tâche suivante sans avoir mis à jour le statut |

---

## Notifications de Progression (OBLIGATOIRE)

### Au Démarrage de la Tâche

Vous DEVEZ afficher immédiatement un message de démarrage avec ce format :

```
🚀 **[NOM-AGENT] DÉMARRÉ**
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📋 [Champ principal] : [Valeur]
📦 [Champ secondaire] : [Valeur]
🎯 [Champ tertiaire] : [Valeur]
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

### À la Fin de la Tâche

**En cas de succès :**
```
✅ **[NOM-AGENT] TERMINÉ**
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📋 [Champ principal] : [Valeur]
📝 [Résultat 1] : [Valeur]
📊 [Résultat 2] : [Valeur]
✅ [Statut final] : [OK / Prêt pour X]
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

**En cas d'erreur :**
```
❌ **[NOM-AGENT] ERREUR**
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📋 [Champ principal] : [Valeur]
❌ Problème : [Description]
🔧 Action requise : [Solution proposée]
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

### Notifications Intermédiaires (optionnel selon agent)

Pour les workflows longs, notifier entre chaque phase majeure :
```
📍 **[NOM-AGENT] - Phase [N]/[Total] : [Nom de la phase]**
   └── [Ce qui va être fait]
```

---

## Règles de Communication

### Langue

- **Français** : Préféré pour les messages utilisateur et la documentation
- **Anglais** : Pour le code, les commits, et les champs techniques (`activeForm`)

### Format des Messages

- Utiliser les emojis de manière cohérente (voir tableau ci-dessous)
- Utiliser les box drawing characters (`━`) pour les bordures
- Garder les messages concis mais informatifs

### Emojis Standards

| Emoji | Signification |
|-------|---------------|
| 🚀 | Démarrage |
| ✅ | Succès / Terminé |
| ❌ | Erreur / Échec |
| ⚠️ | Avertissement / Réserves |
| 📍 | Tâche en cours |
| 📋 | Information principale |
| 📝 | Fichiers / Documentation |
| 📦 | Version / Package |
| 📊 | Statistiques / Métriques |
| 🎯 | Objectif / Cible |
| 🔧 | Action requise / Fix |
| 🌿 | Branche Git |
| 📤 | Push / Export |
| 🧪 | Tests |
| 💾 | Sauvegarde / Mémoire |

---

## Gestion des Erreurs

### Comportement Attendu

1. **Documenter** l'erreur dans le rapport/summary
2. **Proposer** une solution si possible
3. **Signaler** au CDP/orchestrateur pour décision
4. **Ne jamais rester bloqué en silence**

### Format de Signalement

```markdown
## ⚠️ Problème Rencontré

**Type** : [Build / Test / Git / Autre]
**Description** : [Détail du problème]
**Impact** : [Critique / Important / Mineur]
**Solution proposée** : [Si applicable]
```

---

## Coordination Inter-Agents

### Workflow Standard BuzzMaster

```
PLAN → DEV → TEST-WRITER → REVIEW → QA → DOC → DEPLOY
```

### Transmission de Contexte

Chaque agent doit :
1. Lire le **résumé de l'agent précédent**
2. Produire un **résumé structuré** pour l'agent suivant
3. Documenter les **décisions prises** et **problèmes rencontrés**

### Points de Validation

| Agent | Validation produite | Destinataire |
|-------|---------------------|--------------|
| PLAN | Plan d'implémentation | DEV |
| DEV | Summary + commits | TEST-WRITER, REVIEW |
| TEST-WRITER | Fichiers de tests | REVIEW, QA |
| REVIEW | Review Report (APPROVED/REJECTED) | QA ou DEV |
| QA | QA Report (VALIDATED/NOT VALIDATED) | DOC ou DEV |
| DOC | Documentation finalisée | DEPLOY |
| DEPLOY | Deployment Report | Utilisateur |

---

## Références Projet BuzzMaster

> **Détails complets** : Voir `context/PROJECT_CONTEXT.md`

### Fichiers Essentiels

| Fichier | Description |
|---------|-------------|
| `CLAUDE.md` | Architecture complète |
| `CHANGELOG.md` | Historique des versions |
| `server-go/config.json` | Version actuelle |
| `contracts/*.md` | Contrats API |

### Build Order (CRITIQUE)

**TOUJOURS rebuilder le frontend AVANT le backend Go.**

```bash
cd server-go/web && npm run build && cd .. && go build -o server.exe ./cmd/server
```

### Contrainte TV (CRITIQUE)

**L'affichage TV (`/tv`) est STATIQUE - pas de scroll autorisé.**

```css
.container { height: 100vh; overflow: hidden; }
```
