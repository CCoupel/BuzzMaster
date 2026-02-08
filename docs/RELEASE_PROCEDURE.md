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

### 12. Surveiller la CI (Claude)

> **🤖 Responsabilité Claude** : Cette étape est automatiquement effectuée par Claude.
> Claude doit attendre la fin du pipeline CI et vérifier que tous les jobs sont verts (✅).

**URL** : https://github.com/CCoupel/BuzzMaster/actions

Le pipeline s'exécute en 3 étapes :

```
┌─────────────────────────────────────────────┐
│  🔍 Checking (~10s)                         │
│  └─ Vérifie versions config/package/tag     │
└─────────────────────┬───────────────────────┘
                      │
        ┌─────────────┼─────────────┐
        ▼             ▼             ▼
┌──────────────┐ ┌──────────────┐ ┌──────────────────┐
│ 🔨 Compile   │ │ 🔨 Compile   │ │ 🔨 Compile       │
│   Windows    │ │   Linux      │ │   Firmware       │
│  (~1-2 min)  │ │  (~1-2 min)  │ │  (~1-2 min)      │
│ ├─ npm build │ │ ├─ npm build │ │ ├─ install pio   │
│ ├─ go build  │ │ ├─ go build  │ │ ├─ inject version│
│ └─ validate  │ │ └─ validate  │ │ ├─ pio build     │
│    >5MB      │ │    >5MB      │ │ └─ validate      │
│              │ │              │ │    200KB-2MB     │
└──────┬───────┘ └──────┬───────┘ └────────┬─────────┘
       └────────────────┼──────────────────┘
                        ▼
┌─────────────────────────────────────────────┐
│  🚀 Releasing (~30s)                        │
│  ├─ Télécharge les 3 binaires               │
│  │  • buzzcontrol-windows-amd64.exe         │
│  │  • buzzcontrol-linux-arm64               │
│  │  • buzzclick-firmware.bin                │
│  ├─ Extrait notes du CHANGELOG              │
│  └─ Publie la release GitHub                │
└─────────────────────────────────────────────┘
```

**Durée totale** : ~3-4 minutes (builds en parallèle)

**Claude doit** :
1. Attendre la fin complète du pipeline (~3-4 minutes)
2. Vérifier que les 4 jobs sont verts (✅) : checking, compiling x2, compiling-firmware
3. En cas d'échec, analyser les logs et informer l'utilisateur

**En cas d'échec** :
1. Cliquer sur le job en erreur
2. Lire les logs pour identifier le problème
3. Voir section "En Cas de Problème" ci-dessous

---

### 13. Vérifier la Release (Claude)

> **🤖 Responsabilité Claude** : Cette étape est automatiquement effectuée par Claude.
> Claude doit vérifier que la release est bien disponible sur GitHub avec tous les binaires.

**URL** : https://github.com/CCoupel/BuzzMaster/releases

**Claude doit vérifier** :
1. La release `vX.Y.0` existe
2. Les 3 binaires sont attachés :
   - `buzzcontrol-vX.Y.0-windows-amd64.exe` (~8-9 MB)
   - `buzzcontrol-vX.Y.0-linux-arm64` (~8 MB)
   - `buzzclick-vX.Y.0-firmware.bin` (~500KB-1MB)
3. Les notes de release sont extraites du CHANGELOG
4. Informer l'utilisateur du succès ou de l'échec

---

### 14. Relancer le Serveur (Claude)

> **🤖 Responsabilité Claude** : Cette étape est automatiquement effectuée par Claude.
> Claude doit relancer le serveur local avec la version de production.

```bash
# Arrêter le serveur en cours
curl -s http://localhost/shutdown

# Attendre l'arrêt
sleep 2

# Relancer le serveur
cd server-go && ./server.exe &

# Vérifier la version
curl -s http://localhost/version
```

**Claude doit vérifier** :
1. Le serveur répond à `/version`
2. La version retournée correspond à la release (`X.Y.0`)
3. Informer l'utilisateur que le serveur est opérationnel

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
[ ] 12. 🤖 CI surveillée par Claude (3 jobs verts ✅)
[ ] 13. 🤖 Release vérifiée par Claude (binaires + notes)
[ ] 14. 🤖 Serveur relancé par Claude (version vérifiée)
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
