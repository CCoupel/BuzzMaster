# Métadonnées dans les binaires

**Statut** : 📋 Planifié

## Description

Ajouter des métadonnées (nom du produit, version, description) dans les binaires exécutables Windows (.exe) et Linux pour une meilleure identification et traçabilité.

## Objectifs

- [ ] Métadonnées Windows PE (visible dans Propriétés > Détails)
- [ ] Version embarquée via ldflags (Windows + Linux)
- [ ] Automatisation dans le workflow CI

## Tâches

### Phase 1 - Windows PE Metadata

Utiliser `goversioninfo` pour générer un fichier `.syso` avec les infos Windows.

- [ ] Installer `goversioninfo` dans le workflow CI
- [ ] Créer `cmd/server/versioninfo.json` avec template
- [ ] Créer/trouver une icône `assets/icon.ico`
- [ ] Modifier le script de build pour générer le `.syso`
- [ ] Vérifier les métadonnées dans Propriétés Windows

**Fichier versioninfo.json :**
```json
{
  "FixedFileInfo": {
    "FileVersion": {
      "Major": 2,
      "Minor": 46,
      "Patch": 0,
      "Build": 0
    },
    "ProductVersion": {
      "Major": 2,
      "Minor": 46,
      "Patch": 0,
      "Build": 0
    }
  },
  "StringFileInfo": {
    "ProductName": "BuzzControl",
    "ProductVersion": "2.46.0",
    "FileDescription": "Wireless Buzzer System for Quiz Games",
    "CompanyName": "CCoupel",
    "LegalCopyright": "2026 CCoupel",
    "OriginalFilename": "buzzcontrol.exe"
  },
  "IconPath": "../../assets/icon.ico"
}
```

**Commandes build :**
```bash
# Générer le .syso
goversioninfo -o cmd/server/resource.syso cmd/server/versioninfo.json

# Build Windows
go build -o server.exe ./cmd/server
```

### Phase 2 - Version embarquée (ldflags)

Injecter la version au moment du build pour Windows ET Linux.

- [ ] Ajouter variables dans `cmd/server/main.go`
- [ ] Modifier les scripts de build (local + CI)
- [ ] Afficher la version au démarrage du serveur
- [ ] Endpoint `/version` retourne les infos complètes

**Variables Go :**
```go
// cmd/server/main.go
var (
    Version     = "dev"
    ProductName = "BuzzControl"
    BuildTime   = "unknown"
    GitCommit   = "unknown"
)
```

**Commande build :**
```bash
VERSION=$(cat server-go/config.json | jq -r '.version')
COMMIT=$(git rev-parse --short HEAD)
BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

go build -ldflags "\
  -X main.Version=$VERSION \
  -X main.ProductName=BuzzControl \
  -X 'main.BuildTime=$BUILD_TIME' \
  -X main.GitCommit=$COMMIT" \
  -o server.exe ./cmd/server
```

### Phase 3 - Intégration CI

- [ ] Modifier `.github/workflows/release.yml`
- [ ] Installer `goversioninfo` dans le job Windows
- [ ] Générer `versioninfo.json` dynamiquement depuis le tag
- [ ] Ajouter ldflags au build Linux ARM64
- [ ] Tester avec une release de test

**Workflow CI modifié :**
```yaml
- name: Install goversioninfo
  run: go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest

- name: Generate version info
  run: |
    VERSION=${GITHUB_REF#refs/tags/v}
    # Générer versioninfo.json avec la version du tag
    goversioninfo -o cmd/server/resource.syso cmd/server/versioninfo.json

- name: Build with ldflags
  run: |
    go build -ldflags "-X main.Version=$VERSION -X main.GitCommit=$GITHUB_SHA" ...
```

## Résultat attendu

### Windows - Propriétés du fichier
```
Nom du produit : BuzzControl
Version du fichier : 2.46.0.0
Description : Wireless Buzzer System for Quiz Games
Copyright : 2026 CCoupel
```

### Linux - Démarrage serveur
```
BuzzControl v2.46.0 (commit: abc1234, built: 2026-01-31T14:00:00Z)
Starting server on :80...
```

### Endpoint /version (amélioré)
```json
{
  "version": "2.46.0",
  "product": "BuzzControl",
  "commit": "abc1234",
  "build_time": "2026-01-31T14:00:00Z"
}
```

## Fichiers concernés

| Fichier | Modification |
|---------|--------------|
| `cmd/server/main.go` | Variables Version, ProductName, BuildTime, GitCommit |
| `cmd/server/versioninfo.json` | Nouveau - Template métadonnées Windows |
| `assets/icon.ico` | Nouveau - Icône Windows |
| `.github/workflows/release.yml` | Ajout goversioninfo + ldflags |
| `build.ps1` / `build.sh` | Ajout ldflags pour build local |

## Version cible

v2.47.0
