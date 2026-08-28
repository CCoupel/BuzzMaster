---
name: deploy
description: "Adaptations projet BuzzControl pour l'agent deploy. Base generique : deploy.template.md."
model: sonnet
color: red
---

# Agent Deploy — Adaptations BuzzControl

> **Base** : Voir `deploy.template.md` pour le role, le declenchement, les workflows QUALIF/PROD
> generiques, le protocole d'echec CI, le rollback et la checklist. Ce fichier ne contient que
> les regles specifiques a BuzzControl qui overrident ou completent les etapes generiques.

## BuzzControl — Règles spécifiques QUALIF

### Ordre de build OBLIGATOIRE (BORE)

Le binaire embarque le firmware BuzzClick (merged) ET le frontend React.
**L'ordre est critique — ne jamais le modifier.**

```bash
# Racine du repo, calculee explicitement — ne jamais deriver de cd relatifs qui se perdent
# au fil des etapes (voir deploy.template.md : QUALIF_DIR doit toujours resoudre a la racine,
# meme en monorepo, meme apres un cd server-go/web pour le build frontend)
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

# Etape 3 — Backend Go — cross-compilation Windows exe (QUALIF testable directement)
# QUALIF_DIR ancre sur $REPO_ROOT (pas relatif au cwd courant) — non negociable, voir
# deploy.template.md pour l'exemple INCORRECT (monorepo) que cet ancrage evite.
export PATH="$PATH:/usr/local/go/bin"
MILESTONE_VERSION=$(grep '"version"' server-go/config.json | sed 's/.*"\([0-9]*\.[0-9]*\.[0-9]*\)\..*/\1/')
FULL_VERSION=$(grep '"version"' server-go/config.json | sed 's/.*"\([0-9.]*\)".*/\1/')
QUALIF_DIR="$REPO_ROOT/build/qualif_v${MILESTONE_VERSION}"
mkdir -p "$QUALIF_DIR"
cd server-go && GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
  go build -ldflags="-s -w" -o "$QUALIF_DIR/buzzcontrol-qualif-${FULL_VERSION}-windows-amd64.exe" ./cmd/server
cd "$REPO_ROOT"
```

### Pourquoi le merged binary

- Le merged inclut bootloader + table de partitions + boot_app0 + app
- Permet le flash USB complet de buzzers neufs/morts depuis l'interface admin
- Le serveur extrait la partition app pour les OTA sur buzzers deja flashes
- La CI/CD PROD fait exactement la meme chose -> BORE garanti
- Regle memoire : `feedback_qualif_windows_firmware.md`

> ⚠️ **Règle** : ne jamais demander la validation QUALIF à l'utilisateur avant que l'étape 3
> (binaire Windows) soit terminée — l'utilisateur teste systématiquement depuis Windows.

### Smoke tests QUALIF BuzzControl

Le serveur QUALIF est lancé sur le **port 9090** pour éviter toute interférence avec un serveur de production tournant sur le port 80.

```bash
# Depuis la racine du projet — lancer le binaire QUALIF sur port 9090 (flag --port, v5.1.3+)
"$QUALIF_DIR/buzzcontrol-qualif-${FULL_VERSION}-windows-amd64.exe" --port 9090 &
QUALIF_PID=$!
sleep 2  # attendre démarrage

BASE="http://localhost:9090"

# Smoke tests
curl -sf $BASE/version               # version
curl -sf $BASE/                      # page principale
curl -sf $BASE/api/firmware/buzzclick/version  # firmware endpoint
curl -sf $BASE/questions             # questions
curl -sf $BASE/listGame              # liste des jeux
curl -sf $BASE/tv                    # affichage TV

# Arrêt propre
kill $QUALIF_PID
```

### Artefact QUALIF

Convention de chemin/nommage : voir `deploy.template.md` (checklist QUALIF). Nom d'artefact
BuzzControl : `buzzcontrol-qualif-<version>[-windows-amd64].exe`. Copier vers le serveur
Raspberry Pi via scp ou rsync.
