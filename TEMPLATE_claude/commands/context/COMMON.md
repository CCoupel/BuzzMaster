# COMMON.md - Patterns et Commandes Partages

Ce fichier centralise les elements repetes dans les definitions de commandes et agents Claude Code. Les agents et commandes doivent referencer ce fichier plutot que de dupliquer ces informations.

> **Objectif** : Eliminer la duplication a travers les commandes et agents.

---

## 1. Contexte Projet

**A utiliser dans tous les agents et commandes au lieu de repeter ces informations.**

```yaml
Projet: {PROJECT_NAME}
Repository: {REPO_URL}

Structure:
  Source: {SRC_DIR}
  Config version: {VERSION_FILE}
  Backlog: GitHub Issues (gh issue list)
  Documentation: docs/

Branches:
  Production: main
  Features: feature/<nom-court>
  Bugfixes: fix/<nom-court>
```

---

## 2. Commandes de Build

### 2.1 Build Complet

```bash
{BUILD_CMD}
```

### 2.2 Validation du Build

```bash
{BUILD_VALIDATE_CMD}
```

---

## 3. Controle du Serveur

### 3.1 Arret Gracieux

> **REGLE** : Toujours utiliser la methode d'arret prevue, jamais de kill force.

```bash
{SERVER_STOP_CMD}
```

### 3.2 Sequence Redemarrage Complete

```bash
{SERVER_RESTART_CMD}
```

### 3.3 Verification Post-Demarrage

```bash
{SERVER_VERIFY_CMD}
```

---

## 4. Commandes de Test

### 4.1 Tests Unitaires

```bash
{TEST_CMD}
```

### 4.2 Rapport de Couverture

```bash
{COVERAGE_CMD}
```

---

## 5. Gestion des Versions

### 5.1 Fichiers de Version

| Fichier | Champ | Usage |
|---------|-------|-------|
| `{VERSION_FILE}` | `"version"` | Source de verite |

### 5.2 Format et Regles de Versionnement

```
Format dev  : X.Y.Z.a
Format prod : X.Y.Z   (le "a" n'est jamais publie en prod)
```

| Segment | Role |
|---------|------|
| X | Compatibilite des donnees (DB, fichiers). Rupture = upgrade possible mais rollback complique. |
| Y | Compteur de milestone/livraison. Impair = dev, pair = prod. Avance toujours de +1 (jamais +2). |
| Z | Compteur de bugfix. Remis a 0 uniquement au demarrage d'un nouveau milestone. |
| a | Iteration dev interne (un commit = un `a`). Jamais visible en prod. |

### 5.3 Les 5 Operations

| Evenement | Effet |
|-----------|-------|
| Iteration dev (commit courant) | `a+1` |
| Nouveau milestone (feature planifiee) | `Y+1 ; Z=0 ; a=0` |
| Nouveau cycle bugfix (aucun milestone actif) | `Z+1 ; a=0` (reprend le Y de dev de la derniere wave) |
| Promotion dev -> prod | `Y+1 ; Z conserve ; a supprime` |
| Rupture de compatibilite de donnees | `X+1 ; Y=0 ; Z=0 ; a=0` (le prochain milestone repart au Y impair suivant, donc `Y=1`) |

### 5.4 Regle d'Or — Bug Remonte

- **Milestone en cours** : le fix est integre aux iterations du milestone (`a+1`, `Z` ne bouge pas). La prochaine prod livre le Y du milestone.
- **Aucun milestone en cours** : nouveau cycle bugfix cree a partir du dernier Y de dev (meme wave que la prod courante), `Z+1`.
- **Une version prod deja depassee n'est jamais repatchee** — le fix cible toujours la ligne prod courante, jamais une ancienne.

### 5.5 Exemple

```
1.1.0.0 -> 1.1.0.1 -> 1.1.0.2        (dev, Milestone A)
1.2.0                                 (promotion prod)
1.3.0.0 -> 1.3.0.1                    (dev, Milestone B)
1.4.0                                 (promotion prod)
1.3.1.0 -> 1.3.1.1                    (bug remonte, aucun milestone actif — reprise Y=3)
1.4.1                                 (promotion prod, Z conserve)
1.5.0.0                               (nouveau milestone, Z revient a 0)
1.6.0                                 (promotion prod)
```

