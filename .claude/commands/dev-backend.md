# Commande /dev-backend - Implémentation Backend Go

Lance le sous-agent dev-backend pour implémenter du code Go selon un plan.

## Argument reçu

$ARGUMENTS

**Formats possibles** :
- `/dev-backend [plan]` : Plan d'implémentation backend
- `/dev-backend fix "description"` : Correction de bug backend
- `/dev-backend review "corrections"` : Corrections post-review

## Instructions

Utilise le Task tool pour lancer le sous-agent dev-backend avec les paramètres suivants :

```
subagent_type: "dev-backend"
description: "Implémenter backend Go"
prompt: voir ci-dessous
```

### Prompt à transmettre au sous-agent

```
Implémente le code backend Go pour BuzzControl.

**Contexte projet :**
- Répertoire : /home/user/BuzzMaster
- Serveur Go : server-go/
- Config version : server-go/config.json
- Procédure : docs/DEV_PROCEDURE.md

**Input utilisateur :** $ARGUMENTS

**Étapes à exécuter :**

1. **Collecter le contexte**
   - Version actuelle : server-go/config.json → "version"
   - Branche : git branch --show-current

2. **Incrémenter la version (OBLIGATOIRE)**
   - AVANT tout code, incrémenter z : 2.40.1 → 2.40.2
   - Commit : chore(version): Bump to X.Y.Z

3. **Implémenter selon l'ordre backend**
   | Étape | Fichier | Actions |
   |-------|---------|---------|
   | 1 | internal/game/models.go | Structs, champs, JSON tags |
   | 2 | internal/game/engine.go | Logique métier, mutex |
   | 3 | internal/game/engine_test.go | Tests unitaires |
   | 4 | internal/protocol/messages.go | Actions, payloads |
   | 5 | cmd/server/main.go | Handlers WebSocket |
   | 6 | internal/server/http.go | Endpoints REST |

4. **Standards Go**
   - PascalCase exports, camelCase privé
   - Error handling obligatoire
   - Thread-safety avec mutex
   - Tests table-driven

5. **Vérifications finales**
   - go build ./cmd/server (compilation)
   - go test ./... -v (tests)
   - go test -race ./... (race conditions)
   - git push origin <branche>

6. **Générer le résumé** :
   - Fichiers modifiés avec changements
   - Tests créés et résultats
   - Commits créés
   - Nouvelles actions WebSocket (pour DEV-FRONTEND)
   - Nouveaux champs GameState (pour DEV-FRONTEND)

**Règles critiques :**
- Backend UNIQUEMENT : Ne pas toucher aux fichiers JSX/CSS
- Version first : Incrémenter z AVANT tout code
- Tests obligatoires : Chaque fonction publique = tests
- Thread-safety : Mutex pour état partagé
- Commits atomiques : 1 commit par tâche logique
```

## Action immédiate

Lance maintenant le sous-agent dev-backend avec le Task tool.
