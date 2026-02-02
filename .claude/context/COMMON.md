# COMMON.md - Patterns et Commandes Partagés

Ce fichier centralise les éléments répétés dans les définitions de commandes et agents Claude Code. Les agents et commandes doivent référencer ce fichier plutôt que de dupliquer ces informations.

> **Objectif** : Éliminer ~1500 lignes de duplication à travers 17 commandes et 13 agents.

---

## 1. Contexte Projet

**À utiliser dans tous les agents et commandes au lieu de répéter ces informations.**

```yaml
Projet: BuzzControl
Répertoire: /home/user/BuzzMaster
Repository: https://github.com/CCoupel/BuzzMaster

Structure:
  Serveur Go: server-go/
  Frontend React: server-go/web/src/
  Config version: server-go/config.json
  Package frontend: server-go/web/package.json
  Backlog: backlog/*.md
  Documentation: docs/
  Tests: server-go/internal/game/*_test.go

Ports:
  HTTP: 80
  TCP/UDP: 1234
  DNS: 53 (optionnel)

Branches:
  Production: main
  Features: feature/<nom-court>
  Bugfixes: fix/<nom-court>
```

---

## 2. Commandes de Build

### 2.1 Build Complet (Frontend + Backend)

> **RÈGLE CRITIQUE** : Le frontend DOIT être rebuild AVANT le Go build car les fichiers web sont embarqués dans le binaire (mode portable).

```bash
# === BUILD COMPLET ===
# Étape 1: Frontend (OBLIGATOIRE - toujours en premier)
cd server-go/web && npm run build

# Étape 2: Backend Go (embarque le frontend)
cd .. && go build -o server.exe ./cmd/server

# === ONE-LINER ===
cd server-go/web && npm run build && cd .. && go build -o server.exe ./cmd/server
```

### 2.2 Build Cross-Platform

```bash
# Windows AMD64 (développement local)
go build -o server.exe ./cmd/server

# Linux ARM64 (Raspberry Pi 4)
GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o buzzcontrol ./cmd/server

# Linux ARM6 (Raspberry Pi Zero)
GOOS=linux GOARCH=arm GOARM=6 go build -ldflags="-s -w" -o buzzcontrol-arm6 ./cmd/server
```

### 2.3 Validation du Build

```bash
# Vérifier la taille (doit être > 5MB pour le portable)
ls -lh server.exe
# Attendu: ~25-30MB (inclut frontend embarqué)
```

---

## 3. Contrôle du Serveur

### 3.1 Arrêt Gracieux

> **RÈGLE** : Toujours utiliser l'API `/shutdown`, jamais de kill forcé.

```bash
# Arrêt via API (méthode standard)
curl -s http://localhost/shutdown

# Attendre l'arrêt complet
sleep 2
```

### 3.2 Séquence Redémarrage Complète

```bash
# === REDÉMARRAGE STANDARD ===
curl -s http://localhost/shutdown
sleep 2
cd server-go && ./server.exe

# === AVEC REBUILD (après modifications) ===
curl -s http://localhost/shutdown
sleep 2
cd server-go/web && npm run build && cd .. && go build -o server.exe ./cmd/server && ./server.exe
```

### 3.3 Vérification Post-Démarrage

```bash
# Vérifier la version
curl -s http://localhost/version

# Vérifier le fonctionnement
curl -s http://localhost/listGame

# Test complet (version + fonctionnement)
curl -s http://localhost/version && echo "" && curl -s http://localhost/listGame | head -c 100
```

### 3.4 Gestion Version Mismatch

Si `/version` ne correspond pas à `config.json` :

```bash
# 1. Arrêter
curl -s http://localhost/shutdown && sleep 2

# 2. Rebuild complet (frontend + backend)
cd server-go/web && npm run build && cd .. && go build -o server.exe ./cmd/server

# 3. Redémarrer et vérifier
./server.exe &
sleep 3
curl -s http://localhost/version
```

> **Maximum 2 tentatives** avant escalade à l'utilisateur.

---

## 4. Commandes de Test

### 4.1 Tests Unitaires