### 5.6 Lire la Version

```bash
{VERSION_READ_CMD}
```

---

## 6. Operations Git

### 6.1 Creation de Branche Feature

```bash
git checkout main
git pull origin main
git checkout -b feature/<nom-court>
# Nouveau milestone dans {VERSION_FILE} : Y+1 ; Z=0 ; a=0
git add {VERSION_FILE}
git commit -m "chore(version): Start vX.Y.0.0 - <feature name>"
git push -u origin feature/<nom-court>
```

### 6.2 Commit Atomique (Style)

```bash
# Format du message
<type>(<scope>): <description courte>

# Types valides
feat:     Nouvelle fonctionnalite
fix:      Correction de bug
docs:     Documentation uniquement
chore:    Maintenance, config
refactor: Refactoring sans changement fonctionnel
test:     Ajout/modification de tests
style:    Formatage, pas de changement de code
perf:     Amelioration de performance
```

### 6.3 Squash Merge (PROD)

```bash
git checkout main
git pull origin main
git merge --squash feature/<branch>
git commit -m "feat: <description> (v<version>)"
git push origin main
```

### 6.4 Tag et Release

```bash
# Creer le tag annote
git tag -a v<version> -m "Release v<version> - <description>"
git push origin v<version>
```

---

## 7. Checklists Communes

### 7.1 Checklist Fin de Session DEV

```markdown
- [ ] Code compile sans erreur
- [ ] Tests unitaires passes
- [ ] Version incrementee (a pour chaque commit ; Z uniquement au 1er commit d'un cycle bugfix sans milestone actif)
- [ ] Commits atomiques avec messages clairs
- [ ] Pas de fichiers temporaires
- [ ] Push effectue
```

### 7.2 Checklist Pre-QUALIF

```markdown
- [ ] Build complet reussi
- [ ] Tests 100% passes (0 FAIL)
- [ ] Serveur redemarre et operationnel
- [ ] Version correspond au fichier de config
```

### 7.3 Checklist Pre-PROD

```markdown
- [ ] QUALIF validee
- [ ] Review code approuvee
- [ ] CHANGELOG.md mis a jour
- [ ] Documentation mis a jour (si nouvelles features)
- [ ] Version promue (Y+1 ; Z conserve ; a supprime — voir section 5.3)
- [ ] Build reussi
```

### 7.4 Checklist Post-PROD

```markdown
- [ ] Squash merge vers main effectue
- [ ] Tag Git cree et pushe
- [ ] Release creee avec artefacts
- [ ] Branche feature conservee (rollback)
```

---

## 8. Nettoyage

### 8.1 Fichiers Temporaires a Supprimer

```bash
# Fichiers de developpement
rm -f *.bak test-report.txt test-summary.txt
# Fichiers de couverture
rm -f coverage.out coverage.html
```

---

## 9. Patterns de Workflow

### 9.1 Workflow Feature

```
/feature -> CLARIFICATION -> PLAN -> DEV -> REVIEW -> QA -> DOC -> DEPLOY(QUALIF) -> DEPLOY(PROD)
```

### 9.2 Workflow Bugfix

```
/bugfix -> CLARIFICATION -> ANALYSE -> DEV -> REVIEW -> QA -> DEPLOY(QUALIF)
```

### 9.3 Workflow Hotfix (Urgence)

```
/hotfix -> DEV -> QA -> DEPLOY(PROD)
```

---

## 10. Dispatch Automatique

### 10.1 Criteres de Routage

Le routage vers les agents DEV se fait selon les fichiers impactes et le type de modification. Chaque projet definit ses propres criteres dans `context/PROJECT_CONTEXT.md`.

### 10.2 Ordre d'Execution

- **Sequentiel** (Backend -> Frontend) : Si nouvelles APIs, modeles, ou protocoles
- **Parallele** : Si modifications isolees sans dependances

---

## 11. Reference Rapide

### Fichiers Cles

| Fichier | Role |
|---------|------|
| `{VERSION_FILE}` | Version (source de verite) |
| `CHANGELOG.md` | Historique des versions |
| `CLAUDE.md` | Documentation projet |

---

