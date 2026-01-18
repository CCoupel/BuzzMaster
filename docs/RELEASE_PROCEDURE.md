# Procédure de Mise en Production

Ce document décrit la procédure complète pour publier une nouvelle version de BuzzControl.

---

## Prérequis

- [ ] Go 1.21+ installé
- [ ] Node.js 18+ installé
- [ ] Git configuré avec accès Bitbucket
- [ ] Accès en écriture au dépôt

---

## Procédure Complète

### 1. Validation Utilisateur

- [ ] L'utilisateur a validé les fonctionnalités
- [ ] Tous les bugs critiques sont corrigés
- [ ] L'interface fonctionne correctement (admin + TV)

---

### 2. Nettoyage des Fichiers Temporaires

Supprimer les fichiers de debug/test avant le commit :

```bash
# Fichiers à vérifier/supprimer
rm -f server-go/nul
rm -f server-go/server-output.txt
rm -f server-go/server-error.txt
rm -f server-go/test-report.txt
rm -f server-go/test-summary.txt
rm -f server-go/*.bak
rm -f server-go/web/src/pages/*.bak

# Vérifier qu'il n'y a pas de fichiers indésirables
git status
```

---

### 3. Mise à Jour de la Version

**Fichier** : `server-go/config.json`

```json
{
  "version": "x.y.0",
  ...
}
```

**Règles de versionnement** :
- **x** (majeur) : Changement d'architecture ou breaking change
- **y** (mineur) : Nouvelle fonctionnalité
- **z** : Toujours 0 pour une release (utilisé uniquement en dev)

---

### 4. Mise à Jour du CHANGELOG

**Fichier** : `CHANGELOG.md`

Ajouter une nouvelle section en haut du fichier :

```markdown
## [x.y.0] - YYYY-MM-DD

### Ajouté
- Nouvelle fonctionnalité 1
- Nouvelle fonctionnalité 2

### Modifié
- Amélioration 1
- Amélioration 2

### Corrigé
- Bug fix 1
- Bug fix 2
```

---

### 5. Mise à Jour de la Documentation Technique

**Fichier** : `CLAUDE.md`

Mettre à jour les sections pertinentes :
- [ ] Nouvelles fonctionnalités implémentées
- [ ] Décisions d'architecture
- [ ] Nouveaux endpoints API
- [ ] Nouveaux composants UI
- [ ] Modifications du protocole

---

### 6. Mise à Jour du README

**Fichier** : `README.md`

Mettre à jour si nécessaire :
- [ ] Nouvelles fonctionnalités dans la liste
- [ ] Nouvelles captures d'écran
- [ ] Instructions d'installation modifiées
- [ ] Liens vers nouvelle documentation
- [ ] Version dans les exemples de configuration

---

### 7. Mise à Jour du Backlog

**Fichier** : `docs/BACKLOG.md` (ou fichier équivalent)

- [ ] Déplacer les tâches terminées vers la section DONE
- [ ] Mettre à jour les priorités restantes
- [ ] Ajouter les nouvelles idées/demandes

---

### 8. Mise à Jour de la Documentation Utilisateur

**Fichier** : `docs/ADMIN_GUIDE.md`

Si nécessaire, mettre à jour :
- [ ] Nouvelles fonctionnalités utilisateur
- [ ] Captures d'écran
- [ ] Procédures modifiées

---

### 9. Lancer les Tests

```bash
cd server-go

# Tests Go
go test ./... -v

# Vérifier qu'il n'y a pas d'erreurs de compilation
go build -o server.exe ./cmd/server
```

---

### 10. Build des Exécutables Portables

#### Build Complet (Windows + Linux ARM64)

```bash
cd server-go

# 1. Build du frontend React
npm run build --prefix web

# 2. Copier dist pour l'embedding
rm -rf cmd/server/dist
cp -r web/dist cmd/server/dist

# 3. Build Windows (AMD64)
GOOS=windows GOARCH=amd64 go build -o releases/buzzcontrol-windows-amd64.exe ./cmd/server

# 4. Build Linux Raspberry Pi (ARM64)
GOOS=linux GOARCH=arm64 go build -o releases/buzzcontrol-linux-arm64 ./cmd/server

# 5. Vérifier les tailles
ls -lh releases/
```

#### Script PowerShell (Windows)

