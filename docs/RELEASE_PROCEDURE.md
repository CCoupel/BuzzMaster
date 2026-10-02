# Procédure de Mise en Production

Ce document décrit la procédure complète pour publier une nouvelle version de BuzzControl.

> **Note** : Le build et la publication des binaires sont automatisés via GitHub Actions.
> Le workflow se déclenche automatiquement lors du push d'un tag `v*`.

---

## Prérequis

- [ ] Git configuré avec accès GitHub
- [ ] Accès en écriture au dépôt

---

## Règles de Versionnement

> **Note de migration — 2026-08-07.** L'ancienne règle propre au projet (3 segments :
> `x` = architecture, `y` = fonctionnalité, `z` = toujours 0 en release) est **abandonnée** au
> profit du schéma à 4 segments décrit ci-dessous, repris de
> `.claude/commands/context/COMMON.md` §5 **à une exception près, la parité de `Y`** (voir
> l'encadré ci-dessous). Les versions publiées avant cette date (jusqu'à **v6.1.1** incluse)
> suivent l'ancienne règle : voir `CHANGELOG.md` pour l'historique.

### Format

```
Format dev  : X.Y.Z.a     (4 segments)
Format prod : X.Y.Z       (3 segments — le « a » n'est jamais publié)
```

| Segment | Rôle |
|---------|------|
| **X** | Compatibilité des **données** (fichiers de questions, équipes, état persisté). Une rupture reste upgradable, mais le rollback devient compliqué. |
| **Y** | Compteur de milestone/livraison. **Pair = milestone/dev, impair = release/prod.** Avance toujours de +1, jamais de +2. |
| **Z** | Compteur de bugfix. Remis à 0 uniquement au démarrage d'un nouveau milestone. |
| **a** | Itération de développement interne — un commit de cycle = un `a`. Jamais visible en production. |

> ⚠️ **X ne signifie plus « changement d'architecture ».** Sous l'ancienne règle, l'arrivée d'un
> nouveau sous-système justifiait un `X+1` (c'est ce qui a produit le passage 5 → 6 du générateur
> IA). Désormais **seule une rupture de compatibilité des données** incrémente X : une
> fonctionnalité, même volumineuse, avance `Y`.

> ⚠️ **Écart assumé avec le template — parité de `Y`.**
> `.claude/commands/context/COMMON.md` §5 énonce l'inverse : « Impair = dev, pair = prod », et son
> exemple est cohérent avec cet énoncé (dev en 1.1/1.3/1.5, prod en 1.2/1.4/1.6).
> **Ce projet retient la convention opposée** — dev/milestone sur `Y` **pair**, release sur `Y`
> **impair** — sur décision utilisateur, parce qu'elle correspond à la pratique réelle :
> le milestone GitHub **`v6.0`** (Y=0, pair) a produit la release **v6.1.1** (Y=1, impair).
>
> Elle est aussi la seule des deux qui reste cohérente après une rupture de compatibilité des
> données : `X+1` remet `Y=0`, qui est directement un numéro de **dev** valide. Avec la parité du
> template, `Y=0` serait un numéro de prod et il faudrait aussitôt sauter à `Y=1` — ce que
> `COMMON.md` prévoit d'ailleurs explicitement, au prix d'un cas particulier dont on se passe ici.
>
> **Seule la parité change. Toutes les opérations ci-dessous sont identiques à celles du
> template** : les deux conventions décrivent la même mécanique en opposition de phase.
> `COMMON.md` n'a **pas** été modifié — l'écart est ici, documenté et volontaire.

### Les 6 opérations

| Événement | Effet sur la version |
|-----------|----------------------|
| Itération dev (commit de cycle) | `a+1` |
| Nouveau milestone (feature planifiée) | `Y+1 ; Z=0 ; a=0` |
| Nouveau cycle bugfix (aucun milestone actif) | `Z+1 ; a=0` — reprend le `Y` de dev de la dernière vague |
| Hotfix (`/hotfix`) | `Z+1 ; a=0` — **même si un milestone est en cours** |
| Promotion dev → prod | `Y+1 ; Z conservé ; a supprimé` |
| Rupture de compatibilité des données | `X+1 ; Y=0 ; Z=0 ; a=0` — `Y=0` étant pair, c'est **directement** le numéro du prochain milestone (pas de saut, contrairement au template) |

