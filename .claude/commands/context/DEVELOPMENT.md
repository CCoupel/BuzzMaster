# DEVELOPMENT.md - Patterns de Développement

Ce fichier centralise les patterns partagés par les commandes `/dev`, `/dev-backend`, `/dev-frontend`, et `/dev-buzzclick`.

---

## 1. Agents de Développement

| Commande | Agent | Scope |
|----------|-------|-------|
| `/dev` | Dispatch | Backend + Frontend |
| `/dev-backend` | dev-backend | Go uniquement |
| `/dev-frontend` | dev-frontend | React uniquement |
| `/dev-buzzclick` | dev-buzzclick | ESP32 firmware |

---

## 2. Dispatch Automatique (/dev)

```
Analyser les fichiers du plan :
├── *.go, internal/, cmd/ → dev-backend
├── *.jsx, *.css, web/src/ → dev-frontend
├── *.cpp, *.h, src/BuzzClick/ → dev-buzzclick
└── Mixte → Voir section 3
```

---

## 3. Stratégie Multi-Agent

### Séquentiel Obligatoire (Backend → Frontend)

Si le backend crée des éléments utilisés par le frontend :
- Nouvelles actions WebSocket
- Nouveaux champs GameState
- Nouveaux endpoints HTTP

```
1. Lancer dev-backend
2. Récupérer le résumé (actions WS, champs)
3. Lancer dev-frontend avec le résumé
```

### Parallèle Autorisé

Si modifications isolées :
- Refactoring CSS isolé
- Tests unitaires isolés
- Composants sans nouvelles données

```
Lancer dev-backend ET dev-frontend en parallèle (2 Task tools)
```

---

## 4. Workflow Commun

### Étape 1 : Collecte Contexte

```bash
# Version actuelle
cat server-go/config.json | grep '"version"'

# Branche courante
git branch --show-current
```

### Étape 2 : Incrémenter Version (OBLIGATOIRE)

```bash
# AVANT tout code, incrémenter z
# 2.40.1 → 2.40.2
git add server-go/config.json
git commit -m "chore(version): Bump to X.Y.Z"
```

### Étape 3 : Implémenter

Voir ordre par agent (sections 5-7).

### Étape 4 : Build Final

```bash
# ORDRE CRITIQUE : Frontend AVANT Backend (mode portable)
cd server-go/web && npm run build && cd .. && go build -o server.exe ./cmd/server
```

### Étape 5 : Vérifications

```bash
# Tests
go test ./... -v

# Push
git push origin <branche>
```

### Étape 6 : Générer Résumé

```markdown
## Résumé DEV

**Fichiers modifiés :**
- [liste]

**Tests créés :**
- [liste]

**Commits :**
- [liste]

**Pour Frontend (si backend) :**
- Actions WebSocket : [liste]
- Champs GameState : [liste]
```

---

## 5. Ordre Backend (Go)

| Étape | Fichier | Actions |
|-------|---------|---------|
| 1 | internal/game/models.go | Structs, champs, JSON tags |
| 2 | internal/game/engine.go | Logique métier, mutex |
| 3 | internal/game/engine_test.go | Tests unitaires |
| 4 | internal/protocol/messages.go | Actions, payloads |
| 5 | cmd/server/main.go | Handlers WebSocket |
| 6 | internal/server/http.go | Endpoints REST |

### Standards Go

- PascalCase exports, camelCase privé
- Error handling obligatoire
- Thread-safety avec mutex
- Tests table-driven

---

## 6. Ordre Frontend (React)

| Étape | Fichier | Actions |
|-------|---------|---------|
| 1 | hooks/useWebSocket.js | Nouveaux handlers |
| 2 | components/*.jsx | Composants réutilisables |
| 3 | pages/*Page.jsx | Pages admin |
| 4 | pages/PlayerDisplay.jsx | Affichage TV (STATIQUE!) |
| 5 | pages/*.css | Styles |

### Standards React

- Composants fonctionnels + hooks
- useMemo/useCallback pour optimisation
- CSS variables (pas de hardcoded)
- PropTypes si nécessaire

### Contrainte TV STATIQUE

```css
/* PlayerDisplay.jsx - OBLIGATOIRE */
overflow: hidden;  /* JAMAIS auto ou scroll */
/* Utiliser vh, vw, % - pas de px fixes */
/* Tester à 1920x1080 */
```

---

## 7. Ordre BuzzClick (ESP32)

| Étape | Fichier | Actions |
|-------|---------|---------|
| 1 | src/BuzzClick/config.h | Constantes |
| 2 | src/BuzzClick/network.cpp | WiFi, TCP |
| 3 | src/BuzzClick/protocol.cpp | Messages |
| 4 | src/BuzzClick/main.cpp | Logique principale |

### Standards ESP32

- C++17
- Gestion mémoire (pas de memory leak)
- Reconnexion WiFi avec backoff
- Économie batterie

---

## 8. Règles Critiques

| Règle | Détail |
|-------|--------|
| Version first | Incrémenter z AVANT tout code |
| Scope strict | Backend ne touche pas JSX, Frontend ne touche pas Go |
| Build order | Frontend AVANT Backend (mode portable) |
| Tests | Chaque fonction publique = tests |
| Commits | Atomiques, 1 commit par tâche logique |
| TV statique | Jamais de scroll sur PlayerDisplay |

---

## 9. Modes d'Appel

```bash
# Plan complet
/dev [plan détaillé]

# Backend seul
/dev backend [plan backend]
/dev-backend [plan]

# Frontend seul
/dev frontend [plan frontend]
/dev-frontend [plan]

# Bugfix
/dev fix "description du bug"
/dev-backend fix "bug backend"
/dev-frontend fix "bug frontend"

# Post-review
/dev review "corrections demandées"
```

---

## Usage

Dans les commandes DEV, référencer ce fichier :

```markdown
**Contexte DEV :** Voir `context/DEVELOPMENT.md`
- Workflow : section 4
- Ordre backend : section 5
- Ordre frontend : section 6
- Règles : section 8
```
