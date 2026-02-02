# Règles Communes aux Agents DEV

> **Ce fichier contient les règles communes à tous les agents de développement.**
> Agents concernés : `dev-backend`, `dev-frontend`, `dev-feature-implementation`, `dev-buzzclick`
>
> **Prérequis** : Chaque agent DEV doit aussi respecter `@import COMMON.md`

---

## Étape Critique : Incrément de Version

**AVANT TOUT CHANGEMENT DE CODE**, vous DEVEZ :

1. **Lire** la version actuelle depuis `server-go/config.json`
2. **Incrémenter** le numéro z (patch) : `X.Y.Z` → `X.Y.Z+1`
3. **Committer** : `chore(version): Bump to X.Y.Z+1`

```bash
# Exemple
# Version actuelle : 2.40.1
# Nouvelle version : 2.40.2
```

### Règles de Versioning

| Qui | Incrémente | Quand |
|-----|------------|-------|
| **PLAN** | y (minor) | Nouvelle feature (`2.40.0` → `2.41.0`) |
| **DEV** | z (patch) | Chaque cycle de développement (`2.41.0` → `2.41.1`) |
| **DOC** | Reset z=0 | Finalisation release (`2.41.15` → `2.41.0`) |

---

## Format des Commits

```
<type>(<scope>): <description>

<optional body>
```

### Types Autorisés

| Type | Usage |
|------|-------|
| `feat` | Nouvelle fonctionnalité |
| `fix` | Correction de bug |
| `refactor` | Refactoring sans changement de comportement |
| `test` | Ajout/modification de tests |
| `docs` | Documentation uniquement |
| `style` | Formatage, CSS (sans changement de logique) |
| `chore` | Maintenance (version, config, deps) |
| `perf` | Amélioration de performance |

### Scopes par Agent

| Agent | Scopes |
|-------|--------|
| dev-backend | `engine`, `protocol`, `http`, `websocket`, `tcp`, `models` |
| dev-frontend | `admin`, `tv`, `components`, `hooks`, `css` |
| dev-buzzclick | `buzzclick`, `led`, `wifi`, `tcp` |
| dev-feature-implementation | Tous les scopes |

### Exemples

```bash
# Backend
feat(engine): Add QCM hint invalidation logic
fix(websocket): Handle nil bumper in BUTTON action
test(engine): Add tests for Memory game scoring

# Frontend
feat(tv): Add QCM hint badges on answers
fix(admin): Fix team card overflow in GamePage
style(css): Improve responsive breakpoints

# BuzzClick
feat(buzzclick): Add OTA update support
fix(buzzclick): Implement exponential backoff for WiFi
```

---

## Contrats API (OBLIGATOIRE)

**AVANT d'implémenter**, consultez les contrats définis par l'agent PLAN :

```
contracts/
├── websocket-actions.md   # Actions WebSocket
├── http-endpoints.md      # Endpoints REST
├── game-state.md          # Structure GameState
└── models.md              # Modèles partagés (Team, Bumper, Question)
```

### Workflow Contract-First

1. **Lire** les contrats définis dans le plan
2. **Implémenter** selon les contrats
3. **Modifier** les contrats si contrainte technique (avec justification)

### Modification de Contrat

Si vous devez modifier un contrat, documentez-le dans votre summary :

```markdown
## ⚠️ Modification de Contrat

**Fichier** : `contracts/websocket-actions.md`
**Action** : QCM_HINT

**Original** :
| Champ | Type |
|-------|------|
| COLOR | string |

**Modifié** :
| Champ | Type |
|-------|------|
| COLOR | string |
| TIMESTAMP | int64 | ← Ajouté pour sync

**Raison** : Frontend a besoin du timestamp pour sync animation
```

---

## Vérifications Obligatoires

### Avant de Terminer

| Vérification | Commande | Agent |
|--------------|----------|-------|
| Build Go | `go build ./cmd/server` | backend, feature |
| Build npm | `npm run build` | frontend, feature |
| Build PlatformIO | `pio run -e buzzclick` | buzzclick |
| Tests Go | `go test ./...` | backend, feature |
| Pas de scroll TV | Vérification visuelle 1920x1080 | frontend |

### Ordre de Build (Mode Portable)

**⚠️ IMPORTANT** : TOUJOURS rebuilder le frontend AVANT le Go build.

```bash
# Correct
cd server-go/web && npm run build && cd .. && go build -o server.exe ./cmd/server

# Incorrect (modifications React non prises en compte)
go build -o server.exe ./cmd/server
```

---

## Standards de Code

### Backend Go

```go
// Exported function with documentation
// ProcessButtonPress handles button press from a bumper during gameplay.
func (e *Engine) ProcessButtonPress(bumperID string, button string) (int, error) {
    e.mu.Lock()
    defer e.mu.Unlock()

    // Validate game state
    if e.gameState.Phase != PhaseStarted {
        return 0, fmt.Errorf("game not in STARTED phase")
    }
    // ...
}
```

- **Naming** : PascalCase (exported), camelCase (private)
- **Error handling** : Toujours retourner et gérer les erreurs
- **Thread-safety** : Utiliser mutex pour l'état partagé
- **Tests** : Table-driven tests obligatoires

### Frontend React

```jsx
import { useState, useEffect, useMemo } from 'react'
import './ComponentName.css'

export default function ComponentName({ teams, onSelect }) {
    const [selected, setSelected] = useState(null)
    // ...
}
```

- **Components** : Fonctionnels avec hooks
- **CSS** : Variables CSS (pas de valeurs hardcodées)
- **TV Display** : JAMAIS de scroll (overflow: hidden)

### Firmware BuzzClick

```cpp
// IRAM_ATTR obligatoire pour handlers d'interruption
void IRAM_ATTR buttonHandler() {
    if (isGameStarted) {
        buttonInfo->time = String(micros());
        buttonInfo->pressed = true;
    }
}
```

- **Watchdog** : Reset dans les boucles longues (30s max)
- **Mémoire** : 160KB RAM limite, éviter allocations dynamiques
- **Interruptions** : IRAM_ATTR obligatoire

---

## Ce que les Agents DEV NE DOIVENT PAS Faire

| Interdit | Responsable |
|----------|-------------|
| Modifier la documentation | DOC agent |
| Déployer | DEPLOY agent |
| Incrémenter y (version minor) | PLAN agent |
| Exécuter les tests E2E | QA agent |
| Écrire les scénarios E2E | TEST-WRITER agent |

---

## Coordination Entre Agents DEV

### Ordre d'Exécution Standard

```
dev-backend → dev-frontend
dev-backend → dev-buzzclick
```

Le backend DOIT être complété AVANT le frontend/buzzclick si :
- Nouvelles actions WebSocket
- Nouveaux champs GameState
- Nouveaux endpoints HTTP
- Modifications de protocole

### Parallélisation Possible

Frontend et BuzzClick peuvent être parallélisés APRÈS le backend :
```
dev-backend → (dev-frontend ║ dev-buzzclick)
```

---

## Format de Summary

Chaque agent DEV doit produire un summary structuré :

```markdown
# [Agent] Implementation Summary

## Version
- Previous: X.Y.Z
- Current: X.Y.Z+1

## Files Modified

### [file.go/jsx/cpp]
- [Description des modifications]

## Tests Results
- Total: N tests
- Passed: N
- Failed: 0
- Coverage: XX%

## Commits
1. `chore(version): Bump to X.Y.Z+1`
2. `feat(scope): Description`

## Verification
- [x] Build OK
- [x] Tests PASS
- [x] [Autres vérifications spécifiques]
```