```bash
# Tous les tests avec couverture
cd server-go && go test ./... -v -cover

# Tests d'un package spécifique
go test ./internal/game/... -v

# Tests avec rapport
go test ./... -v -cover 2>&1 | tee test-report-$(date +%Y%m%d).txt
```

### 4.2 Rapport de Couverture HTML

```bash
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

### 4.3 Validation Tests (CI/Pré-déploiement)

```bash
# Vérifier qu'aucun test n'échoue
go test ./... 2>&1 | grep -c "FAIL"
# Attendu: 0
```

---

## 5. Gestion des Versions

### 5.1 Fichiers de Version

| Fichier | Champ | Usage |
|---------|-------|-------|
| `server-go/config.json` | `"version"` | Source de vérité |
| `server-go/web/package.json` | `"version"` | Doit être synchronisé |

### 5.2 Règles de Versionnement

```
Format: X.Y.Z (SemVer)

X (major): Breaking changes, migrations nécessaires
Y (minor): Nouvelles features (incrémenté par PLAN)
Z (patch): Bugfixes, corrections (incrémenté par DEV)

Exemple:
- Feature nouvelle: 2.45.0 → 2.46.0
- Bugfix: 2.46.0 → 2.46.1
- Release finale: 2.46.3 → 2.46.0 (reset Z)
```

### 5.3 Incrémenter la Version

```bash
# Lire la version actuelle
cat server-go/config.json | grep '"version"'

# Après modification manuelle, vérifier la synchronisation
cat server-go/config.json | grep '"version"'
cat server-go/web/package.json | grep '"version"'
```

### 5.4 Synchronisation Version (PROD)

Pour une release, synchroniser les deux fichiers ET remettre Z à 0 :

```bash
# config.json: "version": "2.46.3" → "version": "2.46.0"
# package.json: "version": "2.46.3" → "version": "2.46.0"
```

---

## 6. Opérations Git

### 6.1 Création de Branche Feature

```bash
git checkout main
git pull origin main
git checkout -b feature/<nom-court>
# Incrémenter Y dans config.json
git add server-go/config.json
git commit -m "chore(version): Start vX.Y.0 - <feature name>"
git push -u origin feature/<nom-court>
```

### 6.2 Commit Atomique (Style)

```bash
# Format du message
<type>(<scope>): <description courte>

# Types valides
feat:     Nouvelle fonctionnalité
fix:      Correction de bug
docs:     Documentation uniquement
chore:    Maintenance, config
refactor: Refactoring sans changement fonctionnel
test:     Ajout/modification de tests
style:    Formatage, pas de changement de code

# Exemples
feat(memory): Add multi-team support
fix(qcm): Correct score calculation for hints
docs(changelog): Update for v2.46.0
chore(version): Bump to 2.46.1
```

### 6.3 Squash Merge (PROD)

```bash
git checkout main
git pull origin main
git merge --squash feature/<branch>
git commit -m "feat: <description> (v2.X.0)"
git push origin main
```

### 6.4 Tag et Release

```bash
# Créer le tag annoté
git tag -a v2.X.0 -m "Release v2.X.0 - <description>"
git push origin v2.X.0