## 12. Mots-Cles Reserves (Controle de Workflow)

Les commandes CDP (`/feature`, `/bugfix`, `/hotfix`, `/refactor`) reconnaissent des mots-cles speciaux pour interroger ou reprendre un workflow.

### 12.1 Mots-Cles Disponibles

| Mot-cle | Description | Exemple |
|---------|-------------|---------|
| `help` | Affiche l'aide et les mots-cles disponibles | `/feature help` |
| `status` | Affiche l'etat actuel du workflow | `/feature status` |
| `plan` | Affiche le plan sans executer | `/feature plan` |
| `resume <phase>` | Reprend a une phase specifique | `/feature resume qa` |
| `skip <phase>` | Saute une phase | `/feature skip review` |
| `jumpto <tache>` | Demarre a une tache precise du plan | `/feature jumpto "Creer endpoint API"` |

### 12.2 Phases Valides pour resume/skip

```
init -> clarification -> plan -> dev -> review -> qa -> doc -> deploy
```

### 12.3 Comportement par Mot-Cle

**`help`** :
```markdown
## /[commande] - Aide

**Description** : [Description du workflow]

**Usage** :
  /[commande] <description>           Lancer le workflow
  /[commande] help                    Afficher cette aide
  /[commande] status                  Etat du workflow en cours
  /[commande] plan                    Afficher le plan
  /[commande] resume <phase>          Reprendre a une phase
  /[commande] skip <phase>            Sauter une phase
  /[commande] jumpto <tache>          Aller a une tache precise

**Phases** : init -> clarification -> plan -> dev -> review -> qa -> doc -> deploy
```

**`status`** :
```markdown
## Etat du Workflow

**Type** : [TYPE]
**Phase actuelle** : [PHASE] ([N]/[Total])
**Taches** : [N]/[Total] completees
**Prochaine etape** : [Description]
```

**`plan`** :
```markdown
## Plan d'Implementation

- [x] Phase 1 : Init (branche creee)
- [x] Phase 2 : Plan valide
- [ ] Phase 3 : DEV <- en cours
- [ ] Phase 4 : REVIEW
- [ ] Phase 5 : QA
- [ ] Phase 6 : DOC
- [ ] Phase 7 : DEPLOY
```

**`resume <phase>`** :
- Verifie que les phases precedentes sont completes
- Si non, propose de completer ou forcer
- Reprend l'execution a la phase specifiee

**`skip <phase>`** :
- Marque la phase comme "skippee"
- Continue a la phase suivante
- Note dans le rapport final

**`jumpto <tache>`** :
- Recherche la tache par nom (fuzzy match)
- Positionne le workflow a cette tache
- Affiche contexte pour confirmation

### 12.4 Detection Automatique

Le premier mot de `$ARGUMENTS` est verifie contre cette liste. Si match :
- Extraire le mot-cle et les parametres
- Executer l'action correspondante
- Ne PAS lancer le workflow normal

```
$ARGUMENTS = "help"             -> Action: afficher aide commande
$ARGUMENTS = "status"           -> Action: afficher etat
$ARGUMENTS = "resume dev"       -> Action: reprendre a DEV
$ARGUMENTS = "jumpto API test"  -> Action: chercher tache "API test"
$ARGUMENTS = "Ajouter mode X"  -> Action: workflow normal (pas de mot-cle)
```

---

## 13. Adaptations Projet

Chaque commande `xxx.md` est gérée par le template — ne pas l'éditer directement (elle sera écrasée à la prochaine sync).
Pour personnaliser le comportement d'une commande, écrire dans les fichiers `context/` qu'elle référence (ex: `context/COMMON.md`).
Pour les agents, `xxx.template.md` (template) peut avoir un fichier compagnon `xxx.md` pour les adaptations projet.

---

## Usage

**Dans les commandes et agents**, au lieu de repeter le contexte projet :

```markdown
# Avant (repete N fois)
**Contexte projet :**
- Repertoire : ...
- Source : ...
- Config version : ...

# Apres (reference unique)
**Contexte projet :** Voir `context/COMMON.md` section 1
**Build :** Voir `context/COMMON.md` section 2
**Tests :** Voir `context/COMMON.md` section 4
```
