# CLAUDE.md - Project Management Template

> **Version** : 1.0.0
> **Template** : Claude Code Project Management
> **Guide d'initialisation** : [.claude/INITIALIZATION.md](.claude/INITIALIZATION.md)

---

## INITIALISATION OBLIGATOIRE

**IMPORTANT** : Au demarrage, verifie l'etat d'initialisation du projet.

### Verification

1. **Verifier** si `.claude/project-config.json` existe
2. **Si NON** → Lancer l'initialisation (voir ci-dessous)
3. **Si OUI** → Continuer normalement

### Processus d'Initialisation

> Documentation complete : [.claude/INITIALIZATION.md](.claude/INITIALIZATION.md)

**Etape 1 : Detection de code existant**

Avant toute question, analyser le projet pour detecter :
- Fichiers de configuration (`package.json`, `go.mod`, `requirements.txt`, etc.)
- Structure de dossiers (`src/`, `server/`, `web/`, etc.)
- Fichiers CI/CD (`.github/workflows/`, `.gitlab-ci.yml`, etc.)

**Etape 2 : Proposition a l'utilisateur**

```
[Si code detecte]
Je detecte du code existant dans ce projet :
- <liste des technologies detectees>

Voulez-vous :
a) Initialiser en analysant le code existant (recommande)
b) Initialiser manuellement (questionnaire complet)
c) Annuler

[Si projet vide]
Ce projet semble vide. Lancement du questionnaire d'initialisation.
```

**Etape 3 : Configuration**

- Si analyse automatique : deduire la stack et proposer confirmation
- Si manuel : poser les questions (voir INITIALIZATION.md)
- Generer `project-config.json` et les agents

---

## Configuration Projet

> Cette section sera completee par `/init-project`

| Parametre | Valeur |
|-----------|--------|
| Nom du projet | `{{PROJECT_NAME}}` |
| Backend | `{{BACKEND_TECH}}` |
| Frontend | `{{FRONTEND_TECH}}` |
| Base de donnees | `{{DATABASE}}` |
| CI/CD | `{{CICD_SYSTEM}}` |
| Deploiement | `{{DEPLOY_TARGET}}` |

---

## Architecture des Agents

### Agents de Workflow

| Agent | Role | Commande |
|-------|------|----------|
| **CDP** | Chef de projet, orchestration | `/cdp` |
| **Planner** | Planification d'implementation | `/plan` |
| **Reviewer** | Revue de code | `/review` |
| **QA** | Tests et validation | `/qa` |
| **Security** | Audit de securite | `/secu` |
| **Doc** | Documentation | `/doc` |
| **Deploy** | Deploiement | `/deploy` |

### Agents de Developpement

> Generes selon la stack technique du projet

| Agent | Condition | Fichier |
|-------|-----------|---------|
| dev-backend | Si backend configure | `.claude/agents/dev-backend.md` |
| dev-frontend | Si frontend configure | `.claude/agents/dev-frontend.md` |
| dev-firmware | Si firmware configure | `.claude/agents/dev-firmware.md` |
| dev-mobile | Si mobile configure | `.claude/agents/dev-mobile.md` |

---

## Commandes Disponibles

### Developpement

| Commande | Description | Usage |
|----------|-------------|-------|
| `/feature` | Nouvelle fonctionnalite | `/feature <description>` |
| `/bugfix` | Correction de bug | `/bugfix <description>` |
| `/hotfix` | Correction urgente prod | `/hotfix <description>` |
| `/refactor` | Refactoring sans changement fonctionnel | `/refactor <description>` |

### Validation

| Commande | Description | Usage |
|----------|-------------|-------|
| `/review` | Revue de code | `/review` |
| `/qa` | Tests complets | `/qa` |
| `/secu` | Audit de securite | `/secu [scope]` |

### Deploiement

| Commande | Description | Usage |
|----------|-------------|-------|
| `/deploy qualif` | Deploiement qualification | `/deploy qualif` |
| `/deploy prod` | Deploiement production | `/deploy prod` |

### Utilitaires

| Commande | Description | Usage |
|----------|-------------|-------|
| `/init-project` | Initialiser/reconfigurer le projet | `/init-project` |
| `/doc` | Mettre a jour la documentation | `/doc` |
| `/cdp` | Lancer l'orchestrateur complet | `/cdp <description>` |

---

## Workflows

### Workflow Standard (CDP)

```
/feature ou /bugfix
       |
       v
   [PLAN] --> Plan d'implementation
       |
       v
   [DEV] --> Implementation (agents tech)
       |
       v
   [REVIEW] --> Revue de code
       |
       v
   [QA] --> Tests
       |
       v
   [DOC] --> Documentation
       |
       v
   [DEPLOY] --> Qualification puis Production
```

### Workflow Securite (/secu)

```
/secu [scope]
       |
       v
   [SCAN] --> Analyse statique (SAST)
       |
       v
   [DEPS] --> Audit dependances
       |
       v
   [SECRETS] --> Detection secrets/credentials
       |
       v
   [OWASP] --> Verification OWASP Top 10
       |
       v
   [REPORT] --> Rapport + recommandations
       |
       v
   [FIX] --> Corrections (optionnel)
```

### Workflow Hotfix

```
/hotfix <description>
       |
       v
   [ANALYSE] --> Identification rapide
       |
       v
   [FIX] --> Correction minimale
       |
       v
   [TEST] --> Tests critiques uniquement
       |
       v
   [DEPLOY PROD] --> Deploiement direct
       |
       v
   [POST-MORTEM] --> Documentation incident
```

---

## Conventions

### Git

- **Branches** : `feature/<name>`, `bugfix/<name>`, `hotfix/<name>`
- **Commits** : Format conventionnel `type(scope): message`
  - Types : `feat`, `fix`, `docs`, `refactor`, `test`, `chore`
- **Tags** : `v<major>.<minor>.<patch>` (ex: `v2.1.0`)

### Code

- Tests obligatoires pour toute nouvelle fonctionnalite
- Revue de code avant merge
- Documentation des APIs et interfaces publiques

### Securite

- Jamais de secrets dans le code
- Validation des entrees utilisateur
- Audit `/secu` avant chaque release majeure

---

## Structure Projet Recommandee

```
project/
├── .claude/
│   ├── agents/           # Agents specialises
│   ├── commands/         # Commandes slash
│   ├── templates/        # Templates d'agents
│   └── project-config.json
├── docs/                 # Documentation
├── src/                  # Code source
├── tests/                # Tests
├── CLAUDE.md             # Ce fichier
├── CHANGELOG.md          # Historique des versions
└── README.md             # Documentation projet
```

---

## Notes pour Claude

### Regles Critiques

1. **Toujours verifier l'initialisation** au debut de chaque session
2. **Utiliser les agents specialises** selon la tache
3. **Suivre les workflows** definis pour chaque type de tache
4. **Commiter regulierement** avec des messages clairs
5. **Ne jamais push sur main** sans validation

### Detection du Contexte

- Lire `project-config.json` pour connaitre la stack technique
- Adapter les commandes et outils selon la configuration
- Utiliser les agents generes pour le projet

### Gestion des Erreurs

- Si CI echoue : analyser, corriger, re-tester
- Si tests echouent : ne pas deployer
- Si securite critique : bloquer et alerter
