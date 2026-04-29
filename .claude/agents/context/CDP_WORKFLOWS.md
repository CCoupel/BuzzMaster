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

2. Rechercher les issues GitHub liées :
   gh issue list --search "<mots-clés>" --json number,title,body,labels,milestone
   Si numéro d'issue dans $ARGUMENTS → gh issue view <N> --json body,comments

3. Vérifier le milestone si mentionné :
   gh api repos/{owner}/{repo}/milestones

4. Évaluer la complétude de la spec (critères ci-dessous)

5. Décision :
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

### Format de sortie si spec complète

```markdown
✓ Spec suffisamment claire — lancement du workflow.
```
(Puis enchaîner directement sur la phase suivante, sans autre commentaire.)

---

## 5. Phases Communes

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
| BUGFIX | Incremente Z | 2.40.0 -> 2.40.1 |
| HOTFIX | Incremente Z + suffix | 2.40.1 -> 2.40.2-hotfix |
| REFACTOR | Aucun | 2.40.1 (inchange) |

### Phase Dev (Dispatch)

```
Analyser le scope :
|-- Backend seul -> dev-backend
|-- Frontend seul -> dev-frontend
|-- Les deux (dependants) -> dev-backend PUIS dev-frontend
|-- Les deux (independants) -> dev-backend ET dev-frontend (parallele)
```

### Phase Review

```
Lancer code-reviewer (+ test-writer en parallele)
|-- Recevoir DONE + ref fichier rapport
|-- CDP lit le rapport et valide la conformite
    |-- Non conforme -> renvoyer pour correction (hors cycle)
    |-- Conforme :
        |-- APPROVED            -> Phase QA
        |-- APPROVED WITH RESERVATIONS -> Phase QA (noter reserves)
        |-- REJECTED            -> Retour Phase Dev (cycle++)
                                   relancer code-reviewer + test-writer
```

### Phase QA

```
Lancer QA (avec ref scripts SHA + procedures test-writer)
|-- Recevoir DONE + ref fichier rapport
|-- CDP lit le rapport et valide la conformite
    |-- Non conforme -> renvoyer pour correction (hors cycle)
    |-- Conforme :
        |-- VALIDATED                   -> Phase Doc (automatique)
        |-- VALIDATED WITH RESERVATIONS -> Phase Doc (noter reserves, continuer)
        |-- NOT VALIDATED               -> Retour Phase Dev (cycle++)
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
  "phases": {
    "clarification":{ "status": "completed", "skipped": false },
    "plan":         { "status": "completed", "report": ".claude/reports/plan-xxx.md", "timestamp": "..." },
    "dev-backend":  { "status": "completed", "sha": "abc123", "handoff": ".claude/handoff/dev-backend-xxx.md" },
    "dev-frontend": { "status": "completed", "sha": "def456", "handoff": ".claude/handoff/dev-frontend-xxx.md" },
    "test-writer":  { "status": "completed", "sha": "ghi789", "handoff": ".claude/handoff/test-writer-xxx.md" },
    "review":       { "status": "completed", "report": ".claude/reports/code-review-xxx.md" },
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
- Validation : section 6
- Erreurs : section 7
- Regles : section 9
- Mots-cles controle : section 10
- Commande /cdp : section 11
- Etat persistant : section 12
```
