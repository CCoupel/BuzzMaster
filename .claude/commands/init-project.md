# Commande /init-project

Initialisation interactive du projet pour configurer l'environnement Claude Code.

## Declenchement Automatique

Cette commande est executee automatiquement si `.claude/project-config.json` n'existe pas.

## Workflow d'Initialisation

### Etape 1 : Informations Generales

Pose ces questions a l'utilisateur :

```
1. Quel est le nom du projet ?
   > [Texte libre]

2. Decris brievement le projet (1-2 phrases) :
   > [Texte libre]
```

### Etape 2 : Stack Backend

```
3. Quelle technologie backend utilises-tu ?
   a) Go
   b) Node.js (JavaScript/TypeScript)
   c) Python
   d) Java / Kotlin
   e) C# / .NET
   f) PHP
   g) Ruby
   h) Rust
   i) Aucun backend
```

### Etape 3 : Stack Frontend

```
4. Quelle technologie frontend utilises-tu ?
   a) React
   b) Vue.js
   c) Angular
   d) Svelte
   e) HTML/CSS/JS vanilla
   f) Aucun frontend
```

### Etape 4 : Mobile (optionnel)

```
5. As-tu une application mobile ?
   a) React Native
   b) Flutter
   c) iOS natif (Swift)
   d) Android natif (Kotlin)
   e) Pas de mobile
```

### Etape 5 : Firmware/Hardware (optionnel)

```
6. As-tu du code firmware ou embarque ?
   a) ESP32 (Arduino/PlatformIO)
   b) Raspberry Pi
   c) Arduino
   d) STM32
   e) Pas de firmware
```

### Etape 6 : Base de Donnees

```
7. Quelle base de donnees utilises-tu ?
   a) PostgreSQL
   b) MySQL / MariaDB
   c) MongoDB
   d) SQLite
   e) Redis
   f) Autre / Multiple
   g) Aucune
```

### Etape 7 : CI/CD

```
8. Quel systeme CI/CD utilises-tu ?
   a) GitHub Actions
   b) GitLab CI
   c) Jenkins
   d) CircleCI
   e) Azure DevOps
   f) Autre
   g) Aucun
```

### Etape 8 : Deploiement

```
9. Comment deploies-tu ton application ?
   a) Docker / Docker Compose
   b) Kubernetes
   c) Serverless (AWS Lambda, Vercel, etc.)
   d) VPS / Bare metal
   e) PaaS (Heroku, Railway, etc.)
   f) Autre
```

### Etape 9 : Tests

```
10. Quels frameworks de tests utilises-tu ? (plusieurs possibles)
    Backend: [Jest / Pytest / Go test / JUnit / etc.]
    Frontend: [Jest / Vitest / Cypress / Playwright / etc.]
    E2E: [Cypress / Playwright / Selenium / etc.]
```

### Etape 10 : Securite

```
11. Quels aspects securite sont importants pour ce projet ?
    a) Authentification utilisateurs
    b) API publique
    c) Donnees sensibles (RGPD, sante, finance)
    d) Paiements
    e) Multi-tenant
    f) Aucun aspect critique
```

## Actions Post-Questions

Une fois les reponses collectees :

### 1. Generer project-config.json

```json
{
  "name": "<PROJECT_NAME>",
  "description": "<DESCRIPTION>",
  "version": "0.1.0",
  "initialized_at": "<TIMESTAMP>",
  "stack": {
    "backend": "<BACKEND_TECH>",
    "frontend": "<FRONTEND_TECH>",
    "mobile": "<MOBILE_TECH>",
    "firmware": "<FIRMWARE_TECH>",
    "database": "<DATABASE>",
    "cicd": "<CICD_SYSTEM>",
    "deploy": "<DEPLOY_TARGET>"
  },
  "testing": {
    "backend": "<TEST_FRAMEWORK>",
    "frontend": "<TEST_FRAMEWORK>",
    "e2e": "<E2E_FRAMEWORK>"
  },
  "security": {
    "concerns": ["<CONCERN1>", "<CONCERN2>"]
  }
}
```

### 2. Generer les Agents de Developpement

Selon la stack, copier et adapter les templates depuis `.claude/templates/` :

| Stack | Template Source | Destination |
|-------|-----------------|-------------|
| Go | `templates/dev-backend-go.md` | `agents/dev-backend.md` |
| Node.js | `templates/dev-backend-node.md` | `agents/dev-backend.md` |
| Python | `templates/dev-backend-python.md` | `agents/dev-backend.md` |
| React | `templates/dev-frontend-react.md` | `agents/dev-frontend.md` |
| Vue.js | `templates/dev-frontend-vue.md` | `agents/dev-frontend.md` |
| ESP32 | `templates/dev-firmware-esp32.md` | `agents/dev-firmware.md` |
| React Native | `templates/dev-mobile-reactnative.md` | `agents/dev-mobile.md` |

### 3. Mettre a jour CLAUDE.md

Remplacer les placeholders `{{...}}` dans CLAUDE.md avec les vraies valeurs.

### 4. Mettre a jour l'agent CDP

Adapter `.claude/agents/cdp.md` pour inclure uniquement les agents pertinents.

### 5. Mettre a jour l'agent Deploy

Configurer `.claude/agents/deploy.md` selon le CI/CD et la cible de deploiement.

### 6. Configurer l'agent Security

Adapter `.claude/agents/security.md` selon les preoccupations de securite declarees.

## Message de Fin

```
Projet "<PROJECT_NAME>" initialise avec succes !

Configuration :
- Backend : <BACKEND>
- Frontend : <FRONTEND>
- Database : <DATABASE>
- CI/CD : <CICD>

Agents generes :
- dev-backend.md
- dev-frontend.md
- [autres selon config]

Commandes disponibles :
- /feature : Nouvelle fonctionnalite
- /bugfix : Correction de bug
- /hotfix : Correction urgente
- /secu : Audit de securite
- /deploy : Deploiement

Tu peux maintenant commencer a travailler sur le projet !
```

## Reinitialisation

Si `/init-project` est execute sur un projet deja initialise :

```
Ce projet est deja initialise.

Veux-tu :
a) Reconfigurer completement (ecrase la config actuelle)
b) Modifier certains parametres uniquement
c) Annuler
```