### Règle d'or — bug remonté

- **Milestone en cours** : le correctif est intégré aux itérations du milestone (`a+1`, `Z` ne
  bouge pas). La prochaine prod livre le `Y` du milestone.
- **Aucun milestone en cours** : nouveau cycle bugfix créé depuis le dernier `Y` de dev (même
  vague que la prod courante), `Z+1`.
- **Une version prod déjà dépassée n'est jamais repatchée** — le correctif vise toujours la ligne
  prod courante, jamais une ancienne.

### Où se place une release de correctifs

Le schéma **a bien un emplacement** pour livrer des correctifs après une release de
fonctionnalité, et c'est `Z` : le cycle de correctifs se développe sur la vague `Y` **paire** du
milestone avec `Z+1`, puis sa promotion conserve ce `Z`. Le `Y` de production, lui, avance
normalement.

C'est précisément ce qui manquait à l'ancienne règle, dont le « z toujours 0 en release »
n'offrait aucun numéro pour ce cas — d'où la publication de **v6.1.1** hors règle en 2026-08-07.

**Exemple concret — l'histoire réelle du projet relue sous cette règle** :

```
6.0.0.0 → 6.0.0.1 → …        dev du milestone GitHub « v6.0 » (Y=0, pair)
6.1.0                        promotion prod (Y+1 → impair)         ← tag v6.1.0
6.0.1.0 → 6.0.1.1            correctifs post-release : retour sur la vague dev Y=0, Z+1
6.1.1                        promotion prod, Z conservé            ← tag v6.1.1  (réellement publié)
6.2.0.0                      nouveau milestone « v6.2 » : Y+1 depuis la prod, Z=0, a=0
6.3.0                        promotion prod                        ← tag v6.3.0
```

Chaque opération avance `Y` de +1 depuis le numéro courant : le développement occupe les `Y`
pairs, la production les `Y` impairs, en alternance.

### Milestones GitHub

Un milestone se nomme **`vX.Y`** — sans `Z`, sans `a` : il désigne la vague de livraison, pas un
binaire précis. **`Y` y est toujours pair** (`/milestone new v6.2`), puisqu'un milestone est un
cycle de développement ; la release qui en sort porte le `Y` impair suivant.

### Versions parallèles pendant le développement

Deux branches non mergées **divergent naturellement** sur `a`, et c'est attendu : ces numéros ne
servent qu'à distinguer les binaires de QUALIF successifs. La réconciliation se fait une seule
fois, au merge PROD réel, en fixant explicitement la version finale.

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

### 3. Mise à Jour de la Version (avant tagging)

> **Le tag fait foi pour ce qui est publié.** TOUS les fichiers de version doivent être bumpes
> et committes **avant de créer le tag**. La CI ne modifie aucun fichier : elle vérifie seulement
> que tous les fichiers de version correspondent au tag
> (`.github/workflows/release.yml`, job « checking »). Si une incohérence est détectée, le
> workflow échoue et il faut corriger les fichiers, recommitter, puis recréer le tag.

**Fichier 1** : `server-go/config.json`
```json
{
  "version": "X.Y.Z",
  ...
}
```

**Fichier 2** : `server-go/web/package.json`
```json
{
  "version": "X.Y.Z",
  ...
}
```

**Fichier 3** : `server-go/internal/server/config.json`
```json
{
  "version": "X.Y.Z",
  ...
}
```

**Fichier 4** : `platformio.ini` (firmware version — **critique**)

Le firmware doit être bumpe **manuellement** à la version projet avant chaque tag :
```ini
build_flags =
  -D VERSION='\"X.Y.Z\"'
  -D FIRMWARE_VERSION='\"X.Y.Z\"'
```

> **Important** : Contrairement aux versions serveur précédentes, la CI n'injecte plus cette version.
> Elle doit correspondre exactement au tag `vX.Y.Z`.

**Fichier 5** : `server-go/assets/firmware/version.txt` (versionné — artefact suivi)

Fichier texte contenant la version du firmware embarqué (compatible avec suffixe de build `.N`) :
```
X.Y.Z
```

