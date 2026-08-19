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
| `/feature` | FEATURE | Complet | Rattache a un milestone (nouveau ou existant) — `X.Y.Z` fixe par son titre |
| `/bugfix` | BUGFIX | Simplifie | Milestone actif -> aucun changement (commits normaux) ; sinon -> milestone `X.Y.Z+1` cree automatiquement |
| `/hotfix` | HOTFIX | Accelere | Milestone `X.Y.Z+1` cree automatiquement si absent, meme si un autre milestone est en cours |
| `/refactor` | REFACTOR | Leger | Rattache au milestone actif — aucun changement de version (`a` gere par `deploy` au prochain deploiement QUALIF) |

---

## 2.bis Dispatch d'un Teammate

Tous les teammates sont en IDLE depuis `/start-session`. Dispatch = uniquement `SendMessage`.

```
SendMessage({ to: "<nom>", content: "<tâche complète>" })
→ Attendre ACTIF + DONE
```

Plusieurs en parallèle — même message :
```
SendMessage({ to: "dev-backend",  content: "<tâche>" })
SendMessage({ to: "dev-frontend", content: "<tâche>" })
```

### Agents par phase

| Phase | Agent(s) à dispatcher |
|-------|----------------------|
| Plan | `planner` |
| Dev — Backend seul | `dev-backend` |
| Dev — Frontend seul | `dev-frontend` |
| Dev — Les deux | `dev-backend` + `dev-frontend` (parallèle si indépendants) |
| Review | `code-reviewer` + `test-writer` (parallèle) |
| QA | `qa` |
| Doc | `doc-updater` |
| Deploy QUALIF / PROD | `deployer` |

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

   c. Si MILESTONE_NUM toujours indéfini (BUGFIX/HOTFIX sans milestone actif) :
      → créer automatiquement le milestone cible `X.Y.Z+1` (Z+1 sur la dernière version prod livrée)
        en suivant la logique de `/milestone new` (`commands/milestone.md` Mode NEW,
        `context/COMMON.md` section 5.7) → MILESTONE_NUM
      → tout développement doit être rattaché à un milestone, aucun cycle hors milestone (section 5.4)

4. Persister dans workflow-state.json :
   issue_nums: ISSUE_NUMS[]
   milestone_num: MILESTONE_NUM (obligatoire, jamais null)

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

> **Condition** : s'applique toujours — `MILESTONE_NUM` est désormais obligatoire (construit ou créé automatiquement en CLARIFICATION, voir "Convention Milestone" ci-dessous).
> **Nommage** : le titre du milestone est `vX.Y.Z` complet, optionnellement suivi de `" — <nom>"`. Voir "Convention Milestone" ci-dessous.

| Moment | Action |
|--------|--------|
| Deploy PROD OK (tag `vX.Y.Z`) | `gh issue list --milestone "<title>" --state open --json number,title` |
| Milestone à 100% (liste vide) | Fermer : `mcp__plugin_github_github__issue_write` (milestone state: closed) + consigner la version livrée dans la description (écriture ponctuelle, `COMMON.md` section 5.7) + informer l'utilisateur |
| Issues encore ouvertes | Alerter l'utilisateur avec la liste des issues restantes et leur label actuel |

---

### Phase Init (Git)

```bash
# FEATURE / BUGFIX / HOTFIX / REFACTOR — meme sequence, prefixe de branche different
git checkout main && git pull origin main
git checkout -b <feature|bugfix|hotfix|refactor>/<nom-court>

# Ecriture de la version du milestone (X.Y.Z fixe par son titre, voir Phase Versionnement
# ci-dessous) — a la charge de CDP, avant tout commit DEV :
echo "X.Y.Z.0" > {VERSION_FILE}   # adapter selon le format reel de {VERSION_FILE}
git add {VERSION_FILE}
git commit -m "chore(version): Start vX.Y.Z.0 - <nom-court>"
git push -u origin <feature|bugfix|hotfix|refactor>/<nom-court>
```

### Phase Versionnement

**Référence complète** : `context/COMMON.md` section 5 (format `X.Y.Z.a`, cycle de vie de la version).

Le milestone GitHub actif est la **seule source de vérité** pour `X.Y.Z` — plus aucun agent ne recalcule ces segments. Tout workflow est rattaché à un milestone.

| Type | Condition | Action | Exemple |
|------|-----------|--------|---------|
| FEATURE | Toujours | Rattaché à un milestone (nouveau ou existant, `X.Y.Z` fixé par son titre) | milestone `v1.4.0` → dev `1.4.0.0` → prod `1.4.0` |
| BUGFIX | Milestone actif | Intégré aux itérations du milestone : commit normal, sans toucher `{VERSION_FILE}`. `a` s'incrémente au prochain deploiement QUALIF, à la charge de `deploy` | milestone `v1.4.0` → dev `1.4.0.0` (fix inclus) → prod `1.4.0` |
| BUGFIX | Aucun milestone actif | Milestone `X.Y.Z+1` créé automatiquement (Z+1 sur la dernière prod livrée) | dernière prod `1.4.0` → milestone `v1.4.1` créé → dev `1.4.1.0` → prod `1.4.1` |
| HOTFIX | Toujours (urgence prod) | Milestone `X.Y.Z+1` créé automatiquement si absent, même si un autre milestone est en cours | dernière prod `1.4.0` → milestone `v1.4.1` créé → dev `1.4.1.0` → prod `1.4.1` |
| REFACTOR | — | Rattaché au milestone actif — aucun changement de version, `a` reste sous la seule responsabilité de `deploy` | `1.4.0.x` (inchangé en prod) |

