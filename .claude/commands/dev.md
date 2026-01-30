# Commande /dev - Implémentation de Feature

Lance les sous-agents DEV pour implémenter du code selon un plan d'implémentation.

## Argument reçu

$ARGUMENTS

**Formats possibles** :
- `/dev` : Demande le plan si absent
- `/dev [plan]` : Plan fourni directement (dispatch automatique backend/frontend)
- `/dev backend [plan]` : Backend Go uniquement
- `/dev frontend [plan]` : Frontend React uniquement
- `/dev fix "description"` : Bugfix
- `/dev review "corrections"` : Corrections post-review

## Mode de fonctionnement

### Mode unifié (par défaut)

Si aucun préfixe `backend` ou `frontend` :
1. Analyser le plan pour identifier les tâches backend et frontend
2. Déterminer si parallélisable ou séquentiel
3. Lancer les agents appropriés

### Mode séparé

- `/dev backend ...` → Lance uniquement `dev-backend`
- `/dev frontend ...` → Lance uniquement `dev-frontend`

## Instructions

### Étape 1 : Analyser le plan

Identifier les tâches :

| Type | Fichiers | Agent |
|------|----------|-------|
| Backend | `*.go`, `internal/`, `cmd/` | dev-backend |
| Frontend | `*.jsx`, `*.css`, `web/src/` | dev-frontend |

### Étape 2 : Déterminer les dépendances

**Séquentiel obligatoire (Backend → Frontend)** si :
- Nouvelles actions WebSocket
- Nouveaux champs GameState
- Nouveaux endpoints HTTP

**Parallélisable** si :
- Refactoring isolé
- Styling CSS uniquement
- Tests unitaires isolés

### Étape 3 : Lancer les agents

#### Si séquentiel : Backend d'abord

```
subagent_type: "dev-backend"
description: "Implémenter backend"
prompt: [Tâches backend du plan]
```

Attendre la fin, puis :

```
subagent_type: "dev-frontend"
description: "Implémenter frontend"
prompt: [Tâches frontend du plan + résumé backend]
```

#### Si parallélisable : Les deux en même temps

Utiliser deux appels Task en parallèle :

```
# Appel 1
subagent_type: "dev-backend"
description: "Implémenter backend"
prompt: [Tâches backend]

# Appel 2 (dans le même message)
subagent_type: "dev-frontend"
description: "Implémenter frontend"
prompt: [Tâches frontend]
```

### Prompt pour dev-backend

```
Implémente le code backend Go pour BuzzControl.

**Contexte projet :**
- Répertoire : /home/user/BuzzMaster
- Serveur Go : server-go/
- Config version : server-go/config.json

**Plan backend :** $ARGUMENTS (partie backend)

**Actions :**
1. Incrémenter z dans config.json
2. Implémenter : models → engine → tests → protocol → handlers
3. Commits atomiques
4. Documenter nouvelles actions WebSocket et champs GameState
5. Push
```

### Prompt pour dev-frontend

```
Implémente le code frontend React pour BuzzControl.

**Contexte projet :**
- Répertoire : /home/user/BuzzMaster
- Frontend React : server-go/web/src/
- Config version : server-go/config.json

**Plan frontend :** $ARGUMENTS (partie frontend)

**Résumé backend (si applicable) :**
- Nouvelles actions WebSocket : [liste]
- Nouveaux champs GameState : [liste]

**Actions :**
1. Incrémenter z dans config.json
2. Implémenter : hooks → components → pages → PlayerDisplay → CSS
3. Vérifier contrainte TV STATIQUE
4. Commits atomiques
5. Push
```

## Gestion des erreurs

| Situation | Action |
|-----------|--------|
| Plan manquant | Demander le plan ou lancer `/plan` |
| Backend échoue | Ne pas lancer frontend, retourner erreur |
| Frontend échoue | Retourner erreur avec détails |

## Action immédiate

1. **Si argument contient `backend`** → Lancer uniquement dev-backend
2. **Si argument contient `frontend`** → Lancer uniquement dev-frontend
3. **Sinon** → Analyser le plan, dispatcher vers backend et/ou frontend