> **Tolérance** : le fichier tolère un suffixe de build `.N` (e.g., `11.1.0.14`), mais la partie
> `X.Y.Z` doit correspondre au tag. Exemple : pour le tag `v11.1.0`, le fichier peut contenir
> `11.1.0`, `11.1.0.1`, `11.1.0.99`, etc. — seul le `X.Y.Z` est vérifié par la CI.

**Fichier 6** : `server-go/cmd/server/versioninfo.json` (métadonnées Windows PE)

Mettre à jour les champs suivants pour les builds locaux :
```json
{
  "FixedFileInfo": {
    "FileVersion":    { "Major": X, "Minor": Y, "Patch": Z, "Build": 0 },
    "ProductVersion": { "Major": X, "Minor": Y, "Patch": Z, "Build": 0 }
  },
  "StringFileInfo": {
    "FileVersion":    "X.Y.Z.0",
    "ProductVersion": "X.Y.Z"
  }
}
```

> **Note** : La CI régénère `versioninfo.json` au moment du build Windows. Cette mise à jour manuelle
> sert uniquement pour les builds locaux via `build.ps1`. Le `.syso` généré ne doit pas être commité.

---

> **Quel numéro choisir ?** Voir [Règles de Versionnement](#règles-de-versionnement) ci-dessus.

---

### 4. Mise à Jour du CHANGELOG

**Fichier** : `CHANGELOG.md`

Ajouter une nouvelle section en haut du fichier :

```markdown
## [X.Y.Z] - YYYY-MM-DD

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
release: BuzzControl vX.Y.Z

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
git tag -a vX.Y.Z -m "Release vX.Y.Z"

# Pousser le tag (déclenche le build CI)
git push origin vX.Y.Z
```

> **🚀 À ce moment, GitHub Actions se déclenche automatiquement et :**
> 1. **Vérifie** que tous les fichiers de version correspondent au tag (`config.json`,
>    `internal/server/config.json`, `package.json`, `platformio.ini`, `version.txt`) — échoue
>    avec rapport détaillé si incohérence
> 2. Build le frontend React
> 3. Compile les binaires Windows, Linux ARM64 et firmware BuzzClick en parallèle
> 4. Valide l'intégrité des binaires (serveur > 5MB, firmware 200KB–3MB)
> 5. Crée la release GitHub avec tous les artefacts et SHA256SUMS
> 6. Extrait les notes de release depuis CHANGELOG.md

---

### 12. Surveiller la CI (Claude)

> **🤖 Responsabilité Claude** : Cette étape est automatiquement effectuée par Claude.
> Claude doit attendre la fin du pipeline CI et vérifier que tous les jobs sont verts (✅).

**URL** : https://github.com/CCoupel/BuzzMaster/actions

Le pipeline s'exécute en 4 étapes (testing + checking + compiling x2 + releasing) :

**Vérifications bloquantes avant build** :
1. ✅ **Tests unitaires** (`go test -race`) — tous les tests doivent passer
2. ✅ **Go vet** — détecte les erreurs courantes dans le code Go
3. ✅ **Govulncheck** — scan des vulnérabilités connues dans les dépendances
4. ✅ **Vérification tag == version** — le tag doit correspondre exactement à la version du dépôt

En cas d'échec de l'une de ces vérifications, la release est bloquée et l'utilisateur doit corriger le problème avant de retenter.

```
┌─────────────────────────────────────────────────────────┐
│  🧪 Testing (~10-15 min)                                │
│  └─ Tests unitaires, race detector, build frontend      │
└─────────────────────┬───────────────────────────────────┘
                      │
┌─────────────────────────────────────────────────────────┐
│  🔍 Checking (~10s)                                     │
│  └─ Vérifie cohérence tag vs fichiers de version       │
└─────────────────────┬───────────────────────────────────┘
                      │
        ┌─────────────┼─────────────┐
        ▼             ▼             ▼
┌──────────────┐ ┌──────────────┐ ┌──────────────────┐
│ 🔨 Compile   │ │ 🔨 Compile   │ │ 🔨 Compile       │
│   Windows    │ │   Linux      │ │   Firmware       │
│  (~1-2 min)  │ │  (~1-2 min)  │ │  (~1-2 min)      │
│ ├─ npm build │ │ ├─ npm build │ │ ├─ install pio   │
│ ├─ go build  │ │ ├─ go build  │ │ ├─ pio build     │
│ └─ validate  │ │ └─ validate  │ │ └─ validate      │
│    >5MB      │ │    >5MB      │ │    200KB-3MB     │
│              │ │              │ │                  │
└──────┬───────┘ └──────┬───────┘ └────────┬─────────┘
       └────────────────┼──────────────────┘
                        ▼
┌─────────────────────────────────────────────────────────┐
│  🚀 Releasing (~30s)                                    │
│  ├─ Télécharge les 3 binaires                           │
│  │  • buzzcontrol-vX.Y.Z-windows-amd64.exe (~8-9 MB)   │
│  │  • buzzcontrol-vX.Y.Z-linux-arm64 (~8 MB)           │
│  │  • buzzclick-vX.Y.Z-merged.bin (~500KB-1MB)         │
│  ├─ Génère SHA256SUMS                                   │
│  ├─ Extrait notes du CHANGELOG                          │
│  └─ Publie la release GitHub                            │
└─────────────────────────────────────────────────────────┘
```

**Durée totale** : ~3-4 minutes (builds en parallèle)

**Claude doit** :
1. Attendre la fin complète du pipeline (~3-4 minutes)
2. Vérifier que les 6 jobs sont verts (✅) : checking, testing, compiling-firmware, compiling (Windows),
   compiling (Linux), releasing
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
1. La release `vX.Y.Z` existe
2. Les 4 artefacts sont attachés :
   - `buzzcontrol-vX.Y.Z-windows-amd64.exe` (~8-9 MB)
   - `buzzcontrol-vX.Y.Z-linux-arm64` (~8 MB)
   - `buzzclick-vX.Y.Z-merged.bin` (~500KB-1MB) — firmware merged (supports USB + OTA)
   - `SHA256SUMS` — checksums pour tous les binaires
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
2. La version retournée correspond à la release (`X.Y.Z`)
3. Informer l'utilisateur que le serveur est opérationnel

---

## Checklist Récapitulative

```
[ ] 1.  Validation utilisateur obtenue
[ ] 2.  Fichiers temporaires nettoyés
[ ] 3.  Version mise à jour (config.json + package.json + internal/server/config.json +
        platformio.ini + server-go/assets/firmware/version.txt + versioninfo.json)
        → numéro de prod à 3 segments, le « a » du cycle dev est retiré
        → firmware bumpe manuellement dans platformio.ini (VERSION et FIRMWARE_VERSION)
[ ] 4.  CHANGELOG.md mis à jour
[ ] 5.  CLAUDE.md mis à jour (documentation technique)
[ ] 6.  README.md mis à jour (si nécessaire)
[ ] 7.  Backlog mis à jour
[ ] 8.  Documentation utilisateur mise à jour (si nécessaire)
[ ] 9.  Changements commités
[ ] 10. Commits poussés
[ ] 11. Tag créé et poussé (déclenche CI)
[ ] 12. 🤖 CI surveillée par Claude (6 jobs verts ✅ : checking, testing, compiling-firmware,
        compiling Windows, compiling Linux, releasing)
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

### La version publiée ne correspond pas à celle attendue

Le workflow ne **compare** rien : il prend la version du tag et l'écrit dans les fichiers avant de
compiler. Une version inattendue dans la release vient donc du **tag**, pas des fichiers commités.

```bash
# Ce que porte le tag (fait foi pour la release)
git describe --tags --abbrev=0

# Ce que portent les fichiers (builds locaux uniquement)
grep '"version"' server-go/config.json
grep '"version"' server-go/web/package.json
```

Correction : supprimer le tag (voir ci-dessous), puis le recréer avec le bon numéro. Un tag à
4 segments (`v6.1.0.0`) produirait une release étiquetée avec un `a` — **le `a` ne se tague
jamais**, il est retiré à la promotion.

### Rollback du tag

```bash
# Supprimer le tag local
git tag -d vX.Y.Z

# Supprimer le tag distant (annule aussi la release)
git push origin --delete vX.Y.Z
```

### Supprimer une release GitHub

```bash
# Via gh CLI
gh release delete vX.Y.Z --yes

# Ou via l'interface web : Releases > vX.Y.Z > Delete
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
- Le workflow CI utilise Go 1.25 et Node.js 18
- Le firmware compilé (`buzzclick-vX.Y.Z-merged.bin`) est automatiquement embarqué dans le serveur pour les mises à jour OTA
