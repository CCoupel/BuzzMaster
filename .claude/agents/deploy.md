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
# Depuis la racine du projet
cd /mnt/c/Users/cyril/Documents/VScode/GITHUB/BuzzMaster

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

# Etape 2 — Frontend React
cd server-go/web && npm run build && cd ..

# Etape 3 — Backend Go — cross-compilation Windows exe (QUALIF testable directement)
export PATH="$PATH:/usr/local/go/bin"
MILESTONE_VERSION=$(grep '"version"' server-go/config.json | sed 's/.*"\([0-9]*\.[0-9]*\.[0-9]*\)\..*/\1/')
FULL_VERSION=$(grep '"version"' server-go/config.json | sed 's/.*"\([0-9.]*\)".*/\1/')
QUALIF_DIR="build/qualif/${MILESTONE_VERSION}"
mkdir -p "$QUALIF_DIR"
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
  go build -ldflags="-s -w" -o "$QUALIF_DIR/buzzcontrol-qualif-${FULL_VERSION}-windows-amd64.exe" ./cmd/server
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

Le fichier `build/qualif/X.Y.Z/buzzcontrol-qualif-<version>[-windows-amd64].exe` (dossier `build/`
non gitté, à la racine du projet) est l'artefact unique QUALIF. Copier vers le serveur Raspberry Pi
via scp ou rsync.
