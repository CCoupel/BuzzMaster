#!/usr/bin/env bash
# Firmware BuzzClick — script unique (infra.md §3bis C11, #294).
# Appelé par scripts/ci.sh (local + CI), ci-branch.yml (job compiling-firmware),
# release.yml (job compiling-firmware) et la recette BUILD de .claude/agents/deploy.md.
#
# Usage : scripts/build-firmware.sh [SORTIE]
#   SORTIE : chemin du binaire merged produit (défaut : buzzclick-merged.bin à la racine).
#
# Étapes : pio run -e buzzclick → merge esptool (bootloader + partitions + boot_app0 + app)
# → bornes de taille 200 Ko - 3 Mo. Code retour non nul à la moindre erreur.
set -euo pipefail
cd "$(dirname "$0")/.."

OUT="${1:-buzzclick-merged.bin}"
MIN_SIZE=204800   # 200 KB
MAX_SIZE=3145728  # 3 MB

command -v pio >/dev/null 2>&1 || { echo "❌ pio introuvable (PlatformIO requis)" >&2; exit 1; }
command -v jq  >/dev/null 2>&1 || { echo "❌ jq introuvable" >&2; exit 1; }

echo "-- pio run -e buzzclick"
pio run -e buzzclick

# esptool et boot_app0.bin viennent des paquets PlatformIO ; esptool tourne avec le Python
# de PlatformIO (le conteneur des agents n'a pas de commande `python`).
PIO_INFO=$(pio system info --json-output)
PIO_CORE=$(jq -r .core_dir.value <<<"$PIO_INFO")
PIO_PY=$(jq -r .python_exe.value <<<"$PIO_INFO")
ESPTOOL=$(find "$PIO_CORE/packages/tool-esptoolpy" -name esptool.py 2>/dev/null | head -1)
BOOT_APP0=$(find "$PIO_CORE"/packages/framework-arduinoespressif32*/tools/partitions/boot_app0.bin 2>/dev/null | head -1)
[ -n "$ESPTOOL" ]   || { echo "❌ esptool.py introuvable dans $PIO_CORE/packages" >&2; exit 1; }
[ -n "$BOOT_APP0" ] || { echo "❌ boot_app0.bin introuvable dans $PIO_CORE/packages" >&2; exit 1; }

echo "-- merge esptool -> $OUT"
mkdir -p "$(dirname "$OUT")"
"$PIO_PY" "$ESPTOOL" --chip esp32c3 merge_bin \
  -o "$OUT" \
  0x0     .pio/build/buzzclick/bootloader.bin \
  0x8000  .pio/build/buzzclick/partitions.bin \
  0xe000  "$BOOT_APP0" \
  0x10000 .pio/build/buzzclick/firmware.bin

SIZE=$(stat -c%s "$OUT")
echo "Merged firmware binary: $SIZE bytes ($(numfmt --to=iec "$SIZE"))"
if [ "$SIZE" -lt "$MIN_SIZE" ]; then echo "❌ Firmware too small ($SIZE < $MIN_SIZE)" >&2; exit 1; fi
if [ "$SIZE" -gt "$MAX_SIZE" ]; then echo "❌ Firmware too large ($SIZE > $MAX_SIZE)" >&2; exit 1; fi
echo "✅ Firmware OK: $OUT"
