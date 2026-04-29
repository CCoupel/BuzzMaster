# Commande /start-session

Demarrage de session : lecture de la memoire projet et creation de la team de travail.

## Argument recu

$ARGUMENTS

## Instructions

### Etape 1 — Verification des mises a jour du template

**Premiere action, avant tout le reste.**

Si `TEMPLATE_claude/.template-source.json` existe :

```bash
TEMPLATE_REPO=$(cat TEMPLATE_claude/.template-source.json | jq -r '.repo')
TEMPLATE_BRANCH=$(cat TEMPLATE_claude/.template-source.json | jq -r '.branch')
KNOWN_COMMIT=$(cat TEMPLATE_claude/.template-source.json | jq -r '.commit // ""')
SYNCED_AT=$(cat TEMPLATE_claude/.template-source.json | jq -r '.synced_at // ""')

LATEST_COMMIT=$(gh api repos/$TEMPLATE_REPO/commits/$TEMPLATE_BRANCH --jq '.sha' 2>/dev/null || echo "")
```

| Resultat | Action |
|----------|--------|
| `LATEST_COMMIT` vide (pas de reseau / gh non auth) | Continuer silencieusement |
| `LATEST_COMMIT` = `KNOWN_COMMIT` | Template a jour — continuer |
| `LATEST_COMMIT` ≠ `KNOWN_COMMIT` | **Avertir et demander confirmation** |

Si mise a jour disponible, afficher **avant de continuer** :

```
⚠️  Mise a jour du template disponible

   Template  : $TEMPLATE_REPO
   Sync local : $SYNCED_AT ($KNOWN_COMMIT)
   Disponible : $LATEST_COMMIT

   Il est recommande de synchroniser avant de demarrer la session.
   Lancez /init-project (option d) pour mettre a jour commandes et agents.

   Continuer quand meme ? [O/n]
```

- Si **non** → stopper ici, l'utilisateur lance `/init-project`
- Si **oui** → continuer avec les etapes suivantes

### Etape 2 — Lecture de la memoire projet

Lire `.claude/memory/MEMORY.md` (source de verite unique).

Extraire :
- Version courante et environnements
- Travail en cours (branche, phase, issues actives)
- Regles critiques du projet
- Corrections de comportement a appliquer

### Etape 3 — Creation de la TEAM

**Sans demander confirmation**, creer immediatement la team :

1. **TeamCreate** avec le nom `{TEAM_NAME}` (defini dans CLAUDE.md)

2. **Spawner TOUS les agents en parallele** (un seul message avec N appels Task) :

| Nom agent | Type (subagent_type) | Role | Modele |
|-----------|---------------------|------|--------|
| `cdp` | `cdp` | Chef de Projet — Team Leader | sonnet |
| `planner` | `implementation-planner` | Planification + contrats API | sonnet |
| `dev-backend` | `dev-backend` | Backend (stack detectee) | sonnet |
| `dev-frontend` | `dev-frontend` | Frontend (stack detectee) | sonnet |
| `code-reviewer` | `code-reviewer` | Revue de code | sonnet |
| `qa` | `qa` | Tests et validation | sonnet |
| `security` | `security` | Audit securite | sonnet |
| `doc-updater` | `doc-updater` | Documentation | sonnet |
| `deployer` | `deploy` | Deploiement QUALIF/PROD | sonnet |
| `infra` | `infra` | Infrastructure Docker/Helm/CI | sonnet |
| `marketing` | `marketing-release` | Communication de release | sonnet |

> **Adapter la liste** selon la stack du projet (definie dans `project-config.json`) :
> - Pas de frontend → ne pas spawner `dev-frontend`
> - Firmware → ajouter `dev-firmware` (subagent_type: `dev-firmware`)
> - Pas de K8s/Docker → `infra` optionnel

**Prompt pour `cdp`** (team leader) :

```
Lis .claude/agents/cdp.md et applique ces instructions pour toute la session.

Tu es le Chef de Projet de {PROJECT_NAME} dans la team {TEAM_NAME}.
Memoire projet : .claude/memory/MEMORY.md

Attends les instructions de l'utilisateur avant de demarrer un workflow.
```

**Prompt pour tous les autres agents** :

```
Lis .claude/agents/context/TEAMMATES_PROTOCOL.md puis .claude/agents/[nom].md,
et applique ces instructions pour toute la session.

Tu fais partie de la team {TEAM_NAME} sur le projet {PROJECT_NAME}.
Reste en mode IDLE et attends les ordres du CDP avant de commencer tout travail.
```

### Etape 4 — Etat du backlog GitHub

Executer les deux requetes en parallele :

**Milestone actif :**
```bash
gh api repos/{owner}/{repo}/milestones \
  --jq '[.[] | select(.state=="open")] | sort_by(.due_on) | .[0] | {title, open_issues, closed_issues, due_on}'
```
Calculer : `progress = closed_issues / (open_issues + closed_issues) * 100`
Si aucun milestone actif → ne pas afficher le bloc milestone.

**Issues ouvertes :**
```bash
gh issue list --state open --limit 50 \
  --json number,title,labels,milestone,assignees,updatedAt \
  --jq 'sort_by(.milestone.title, .number) | .[] | [.number, .title, ([.labels[].name] | join(",")), (.milestone.title // "—"), ([.assignees[].login] | join(",") | if . == "" then "-" else . end), .updatedAt[:10]] | @tsv'
```

### Etape 5 — Confirmation a l'utilisateur



```markdown
## Session demarree — {PROJECT_NAME}

**Team** : {TEAM_NAME}
**Version** : [lue depuis MEMORY.md]
**Branche** : [lue depuis MEMORY.md]
**Travail en cours** : [lu depuis MEMORY.md]

---

**Milestone actif** : <version>  ████████░░  <X>%  (<closed>/<total> issues)
**Echeance** : <date ou "non definie">

### Backlog — Issues ouvertes

| # | Titre | Labels | Milestone | Assignee | Maj |
|---|-------|--------|-----------|----------|-----|
| 42 | ... | feature | v1.2.0 | - | 2026-01-10 |
| 38 | ... | bug | v1.2.0 | @user | 2026-01-08 |
| 35 | ... | refactor | — | - | 2026-01-05 |

_(Si aucune issue ouverte : "Aucune issue ouverte.")_

---

**Agents actifs (IDLE)** :
- cdp (Chef de Projet — interlocuteur principal)
- planner / dev-backend / dev-frontend
- code-reviewer / qa / security
- doc-updater / deployer / infra / marketing

**Commandes disponibles** :
- `/feature <description>` — Nouvelle feature complete
- `/bugfix <description>` — Correction de bug
- `/backlog [description]` — Consulter ou traiter les GitHub Issues
- `/milestone status` — Progression du milestone actif
- `/review [scope] [mode]` — Revue de code
- `/secu [scope]` — Audit securite
- `/deploy qualif|prod` — Deployer
- `/end-session` — Cloturer la session
```

## Regles

- La MEMORY projet est la **seule source de verite** au demarrage
- La TEAM est **toujours creee** sans demander confirmation
- Le nom de la TEAM est **toujours** `{TEAM_NAME}` (defini dans CLAUDE.md)
- Le CDP est **toujours le premier agent spawne** (team leader)
- Tous les autres agents demarrent en **mode IDLE strict** — ils attendent
  un ordre explicite du CDP via SendMessage, sans verifier la TaskList