#### Règle — bug remonté sur une ancienne version prod

Une version prod déjà remplacée n'est **jamais repatchée**. Le fix cible toujours la ligne prod courante, quelle que soit l'ancienneté du bug (voir `context/COMMON.md` section 5.4).

#### Convention Milestone GitHub — nommage `vX.Y.Z` (+ nom optionnel)

Le titre du milestone GitHub porte la version complète `X.Y.Z` fixée dès sa création, optionnellement
suivie de `" — <nom>"` (ex: `v1.4.0 — Authentification OAuth2`) — voir `COMMON.md` section 5.7 pour la
logique complète de validation (X/Y obligatoires, Z auto-complété, unicité, cohérence labels↔version).
Toute recherche/comparaison de milestone par version porte sur le **préfixe** `vX.Y.Z`, jamais sur le
titre entier (le nom ne doit jamais empêcher un matching).
- Un cycle FEATURE crée/utilise le milestone GitHub `vX.Y.Z[ — nom]`, clôturé au deploy PROD (tag `vX.Y.Z` identique).
- Un cycle BUGFIX/HOTFIX sans milestone actif crée automatiquement le milestone `vX.Y.(Z+1)` (sans nom, création non-interactive) — plus de cycle "hors milestone" qui patcherait silencieusement une livraison.
- `deploy.template.md` lit `X.Y.Z` directement depuis `{VERSION_FILE}` à la promotion (déjà fixé par le milestone à l'ouverture du cycle) — aucun recalcul, et matche le milestone à clôturer par préfixe (voir `deploy.template.md` Étape 7).

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
Analyser le scope, puis dispatcher les agents concernés via SendMessage :

|-- Backend seul ->
|     SendMessage({ to: "dev-backend", content: "
|       [description de la tâche]
|       Handoff planner : _work/handoff/planner-[timestamp].md
|     " })
|
|-- Frontend seul ->
|     SendMessage({ to: "dev-frontend", content: "
|       [description de la tâche]
|       Handoff planner : _work/handoff/planner-[timestamp].md
|     " })
|
|-- Les deux (dépendants) ->
|     SendMessage dev-backend → attendre DONE →
|     SendMessage dev-frontend :
|       Handoff planner     : _work/handoff/planner-[timestamp].md
|       Handoff dev-backend : _work/handoff/dev-backend-[timestamp].md
|
|-- Les deux (indépendants) ->
|     SendMessage dev-backend + dev-frontend dans le même tour
|     Attendre les deux DONE
```

### Phase Review

**→ Appliquer label `EN REVIEW`** sur toutes les issues de `ISSUE_NUMS[]` si non vide :

```
pour chaque issue_num dans ISSUE_NUMS[] :
  mcp__plugin_github_github__issue_write({ owner: <owner>, repo: <repo>, issue_number: issue_num,
    labels: { add: ["EN REVIEW"], remove: ["EN COURS", "PLANNING", "EN QA", "DONE"] } })
```

```
// Dispatcher code-reviewer + test-writer en parallèle (même tour)
SendMessage({ to: "code-reviewer", content: "
  Revue du code sur [branche].
  Handoff dev-backend  : _work/handoff/dev-backend-[timestamp].md  (si applicable)
  Handoff dev-frontend : _work/handoff/dev-frontend-[timestamp].md (si applicable)
  Retourne : APPROVED / APPROVED WITH RESERVATIONS / REJECTED + rapport.
" })
SendMessage({ to: "test-writer", content: "
  Écris les tests pour : [description]
  SHA dev : [sha]
  Handoff planner      : _work/handoff/planner-[timestamp].md       (si applicable)
  Handoff dev-backend  : _work/handoff/dev-backend-[timestamp].md   (si applicable)
  Handoff dev-frontend : _work/handoff/dev-frontend-[timestamp].md  (si applicable)
  Contrats API : contracts/ — valider la conformité aux contrats.
  Produire : scripts de tests + procédures manuelles tests/procedures/.
" })

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
// Dispatcher qa
SendMessage({ to: "qa", content: "
  Execute les tests sur [branche].
  Scripts de tests : SHA [sha] (commités par test-writer).
  Procédures manuelles : tests/procedures/[feature].md
  Rapport code-reviewer : _work/reports/code-reviewer-[timestamp].md
  Retourne : VALIDATED / NOT VALIDATED + rapport.
" })

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

### Phase Doc

> Applicable : **FEATURE** (obligatoire) — **BUGFIX** (si majeur) — **HOTFIX** (post-mortem) — **REFACTOR** (non)

```
// Dispatcher doc-updater
SendMessage({ to: "doc-updater", content: "
  Mets à jour la documentation pour : [description]
  Type : [FEATURE|BUGFIX|HOTFIX]
  SHA dev : [sha]
  Handoff dev : _work/handoff/dev-[timestamp].md
  Handoff QA  : _work/handoff/qa-[timestamp].md
  Fichiers modifiés : [liste]
" })

|-- Recevoir DONE + ref handoff
|-- CDP valide la conformité
    |-- Non conforme -> renvoyer pour correction
    |-- Conforme     -> Phase Deploy QUALIF
```

### Phase Deploy QUALIF

```
// Dispatcher deployer
SendMessage({ to: "deployer", content: "
  Déploie en QUALIF.
  Branche : [branche]
  Version : [X.Y.Z]
  Handoff doc : _work/handoff/doc-updater-[timestamp].md
" })

|-- Recevoir DONE + rapport de déploiement
|-- CDP informe l'utilisateur : QUALIF déployée, scénarios de validation fournis
|-- Deploy PROD : déclenché uniquement par commande explicite `/deploy prod`
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
