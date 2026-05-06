# CDP_WORKFLOWS.md - Workflows Orchestres par CDP

Ce fichier centralise les patterns partages par les commandes `/feature`, `/bugfix`, `/hotfix`, et `/refactor`.

---

## 1. Contexte CDP

```yaml
Agent: CDP (Chef De Projet)
Role: Orchestrer workflows multi-agents avec validation utilisateur
```

---

## 2. Workflows Disponibles

| Commande | Type | Workflow | Version |
|----------|------|----------|---------|
| `/feature` | FEATURE | Complet | Incremente Y |
| `/bugfix` | BUGFIX | Simplifie | Incremente Z |
| `/hotfix` | HOTFIX | Accelere | Incremente Z + suffix |
| `/refactor` | REFACTOR | Leger | Aucun changement |

---

## 3. Workflow Standard CDP

```
[INIT] -> [CLARIFICATION] -> [PLAN/ANALYSE] -> [DEV] -> [REVIEW] -> [QA] -> [DOC] -> [DEPLOY] -> [FIN]
```

### Variantes par type

| Phase | FEATURE | BUGFIX | HOTFIX | REFACTOR |
|-------|---------|--------|--------|----------|
| Clarification | Oui | Oui | Non | Non |
| Backlog (GitHub Issues) | Oui | Si lié | Non | Non |
| Plan | Oui | Souvent | Non | Rarement |
| Dev | Complet | Cible | Minimal | Structure |
| Review | Oui | Oui | Rapide | Oui |
| QA | Complet | Regression | Critique | Complet |
| Doc | Oui | Si majeur | Post-mortem | Non |
| Deploy QUALIF | Oui | Oui | Optionnel | Oui |

---

## 4. Phase Clarification (FEATURE et BUGFIX uniquement)

### Objectif

Vérifier que la demande est suffisamment spécifiée **avant** de lancer le développement.
Si la spec est claire → passer directement à la phase suivante, sans poser de question.
Si des zones d'ombre existent → les lister et attendre la validation utilisateur.

### Algorithme

```
1. Lire $ARGUMENTS (description utilisateur)

2. Construire ISSUE_NUMS[] (issues à suivre tout au long du workflow) :
   a. Extraire les numéros explicites dans $ARGUMENTS (pattern #\d+) :
      pour chaque #N trouvé :
        gh issue view <N> --json number,title,body,labels,milestone
        → ajouter à ISSUE_NUMS[]
        → si l'issue est liée à un milestone → MILESTONE_NUM = ce milestone

   b. Rechercher les issues liées par mots-clés (si ISSUE_NUMS[] toujours vide) :
      gh issue list --search "<mots-clés>" --json number,title,body,labels,milestone
      → si résultats pertinents → ajouter les numéros à ISSUE_NUMS[]

3. Détecter et résoudre le milestone :
   a. Si "milestone:" ou un nom de milestone est mentionné dans $ARGUMENTS :
      gh api repos/{owner}/{repo}/milestones
      → identifier le milestone correspondant → MILESTONE_NUM

   b. Si MILESTONE_NUM détecté (via étape 2 ou 3a) :
      gh issue list --milestone "<title>" --state open \
        --json number,title,labels
      → ajouter toutes les issues ouvertes du milestone à ISSUE_NUMS[] (dédupliquer)

4. Persister dans workflow-state.json :
   issue_nums: ISSUE_NUMS[]
   milestone_num: MILESTONE_NUM (ou null si aucun)

5. Évaluer la complétude de la spec (critères ci-dessous)

6. Décision :
   |-- Spec complète → continuer sans interruption
   |-- Gaps détectés → afficher les questions, attendre réponse utilisateur
                    → puis continuer avec la spec enrichie
```

### Critères de complétude

#### Pour FEATURE
| Critère | Complet si… |
|---------|-------------|
| Comportement attendu | Le résultat final est décrit sans ambiguïté |
| Critères d'acceptance | Au moins un critère testable est identifiable |
| Scope | Les limites (ce qui est dedans / dehors) sont claires |
| Edge cases | Les cas limites évidents sont couverts ou explicitement exclus |
| Dépendances | Les APIs, services ou composants tiers sont identifiés |