```powershell
# build-release.ps1
$VERSION = (Get-Content config.json | ConvertFrom-Json).version

Write-Host "Building BuzzControl v$VERSION..."

# Frontend
npm run build --prefix web

# Copy dist
Remove-Item -Recurse -Force cmd/server/dist -ErrorAction SilentlyContinue
Copy-Item -Recurse web/dist cmd/server/dist

# Create releases folder
New-Item -ItemType Directory -Force -Path releases | Out-Null

# Windows build
$env:GOOS = "windows"
$env:GOARCH = "amd64"
go build -o "releases/buzzcontrol-v$VERSION-windows-amd64.exe" ./cmd/server

# Linux ARM64 build (Raspberry Pi)
$env:GOOS = "linux"
$env:GOARCH = "arm64"
go build -o "releases/buzzcontrol-v$VERSION-linux-arm64" ./cmd/server

# Reset env
Remove-Item Env:GOOS
Remove-Item Env:GOARCH

Write-Host "Build complete!"
Get-ChildItem releases/
```

---

### 11. Validation des Exécutables Générés

Les tests fonctionnels ayant été réalisés en phase de qualification, cette étape se limite à valider l'intégrité des fichiers générés.

#### Vérifications

```bash
cd server-go/releases

# 1. Vérifier que les fichiers existent et ont une taille raisonnable (>5MB)
ls -lh buzzcontrol-v*

# 2. Vérifier les versions dans config.json et package.json
cat ../config.json | grep '"version"'
cat ../web/package.json | grep '"version"'
```

**Checklist** :
- [ ] Fichier Windows existe et >5MB
- [ ] Fichier Linux ARM64 existe et >5MB
- [ ] Version config.json = x.y.0
- [ ] Version package.json = x.y.0
- [ ] Les deux versions sont identiques

---

### 12. Commit des Changements

```bash
cd /path/to/buzzcontrol

# Ajouter tous les fichiers modifiés
git add .

# Vérifier ce qui va être commité
git status

# Commit avec message descriptif
git commit -m "$(cat <<'EOF'
release: BuzzControl vX.Y.0

## Nouveautés
- Feature 1
- Feature 2

## Corrections
- Fix 1
- Fix 2

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

### 13. Push des Commits

```bash
git push origin main
```

---

### 14. Créer le Tag de Version

```bash
# Créer le tag annoté
git tag -a vX.Y.0 -m "Release vX.Y.0"

# Pousser le tag
git push origin vX.Y.0
```

---

### 15. Publier la Release sur Bitbucket

#### Via l'interface web Bitbucket

1. Aller sur : `https://bitbucket.org/ccoupel/buzzcontrol/downloads/`
2. Cliquer sur "Add files"
3. Uploader les fichiers :
   - `buzzcontrol-vX.Y.0-windows-amd64.exe`
   - `buzzcontrol-vX.Y.0-linux-arm64`
4. Ajouter une description de la release

#### Via l'API Bitbucket (optionnel)

```bash
# Upload Windows exe
curl -X POST \
  -u username:app_password \
  -F files=@releases/buzzcontrol-vX.Y.0-windows-amd64.exe \
  https://api.bitbucket.org/2.0/repositories/ccoupel/buzzcontrol/downloads

# Upload Linux binary
curl -X POST \
  -u username:app_password \
  -F files=@releases/buzzcontrol-vX.Y.0-linux-arm64 \
  https://api.bitbucket.org/2.0/repositories/ccoupel/buzzcontrol/downloads
```

---

## Checklist Récapitulative

```
[ ] 1.  Validation utilisateur obtenue
[ ] 2.  Fichiers temporaires nettoyés
[ ] 3.  Version mise à jour dans config.json
[ ] 4.  CHANGELOG.md mis à jour
[ ] 5.  CLAUDE.md mis à jour (documentation technique)
[ ] 6.  README.md mis à jour (si nécessaire)
[ ] 7.  Backlog mis à jour (DONE)
[ ] 8.  Documentation utilisateur mise à jour (si nécessaire)
[ ] 9.  Tests passés avec succès
[ ] 10. Builds Windows + Linux générés
[ ] 11. Exécutables validés (intégrité + versions)
[ ] 12. Changements commités
[ ] 13. Commits poussés
[ ] 14. Tag de version créé et poussé
[ ] 15. Release publiée sur Bitbucket
```

---

## En Cas de Problème

### Rollback du tag

```bash
# Supprimer le tag local
git tag -d vX.Y.0

# Supprimer le tag distant
git push origin --delete vX.Y.0
```

### Annuler le dernier commit (avant push)

```bash
git reset --soft HEAD~1
```

### Supprimer une release Bitbucket

Via l'interface web : Downloads > Supprimer le fichier

---

## Notes

- Les exécutables portables embarquent le site web (pas besoin de fichiers séparés)
- Le dossier `data/` doit être créé à côté de l'exécutable pour stocker les données
- Sur Raspberry Pi, penser à donner les droits d'exécution : `chmod +x buzzcontrol-linux-arm64`
