# Contrats API - BuzzMaster

Ce repertoire contient les **contrats d'interface** entre les differents composants du systeme.

## Composants du Systeme

```
┌─────────────┐     WebSocket/HTTP     ┌─────────────┐
│  Frontend   │◄──────────────────────►│   Server    │
│   (React)   │                        │    (Go)     │
└─────────────┘                        └──────┬──────┘
                                              │
                                       TCP/UDP│Broadcast
                                              │
                                       ┌──────▼──────┐
                                       │  BuzzClick  │
                                       │  (ESP32-C3) │
                                       └─────────────┘
```

## Principe "Contract First"

```
PLAN (definit) → contracts/ → DEV-BACKEND (implemente/ajuste)
                     │
                     ├──────→ DEV-FRONTEND (consomme - WebSocket/HTTP)
                     │
                     └──────→ DEV-BUZZCLICK (consomme - TCP/UDP)
```

1. **PLAN** cree les contrats avant le developpement
2. **DEV-BACKEND** implemente et peut ajuster si necessaire
3. **DEV-FRONTEND** consulte les contrats WebSocket/HTTP
4. **DEV-BUZZCLICK** consulte les contrats TCP/UDP (protocole fige)

## Structure

```
contracts/
├── README.md                    # Ce fichier
├── websocket-actions.md         # Actions WebSocket (server ↔ web client)
├── http-endpoints.md            # Endpoints REST API
├── game-state.md                # Structure GameState
├── models.md                    # Modeles partages (Team, Bumper, Question)
└── tcp-udp-protocol.md          # Protocole BuzzClick (server ↔ buzzer)
```

## Contrats par Composant

| Composant | Contrats a consulter |
|-----------|----------------------|
| **Frontend (React)** | websocket-actions.md, http-endpoints.md, game-state.md, models.md |
| **BuzzClick (ESP32)** | tcp-udp-protocol.md, models.md (Bumper) |
| **Backend (Go)** | Tous (reference) |

## Format des fichiers

### websocket-actions.md

```markdown
# Actions WebSocket

## [ACTION_NAME]

| Propriété | Valeur |
|-----------|--------|
| Direction | `Server→Client` / `Client→Server` / `Bidirectional` |
| Phase     | Phase de jeu concernée (STARTED, STOPPED, etc.) |
| Trigger   | Ce qui déclenche l'action |

### Payload

| Champ | Type | Obligatoire | Description |
|-------|------|-------------|-------------|
| FIELD | string | ✅ | Description |

### Exemple

\`\`\`json
{
  "ACTION": "ACTION_NAME",
  "MSG": {
    "FIELD": "value"
  }
}
\`\`\`
```

### http-endpoints.md

```markdown
# Endpoints HTTP

## [METHOD] /path

| Propriété | Valeur |
|-----------|--------|
| Auth      | Aucune / Admin / Token |
| Content-Type | application/json / multipart/form-data |

### Request

| Param | Type | In | Description |
|-------|------|-----|-------------|
| id | string | path | Identifiant |

### Response 200

\`\`\`json
{
  "success": true,
  "data": {}
}
\`\`\`

### Errors

| Code | Description |
|------|-------------|
| 400 | Invalid input |
| 404 | Not found |
```

### game-state.md

```markdown
# GameState

Structure de l'état du jeu broadcast aux clients.

| Champ | Type | Description |
|-------|------|-------------|
| PHASE | string | Phase actuelle (STOP, PREPARE, READY, START, PAUSE) |
| DELAY | int | Durée totale en secondes |
| CURRENT_TIME | int | Temps restant |
```

## Cycle de vie des contrats

### Création (PLAN)

L'agent `implementation-planner` crée ou met à jour les contrats :
- Nouvelles actions WebSocket
- Nouveaux endpoints HTTP
- Nouveaux champs GameState/modèles

### Modification (DEV-BACKEND)

L'agent `dev-backend` peut ajuster les contrats si :
- Contrainte technique découverte
- Optimisation nécessaire
- Erreur dans le contrat initial

**Obligation** : Documenter tout changement dans le commit.

### Consultation (DEV-FRONTEND)

L'agent `dev-frontend` **doit** consulter les contrats avant d'implementer :
1. Lire `websocket-actions.md` pour les handlers WebSocket
2. Lire `http-endpoints.md` pour les appels API
3. Lire `game-state.md` pour les champs a afficher

### Consultation (DEV-BUZZCLICK)

L'agent `dev-buzzclick` **doit** consulter les contrats avant d'implementer :
1. Lire `tcp-udp-protocol.md` pour le protocole de communication
2. Lire `models.md` pour la structure Bumper

**Important** : Le protocole TCP/UDP est **fige** pour la retrocompatibilite.
Toute modification doit etre coordonnee avec `dev-backend`.

## Versioning

Les contrats évoluent avec le projet. Chaque modification doit :
1. Être documentée dans le fichier concerné
2. Inclure la date de modification
3. Être committée avec le code correspondant

## Validation

Le `code-reviewer` vérifie que :
- Le code backend respecte les contrats
- Le code frontend utilise les contrats correctement
- Les contrats sont à jour avec l'implémentation