#### Pour BUGFIX
| Critère | Complet si… |
|---------|-------------|
| Comportement actuel | Le symptôme est décrit (message d'erreur, comportement observé) |
| Comportement attendu | Ce qui devrait se passer est clair |
| Reproductibilité | Les étapes ou conditions de déclenchement sont connues |
| Scope | Le fix est délimité (pas de refactoring implicite attendu) |

### Format de sortie si questions nécessaires

Présenter directement à l'utilisateur :

```markdown
## Clarification requise avant de démarrer

J'ai analysé votre demande [+ issue #N si trouvée]. Avant de lancer le développement,
j'ai besoin de précisions sur les points suivants :

**1. [Titre du point ambigu]**
[Question ciblée et concise]

**2. [Titre du point ambigu]**
[Question ciblée et concise]

_Une fois ces points clarifiés, je lance immédiatement le workflow._
```

Attendre la réponse de l'utilisateur avant de continuer.

### Format de sortie si spec complète

```markdown
✓ Spec suffisamment claire — lancement du workflow.
```

Enchaîner directement sur la phase suivante sans autre attente.

---

## 5. Phases Communes

### Labels GitHub — Suivi de Phase

> **Condition** : s'applique si `ISSUE_NUMS[]` est non vide (construit en CLARIFICATION).
> **Règle** : chaque transition de phase met à jour le label de **toutes** les issues suivies. Un seul label actif à la fois par issue.

| Transition | Label à ajouter | Labels à retirer |
|------------|----------------|-----------------|
| Entrée Phase Plan (FEATURE) | `PLANNING` | `EN COURS`, `EN REVIEW`, `EN QA`, `DONE` |
| Entrée Phase Dev | `EN COURS` | `PLANNING`, `EN REVIEW`, `EN QA`, `DONE` |
| Retour Phase Dev (cycle REVIEW ou QA) | `EN COURS` | `EN REVIEW`, `EN QA` |
| Entrée Phase Review | `EN REVIEW` | `EN COURS`, `PLANNING`, `EN QA`, `DONE` |
| Entrée Phase QA | `EN QA` | `EN REVIEW`, `EN COURS`, `PLANNING`, `DONE` |
| QA VALIDATED | `DONE` | `EN QA`, `EN REVIEW`, `EN COURS`, `PLANNING` |
| Deploy PROD confirmé (GATE 4) | — (issues fermées) | — |

Appel MCP pour chaque transition — boucler sur toutes les issues suivies :
```
pour chaque issue_num dans ISSUE_NUMS[] :
  mcp__plugin_github_github__issue_write({
    owner: <owner>, repo: <repo>, issue_number: issue_num,
    labels: { add: ["<label>"], remove: ["<labels à retirer>"] }
  })
```

Deploy PROD confirmé — fermer toutes les issues suivies :
```
pour chaque issue_num dans ISSUE_NUMS[] :
  mcp__plugin_github_github__add_issue_comment({
    issue_number: issue_num,
    body: "✅ Livré — QA OK — documentation mise à jour"
  })
  mcp__plugin_github_github__issue_write({
    issue_number: issue_num, state: "closed"
  })
```

> **BUGFIX** : pas de phase Plan → pas de label `PLANNING`. Démarre directement à `EN COURS` lors de la Phase Dev.
> **HOTFIX / REFACTOR** : pas de gestion de labels (ISSUE_NUMS[] vide en règle générale).

### Milestone — Suivi de Complétion

> **Condition** : s'applique si `MILESTONE_NUM` est défini (construit en CLARIFICATION).
> **Nommage** : le titre du milestone est `vX.Y` (sans Z). Voir "Convention Milestone" ci-dessus.

| Moment | Action |
|--------|--------|
| Deploy PROD OK (tag `vX.Y.0`) | `gh issue list --milestone "<title>" --state open --json number,title` |
| Milestone à 100% (liste vide) | Fermer : `mcp__plugin_github_github__issue_write` (milestone state: closed) + informer l'utilisateur |
| Issues encore ouvertes | Alerter l'utilisateur avec la liste des issues restantes et leur label actuel |

---

### Phase Init (Git)

```bash
# FEATURE
git checkout main && git pull origin main
git checkout -b feature/<nom-court>

# BUGFIX
git checkout main && git pull origin main
git checkout -b bugfix/<nom-court>

# HOTFIX (depuis production)
git checkout main && git pull origin main
git checkout -b hotfix/<nom-court>

# REFACTOR
git checkout main && git pull origin main
git checkout -b refactor/<nom-court>
```

### Phase Versionnement

| Type | Action | Exemple |
|------|--------|---------|
| FEATURE | Incremente Y, reset Z | 2.40.3 -> 2.41.0 |
| BUGFIX | Incremente Z (build counter) | 2.41.0 -> 2.41.1 |
| HOTFIX | Incremente Z + suffix | 2.41.1 -> 2.41.2-hotfix |
| REFACTOR | Aucun | 2.41.1 (inchange) |

#### Convention Z — compteur de build / reset PROD

- **Z** est le compteur de build : incrémenté à chaque BUGFIX ou build intermédiaire durant le cycle.
- **Deploy PROD** : Z est **toujours remis à 0**. Le tag de release est `vX.Y.0`.
  - Exemple : si le cycle a produit `v2.41.1`, `v2.41.2`… le tag PROD est `v2.41.0`.

#### Convention Milestone — nommage `vX.Y`

Le titre du milestone correspond à la version cible **sans Z** : `vX.Y`.
- Exemple : milestone `v2.41` regroupe toutes les issues de la release Y=41, quel que soit Z.
- Le milestone se clôture lors du deploy PROD → tag `vX.Y.0`.

### Phase Plan

> Applicable : **FEATURE** (obligatoire) — **BUGFIX** (si complexe : plusieurs fichiers, risque de régression, changement d'architecture)

**→ Appliquer label `PLANNING`** sur toutes les issues de `ISSUE_NUMS[]` si non vide (FEATURE uniquement) :

```
pour chaque issue_num dans ISSUE_NUMS[] :
  mcp__plugin_github_github__issue_write({ owner: <owner>, repo: <repo>, issue_number: issue_num,
    labels: { add: ["PLANNING"], remove: ["EN COURS", "EN REVIEW", "EN QA", "DONE"] } })
```

> **Le CDP ne rédige jamais le plan lui-même.** C'est le rôle exclusif du planner.

```
SendMessage({ to: "planner", content: "
  Crée un plan d'implémentation pour : [description]
  Type : [FEATURE|BUGFIX]
  [FEATURE] Contrats API à créer dans contracts/ si nouveaux endpoints.
  [BUGFIX] Identifier la cause racine, le fix minimal, le scope impacté et le risque de régression.
  Retourne le plan structuré avec : tâches ordonnées, dépendances, risques.
" })
```

Recevoir le plan → lire intégralement, vérifier cohérence et complétude.
Lire `contracts/CHANGELOG.md` si FEATURE — signaler tout changement BREAKING lors du GATE 2.

**Présenter le plan validé à l'utilisateur et demander validation** ← GATE 2

---

### Phase Dev (Dispatch)

**→ Appliquer label `EN COURS`** sur toutes les issues de `ISSUE_NUMS[]` si non vide :

```
pour chaque issue_num dans ISSUE_NUMS[] :
  mcp__plugin_github_github__issue_write({ owner: <owner>, repo: <repo>, issue_number: issue_num,
    labels: { add: ["EN COURS"], remove: ["PLANNING", "EN REVIEW", "EN QA", "DONE"] } })
```

```
Analyser le scope :
|-- Backend seul -> dev-backend
|-- Frontend seul -> dev-frontend
|-- Les deux (dependants) -> dev-backend PUIS dev-frontend
|-- Les deux (independants) -> dev-backend ET dev-frontend (parallele)
```

### Phase Review

**→ Appliquer label `EN REVIEW`** sur toutes les issues de `ISSUE_NUMS[]` si non vide :

```
pour chaque issue_num dans ISSUE_NUMS[] :
  mcp__plugin_github_github__issue_write({ owner: <owner>, repo: <repo>, issue_number: issue_num,
    labels: { add: ["EN REVIEW"], remove: ["EN COURS", "PLANNING", "EN QA", "DONE"] } })
```

```
Lancer code-reviewer (+ test-writer en parallele)
|-- Recevoir DONE + ref fichier rapport
|-- CDP lit le rapport et valide la conformite
    |-- Non conforme -> renvoyer pour correction (hors cycle)
    |-- Conforme :
        |-- APPROVED            -> Phase QA
        |-- APPROVED WITH RESERVATIONS -> Phase QA (noter reserves)
        |-- REJECTED            -> Retour Phase Dev (cycle++) :
                                   mcp__plugin_github_github__issue_write( labels: add ["EN COURS"], remove ["EN REVIEW", "EN QA"] )
                                   relancer code-reviewer + test-writer
```

### Phase QA

**→ Appliquer label `EN QA`** sur toutes les issues de `ISSUE_NUMS[]` si non vide :

```
pour chaque issue_num dans ISSUE_NUMS[] :
  mcp__plugin_github_github__issue_write({ owner: <owner>, repo: <repo>, issue_number: issue_num,
    labels: { add: ["EN QA"], remove: ["EN REVIEW", "EN COURS", "PLANNING", "DONE"] } })
```

```
Lancer QA (avec ref scripts SHA + procedures test-writer)
|-- Recevoir DONE + ref fichier rapport
|-- CDP lit le rapport et valide la conformite
    |-- Non conforme -> renvoyer pour correction (hors cycle)
    |-- Conforme :
        |-- VALIDATED                   -> Phase Doc (automatique) :
                                           mcp__plugin_github_github__issue_write( labels: add ["DONE"], remove ["EN QA", "EN REVIEW", "EN COURS", "PLANNING"] )
        |-- VALIDATED WITH RESERVATIONS -> Phase Doc (noter reserves, continuer) :
                                           mcp__plugin_github_github__issue_write( labels: add ["DONE"], remove ["EN QA", "EN REVIEW", "EN COURS", "PLANNING"] )
        |-- NOT VALIDATED               -> Retour Phase Dev (cycle++) :
                                           mcp__plugin_github_github__issue_write( labels: add ["EN COURS"], remove ["EN QA", "EN REVIEW"] )
                                           relancer code-reviewer + test-writer

Si cycle > 3 -> ESCALADE utilisateur
```

---

## 6. Points de Validation Utilisateur

| Point | Conditions | Options |
|-------|------------|---------|
| Clarification | Gaps détectés dans la spec | Répondre aux questions / Continuer tel quel |
| Plan | Si creation plan | Valider / Modifier / Refuser |
| Escalade | 3 cycles atteints | Continuer / Abandonner |
| Deploy PROD | Toujours | Commande explicite `/deploy prod` |

---

## 7. Gestion des Erreurs CDP

| Situation | Action |
|-----------|--------|
| Issue GitHub non trouvee | Proposer creation via `gh issue create` ou continuer sans |
| Plan refuse | Demander modifications |
| Review rejetee | Retour DEV avec corrections |
| QA echoue | Retour DEV avec erreurs |
| Build echoue | Retour DEV avec erreur build |
| 3 cycles atteints | Escalade utilisateur |

---

## 8. Rapport Final CDP

```markdown
## Rapport de Workflow [TYPE]

**Informations**
- Type : [FEATURE|BUGFIX|HOTFIX|REFACTOR]
- Branche : [nom]
- Version : [X.Y.Z]
- Duree : [temps]
- Cycles : [nombre]

**Livrables**
- Code : [fichiers modifies]
- Tests : [ajoutes/modifies]
- Documentation : [mise a jour]

**Prochaines etapes**
- QUALIF deployee — voir scenarios de validation fournis ci-dessus
- `/deploy PROD` quand pret
```

---

## 9. Regles par Type

### FEATURE

- Scope large autorise
- Refactoring autorise
- Tests nouveaux requis
- Documentation complete
- QUALIF obligatoire

### BUGFIX

- Scope minimal obligatoire
- Pas de refactoring
- Test non-regression OBLIGATOIRE
- Doc si majeur
- QUALIF obligatoire

### HOTFIX

- Fix minimal UNIQUEMENT
- Pas de refactoring
- Test critique obligatoire
- QUALIF optionnel si urgent
- Post-mortem requis

### REFACTOR

- Comportement identique obligatoire
- Tests AVANT refactoring
- Incremental (petits changements)
- Pas de documentation
- QUALIF pour validation

---

## 10. Mots-Cles de Controle

Les commandes CDP reconnaissent des mots-cles speciaux pour interroger ou reprendre un workflow.

**Reference complete :** Voir `context/COMMON.md` section 12

### Handling des Mots-Cles

```
Reception $ARGUMENTS
    |
    |-- Premier mot = "help" ?
    |   |-- Afficher aide et mots-cles disponibles
    |
    |-- Premier mot = "status" ?
    |   |-- Afficher etat workflow actuel
    |
    |-- Premier mot = "plan" ?
    |   |-- Afficher plan sans executer
    |
    |-- Premier mot = "resume" ?
    |   |-- Extraire <phase>, valider, reprendre
    |
    |-- Premier mot = "skip" ?
    |   |-- Extraire <phase>, marquer skippee, continuer
    |
    |-- Premier mot = "jumpto" ?
    |   |-- Extraire <tache>, rechercher, positionner
    |
    |-- Sinon -> Workflow normal
```

### Etat Persistant

Pour supporter `status`/`resume`/`jumpto`, le CDP maintient un etat :

```yaml
workflow_state:
  type: FEATURE|BUGFIX|HOTFIX|REFACTOR
  description: "..."
  branch: feature/xxx
  current_phase: dev|review|qa|doc|deploy
  phase_status:
    init: completed
    plan: completed
    dev: in_progress
    review: pending
    qa: pending
    doc: pending
    deploy: pending
  tasks:
    - name: "Backend API"
      status: completed
    - name: "Frontend composant"
      status: in_progress
    - name: "Tests"
      status: pending
  cycles: 1
  started_at: "2025-01-15T10:00:00"
```

### Etat Global CDP

Pour la commande `/cdp`, l'orchestrateur maintient un etat global :

```yaml
cdp_state:
  active_workflow:
    type: FEATURE|BUGFIX|HOTFIX|REFACTOR
    description: "..."
    branch: feature/xxx
    current_phase: dev
  context_additions:
    - "Information supplementaire 1"
    - "Information supplementaire 2"
  notes:
    - "Note pour le rapport final"
  priority: normal|high|low
  paused: false
  config:
    max_cycles: 3
    auto_commit: false
    parallel_agents: true
  history:
    - {type: BUGFIX, description: "...", completed_at: "..."}
```

---

## 11. Commande /cdp

La commande `/cdp` permet le controle direct de l'orchestrateur :

| Mot-cle | Action |
|---------|--------|
| `help` | Aide sur /cdp |
| `status` | Vue globale (tous workflows) |
| `abort` | Abandonner workflow actuel |
| `pause` | Mettre en pause |
| `resume` | Reprendre apres pause |
| `context "..."` | Ajouter contexte aux sous-agents |
| `note "..."` | Ajouter note au rapport final |
| `priority <level>` | Changer priorite (high/normal/low) |
| `config` | Afficher configuration CDP |

**Difference cle** :
- `/feature status` -> etat du workflow FEATURE
- `/cdp status` -> vue globale de l'orchestrateur

---

## 12. État Persistant du Workflow

Le CDP maintient `.claude/workflow-state.json` mis à jour à chaque transition de phase.
Ce fichier est la source de vérité pour les commandes `status`, `resume`, `skip`, `jumpto`.

### Format

```json
{
  "type": "FEATURE",
  "description": "...",
  "branch": "feature/xxx",
  "version": "X.Y.Z",
  "started_at": "2026-04-26T14:30:00Z",
  "cycles": 1,
  "issue_nums": [123, 456],
  "milestone_num": 5,
  "phases": {
    "clarification":{ "status": "completed", "skipped": false },
    "plan":         { "status": "completed", "report": "_work/reports/plan-xxx.md", "timestamp": "..." },
    "dev-backend":  { "status": "completed", "sha": "abc123", "handoff": "_work/handoff/dev-backend-xxx.md" },
    "dev-frontend": { "status": "completed", "sha": "def456", "handoff": "_work/handoff/dev-frontend-xxx.md" },
    "test-writer":  { "status": "completed", "sha": "ghi789", "handoff": "_work/handoff/test-writer-xxx.md" },
    "review":       { "status": "completed", "report": "_work/reports/code-review-xxx.md" },
    "qa":           { "status": "in_progress", "report": null },
    "doc":          { "status": "pending" },
    "deploy-qualif":{ "status": "pending" },
    "deploy-prod":  { "status": "pending" }
  }
}
```

### Règles de mise à jour

| Moment | Action CDP |
|--------|------------|
| Démarrage workflow | Créer le fichier avec toutes les phases à `pending` |
| Dispatch d'un agent | Passer la phase à `in_progress` |
| Réception DONE conforme | Passer la phase à `completed`, enregistrer refs |
| Renvoi pour correction | Laisser `in_progress`, incrémenter `cycles` |
| Échec définitif | Passer à `failed`, noter la raison |

### Utilisation par les commandes de contrôle

- `status` : lire le fichier, afficher le tableau de phases
- `resume <phase>` : lire le fichier, reprendre à la phase indiquée
- `skip <phase>` : marquer la phase `skipped` et passer à la suivante
- `jumpto <tache>` : rechercher dans les phases et tâches, se positionner

---

## Usage

Dans les commandes CDP, referencer ce fichier :

```markdown
**Workflow CDP :** Voir `context/CDP_WORKFLOWS.md`
- Type : FEATURE|BUGFIX|HOTFIX|REFACTOR
- Phases : section 3
- Clarification : section 4
- Labels GitHub + Milestone : section 5 (Labels GitHub — Suivi de Phase)
- Phases communes : section 5
- Validation : section 6
- Erreurs : section 7
- Regles : section 9
- Mots-cles controle : section 10
- Commande /cdp : section 11
- Etat persistant : section 12
```
