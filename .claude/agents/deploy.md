---
name: deploy
description: "Adaptations projet BuzzControl pour l'agent deploy. Base generique : deploy.template.md."
model: sonnet
color: red
---

# Agent Deploy — Adaptations BuzzControl

> **Base** : Voir `deploy.template.md` pour le role, le declenchement, la Tache BUILD generique,
> le protocole d'echec, le rollback et la checklist. La mecanique concrete PUBLISH/DEPLOY par
> environnement vit desormais dans `.claude/agents/environments/{publish,deploy}.{qualif,prod}.template.md`
> (+ compagnons `.md` — adaptations BuzzControl : QUALIF n'a pas de serveur distant, PROD n'a pas
> de plateforme geree par `deployer`, voir ces fichiers). Ce fichier ne contient que les regles
> BUILD specifiques a BuzzControl, qui n'ont pas de fichier d'environnement (BUILD est agnostique).

## BuzzControl — Tâche BUILD

### Ordre de build OBLIGATOIRE (BORE)

Le binaire embarque le firmware BuzzClick (merged) ET le frontend React.
**L'ordre est critique — ne jamais le modifier.**

```bash
# Racine du repo, calculee explicitement — ne jamais deriver de cd relatifs qui se perdent
# au fil des etapes (voir deploy.template.md : les repertoires doivent toujours resoudre a la
# racine, meme en monorepo, meme apres un cd server-go/web pour le build frontend)
REPO_ROOT=$(git rev-parse --show-toplevel)
cd "$REPO_ROOT"

# Etape 1 — Firmware BuzzClick MERGED (TOUJOURS en premier)
# On produit le merged binary (bootloader + partitions + boot_app0 + app)
# identique a la CI/CD PROD -> garantit le BORE QUALIF <-> PROD

# 1a. Compiler
powershell.exe -NoProfile -Command "& 'C:\Users\cyril\.platformio\penv\Scripts\pio.exe' run -e buzzclick"

# 1b. Merger
powershell.exe -NoProfile -Command "
  python 'C:\Users\cyril\.platformio\packages\tool-esptoolpy\esptool.py' --chip esp32c3 merge_bin \`
    -o buzzclick-merged.bin \`
    0x0     .pio\build\buzzclick\bootloader.bin \`
    0x8000  .pio\build\buzzclick\partitions.bin \`
    0xe000  'C:\Users\cyril\.platformio\packages\framework-arduinoespressif32\tools\partitions\boot_app0.bin' \`
    0x10000 .pio\build\buzzclick\firmware.bin
"

# 1c. Integrer dans les assets Go (merged binary + version)
VERSION=$(grep '"version"' server-go/config.json | sed 's/.*"\([0-9.]*\)".*/\1/')
cp buzzclick-merged.bin server-go/assets/firmware/buzzclick-latest.bin
echo -n "$VERSION" > server-go/assets/firmware/version.txt
rm buzzclick-merged.bin

# Etape 2 — Frontend React (revient explicitement a $REPO_ROOT, jamais un "cd .." relatif)
cd "$REPO_ROOT/server-go/web" && npm run build
cd "$REPO_ROOT"

# Etape 3 — Backend Go — cross-compilation Windows exe (candidat local, agnostique a
# l'environnement — c'est PUBLISH QUALIF qui le rend disponible pour QUALIF)
# BUILD_DIR ancre sur $REPO_ROOT (pas relatif au cwd courant) — non negociable, voir
# deploy.template.md pour l'exemple INCORRECT (monorepo) que cet ancrage evite.
export PATH="$PATH:/usr/local/go/bin"
MILESTONE_VERSION=$(grep '"version"' server-go/config.json | sed 's/.*"\([0-9]*\.[0-9]*\.[0-9]*\)\..*/\1/')
FULL_VERSION=$(grep '"version"' server-go/config.json | sed 's/.*"\([0-9.]*\)".*/\1/')
BUILD_DIR="$REPO_ROOT/build/candidate_v${MILESTONE_VERSION}"
mkdir -p "$BUILD_DIR"
cd server-go && GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
  go build -ldflags="-s -w" -o "$BUILD_DIR/buzzcontrol-candidate-${FULL_VERSION}-windows-amd64.exe" ./cmd/server
cd "$REPO_ROOT"
```

### Pourquoi le merged binary

- Le merged inclut bootloader + table de partitions + boot_app0 + app
- Permet le flash USB complet de buzzers neufs/morts depuis l'interface admin
- Le serveur extrait la partition app pour les OTA sur buzzers deja flashes
- La CI/CD PROD fait exactement la meme chose -> BORE garanti
- Regle memoire : `feedback_qualif_windows_firmware.md`

### Artefact BUILD

Convention de chemin/nommage : voir `deploy.template.md` (checklist BUILD). Nom d'artefact
BuzzControl : `buzzcontrol-candidate-<version>-windows-amd64.exe`, produit dans
`build/candidate_v<X.Y.Z>/` — candidat local, pas encore disponible pour QUALIF tant que
PUBLISH QUALIF ne l'a pas promu (voir `.claude/agents/environments/publish.qualif.md`).
