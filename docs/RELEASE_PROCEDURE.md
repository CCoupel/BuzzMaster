# Procédure de Mise en Production

Ce document décrit la procédure complète pour publier une nouvelle version de BuzzControl.

> **Note** : Le build et la publication des binaires sont automatisés via GitHub Actions.
> Le workflow se déclenche automatiquement lors du push d'un tag `v*`.

---

## Prérequis

- [ ] Git configuré avec accès GitHub
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

**IMPORTANT** : Les deux fichiers doivent avoir la même version.

**Fichier 1** : `server-go/config.json`
```json
{
  "version": "x.y.0",
  ...
}
```

**Fichier 2** : `server-go/web/package.json`
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

> **Note** : Le contenu de cette section sera automatiquement extrait pour les notes de release GitHub.

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

---

### 7. Mise à Jour du Backlog

**Fichier** : `BACKLOG.md`

- [ ] Marquer les tâches terminées avec `[x]` et la version
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

### 9. Commit des Changements

```bash
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

### 10. Push des Commits

```bash
git push origin main
```

---

### 11. Créer et Pousser le Tag de Version

```bash
# Créer le tag annoté
git tag -a vX.Y.0 -m "Release vX.Y.0"

# Pousser le tag (déclenche le build CI)
git push origin vX.Y.0
```

> **🚀 À ce moment, GitHub Actions se déclenche automatiquement et :**
> 1. Vérifie que les versions correspondent (config.json, package.json, tag)
> 2. Build le frontend React
> 3. Compile les binaires Windows et Linux ARM64
> 4. Valide l'intégrité des binaires (taille > 5MB)
> 5. Crée la release GitHub avec les binaires attachés
> 6. Extrait les notes de release depuis CHANGELOG.md

---

### 12. Vérifier la Release

1. Aller sur : https://github.com/CCoupel/BuzzMaster/actions
2. Vérifier que le workflow "Build and Release" est en succès ✅
3. Aller sur : https://github.com/CCoupel/BuzzMaster/releases
4. Vérifier que la release contient :
   - `buzzcontrol-vX.Y.0-windows-amd64.exe`
   - `buzzcontrol-vX.Y.0-linux-arm64`
   - Notes de release extraites du CHANGELOG

---

## Checklist Récapitulative

```
[ ] 1.  Validation utilisateur obtenue
[ ] 2.  Fichiers temporaires nettoyés
[ ] 3.  Version mise à jour (config.json + package.json)
[ ] 4.  CHANGELOG.md mis à jour
[ ] 5.  CLAUDE.md mis à jour (documentation technique)
[ ] 6.  README.md mis à jour (si nécessaire)
[ ] 7.  Backlog mis à jour
[ ] 8.  Documentation utilisateur mise à jour (si nécessaire)
[ ] 9.  Changements commités
[ ] 10. Commits poussés
[ ] 11. Tag créé et poussé (déclenche CI)
[ ] 12. Release vérifiée sur GitHub
```

---

## En Cas de Problème

### Le workflow CI échoue

1. Aller sur https://github.com/CCoupel/BuzzMaster/actions
2. Cliquer sur le workflow en échec
3. Consulter les logs pour identifier l'erreur
4. Corriger, commiter, et recréer le tag

### Erreur "versions don't match"

Le workflow vérifie que `config.json`, `package.json` et le tag ont la même version.

```bash
# Vérifier les versions
grep '"version"' server-go/config.json
grep '"version"' server-go/web/package.json
git describe --tags --abbrev=0
```

### Rollback du tag

```bash
# Supprimer le tag local
git tag -d vX.Y.0

# Supprimer le tag distant (annule aussi la release)
git push origin --delete vX.Y.0
```

### Supprimer une release GitHub

```bash
# Via gh CLI
gh release delete vX.Y.0 --yes

# Ou via l'interface web : Releases > vX.Y.0 > Delete
```

---

## Build Local (optionnel)

Si besoin de tester le build localement avant le tag :

```bash
cd server-go

# Frontend
npm run build --prefix web
cp -r web/dist cmd/server/dist

# Windows
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o releases/test-windows.exe ./cmd/server

# Linux ARM64
GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o releases/test-linux ./cmd/server

# Vérifier les tailles (>5MB)
ls -lh releases/
```

---

## Notes

- Les exécutables portables embarquent le site web (pas besoin de fichiers séparés)
- Le dossier `data/` est créé automatiquement à côté de l'exécutable
- Sur Raspberry Pi, donner les droits d'exécution : `chmod +x buzzcontrol-linux-arm64`
- Le workflow CI utilise Go 1.21 et Node.js 18
