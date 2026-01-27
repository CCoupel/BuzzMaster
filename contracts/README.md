# Contrats API - BuzzMaster

Ce répertoire contient les **contrats d'interface** entre le backend (Go) et le frontend (React).

## Principe "Contract First"

```
PLAN (définit) → contracts/ → DEV-BACKEND (implémente/ajuste)
                     │
                     └──────→ DEV-FRONTEND (consomme)
```

1. **PLAN** crée les contrats avant le développement
2. **DEV-BACKEND** implémente et peut ajuster si nécessaire
3. **DEV-FRONTEND** consulte les contrats pour implémenter

## Structure

```
contracts/
├── README.md                    # Ce fichier
├── websocket-actions.md         # Actions WebSocket (server ↔ client)
├── http-endpoints.md            # Endpoints REST API
├── game-state.md                # Structure GameState
└── models.md                    # Modèles partagés (Team, Bumper, Question)
```

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

L'agent `dev-frontend` **doit** consulter les contrats avant d'implémenter :
1. Lire `websocket-actions.md` pour les handlers WebSocket
2. Lire `http-endpoints.md` pour les appels API
3. Lire `game-state.md` pour les champs à afficher

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
