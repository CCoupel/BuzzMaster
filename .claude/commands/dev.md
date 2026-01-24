# Commande /dev - Implémentation de Feature

Lance le sous-agent DEV pour implémenter du code selon un plan d'implémentation.

## Argument reçu

$ARGUMENTS

**Formats possibles** :
- `/dev` : Demande le plan si absent
- `/dev [plan]` : Plan fourni directement
- `/dev fix "description"` : Bugfix
- `/dev review "corrections"` : Corrections post-review

## Instructions

Utilise le Task tool pour lancer le sous-agent dev-feature-implementation avec les paramètres suivants :

```
subagent_type: "dev-feature-implementation"
description: "Implémenter feature/bugfix"
prompt: voir ci-dessous
```

### Prompt à transmettre au sous-agent

```
Implémente le code pour BuzzControl selon le plan fourni.

**Contexte projet :**
- Répertoire : C:\Users\cyril\Documents\VScode\buzzcontrol
- Serveur Go : server-go/
- Frontend React : server-go/web/src/
- Config version : server-go/config.json
- Procédure : docs/DEV_PROCEDURE.md

**Input utilisateur :** $ARGUMENTS

**Étapes à exécuter :**

1. **Collecter le contexte**
   - Lire la version actuelle : server-go/config.json → "version"
   - Récupérer la branche : git branch --show-current
   - Identifier le plan ou les corrections à appliquer

2. **Incrémenter la version (OBLIGATOIRE)**
   - AVANT tout code, incrémenter z : 2.40.1 → 2.40.2
   - Commit : chore(version): Bump to X.Y.Z

3. **Implémenter selon l'ordre**
   | Étape | Fichiers | Actions |
   |-------|----------|---------|
   | 1 | internal/game/models.go | Structs, champs |
   | 2 | internal/game/engine.go | Logique métier |
   | 3 | internal/game/engine_test.go | Tests unitaires |
   | 4 | internal/protocol/messages.go | Nouveaux messages |
   | 5 | cmd/server/main.go | Handlers WebSocket |
   | 6 | web/src/pages/*.jsx | Admin UI |
   | 7 | web/src/pages/PlayerDisplay.jsx | TV display |
   | 8 | web/src/pages/*.css | Styles |

4. **Vérifications finales**
   - go build ./cmd/server (compilation)
   - go test ./... -v (tests)
   - git push origin <branche>

5. **Générer le résumé d'implémentation** :
   - Tâches complétées (Backend, Frontend, Tests)
   - Statistiques (fichiers, lignes, commits)
   - Vérifications (build, tests, lint)
   - Issues rencontrées (si applicable)

**Standards de code :**
- Go : PascalCase exports, camelCase privé, tests obligatoires
- React : Composants fonctionnels + hooks, PropTypes
- TV (/tv) : STATIQUE, pas de scroll
- Commits : <type>(<scope>): <description>

**Règles critiques :**
- Version first : Incrémenter z AVANT tout code
- Suivre le plan : Ne pas improviser
- Tests obligatoires : Chaque fonction = tests
- Commits atomiques : 1 commit par tâche majeure
- Build avant fin : Vérifier la compilation
- Push en fin : Pousser sur la branche feature

**Ce que tu ne dois PAS faire :**
- Dévier du plan
- Sauter les tests unitaires
- Faire un seul gros commit
- Modifier la documentation (agent DOC)
- Déployer (agent DEPLOY)
- Incrémenter y (agent PLAN)
```

## Action immédiate

Lance maintenant le sous-agent dev-feature-implementation avec le Task tool.