# NE PAS supprimer la branche (gardée pour rollback)
```

---

## 7. Checklists Communes

### 7.1 Checklist Fin de Session DEV

```markdown
- [ ] Code compilé sans erreur (go build)
- [ ] Tests unitaires passés (go test ./... -v)
- [ ] Version incrémentée (Z pour bugfix)
- [ ] Commits atomiques avec messages clairs
- [ ] Pas de fichiers temporaires (*.bak, nul, server-output.txt)
- [ ] Push effectué
```

### 7.2 Checklist Pré-QUALIF

```markdown
- [ ] Build frontend réussi (npm run build)
- [ ] Build backend réussi (go build)
- [ ] Tests 100% passés (0 FAIL)
- [ ] Serveur redémarré et opérationnel
- [ ] Version /version correspond à config.json
- [ ] /listGame retourne un JSON valide
```

### 7.3 Checklist Pré-PROD

```markdown
- [ ] QUALIF validée
- [ ] Review code approuvée
- [ ] CHANGELOG.md mis à jour
- [ ] CLAUDE.md mis à jour (si nouvelles features)
- [ ] Version Z remise à 0
- [ ] package.json synchronisé
- [ ] Build Windows + ARM64 réussis
- [ ] Taille binaires > 5MB (mode portable)
```

### 7.4 Checklist Post-PROD

```markdown
- [ ] Squash merge vers main effectué
- [ ] Tag Git créé et pushé
- [ ] CI GitHub Actions passée
- [ ] Release GitHub créée avec binaires
- [ ] Exécutable release validé localement
- [ ] Branche feature conservée (rollback)
```

---

## 8. Nettoyage

### 8.1 Fichiers Temporaires à Supprimer

```bash
# Fichiers de développement
rm -f nul server-output.txt server-error.txt
rm -f test-report.txt test-summary.txt
rm -f *.bak web/src/pages/*.bak

# Fichiers de couverture
rm -f coverage.out coverage.html

# Build artifacts (si nécessaire)
rm -f server.exe buzzcontrol buzzcontrol-arm6
```

---

## 9. Patterns de Workflow

### 9.1 Workflow Feature

```
/plan → PLAN → DEV-BACKEND → DEV-FRONTEND → REVIEW → QA → DOC → DEPLOY(QUALIF) → DEPLOY(PROD)
        │        ↓              ↓
        │    (parallélisable si pas de dépendances)
        └── Incrémente Y
```

### 9.2 Workflow Bugfix

```
/bugfix → CDP → DEV → REVIEW → QA → DEPLOY(QUALIF)
                │
                └── Incrémente Z
```

### 9.3 Workflow Hotfix (Urgence)

```
/hotfix → DEV → QA → DEPLOY(PROD)
          │
          └── Incrémente Z, pas de QUALIF
```

---

## 10. Dispatch Automatique Backend/Frontend

### 10.1 Critères de Routage

| Critère | Agent |
|---------|-------|
| Fichiers `*.go`, `internal/`, `cmd/` | dev-backend |
| Fichiers `*.jsx`, `*.css`, `web/src/` | dev-frontend |
| Nouveaux endpoints HTTP | dev-backend → dev-frontend |
| Nouvelles actions WebSocket | dev-backend → dev-frontend |
| Nouveaux champs GameState | dev-backend → dev-frontend |
| Refactoring CSS isolé | dev-frontend seul |
| Tests Go uniquement | dev-backend seul |

### 10.2 Ordre d'Exécution

- **Séquentiel** (Backend → Frontend) : Si nouvelles APIs, WebSocket, ou GameState
- **Parallèle** : Si modifications isolées sans dépendances

---

## 11. Référence Rapide

### Commandes Essentielles

| Action | Commande |
|--------|----------|
| Build complet | `cd server-go/web && npm run build && cd .. && go build -o server.exe ./cmd/server` |
| Restart serveur | `curl -s http://localhost/shutdown && sleep 2 && ./server.exe` |
| Tests | `go test ./... -v -cover` |
| Version | `curl -s http://localhost/version` |
| Fonctionnement | `curl -s http://localhost/listGame` |

### Fichiers Clés

| Fichier | Rôle |
|---------|------|
| `server-go/config.json` | Version (source de vérité) |
| `server-go/internal/game/models.go` | Modèles de données |
| `server-go/internal/game/engine.go` | Logique de jeu |
| `server-go/web/src/pages/PlayerDisplay.jsx` | Affichage TV |
| `CHANGELOG.md` | Historique des versions |
| `CLAUDE.md` | Documentation projet |

---

## Usage

**Dans les commandes et agents**, au lieu de répéter le contexte projet :

```markdown
# Avant (répété 17+ fois)
**Contexte projet :**
- Répertoire : /home/user/BuzzMaster
- Serveur Go : server-go/
- Frontend React : server-go/web/src/
- Config version : server-go/config.json

# Après (référence unique)
**Contexte projet :** Voir `.claude/context/COMMON.md` section 1
**Build :** Voir `.claude/context/COMMON.md` section 2
**Tests :** Voir `.claude/context/COMMON.md` section 4
```
