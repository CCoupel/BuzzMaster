#!/bin/bash
# build-release.sh
# Script de build pour la release BuzzControl (Windows + Linux ARM64)

set -e

# Options
SKIP_FRONTEND=false
WINDOWS_ONLY=false
LINUX_ONLY=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --skip-frontend) SKIP_FRONTEND=true; shift ;;
        --windows-only) WINDOWS_ONLY=true; shift ;;
        --linux-only) LINUX_ONLY=true; shift ;;
        *) echo "Unknown option: $1"; exit 1 ;;
    esac
done

# Lire la version depuis config.json
VERSION=$(grep -o '"version": *"[^"]*"' config.json | grep -o '"[^"]*"$' | tr -d '"')

echo "========================================"
echo " BuzzControl Release Build v$VERSION"
echo "========================================"
echo ""

# 1. Build du frontend (si pas skip)
if [ "$SKIP_FRONTEND" = false ]; then
    echo "[1/4] Building frontend..."
    npm run build --prefix web
    echo "Frontend build complete."
else
    echo "[1/4] Skipping frontend build."
fi

# 2. Copier dist pour l'embedding
echo "[2/4] Preparing embedded files..."
rm -rf cmd/server/dist
cp -r web/dist cmd/server/dist
echo "Embedded files ready."

# 3. Creer le dossier releases
mkdir -p releases

# 4. Build des executables
echo "[3/4] Building executables..."

if [ "$LINUX_ONLY" = false ]; then
    echo "  - Windows AMD64..."
    GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o "releases/buzzcontrol-v$VERSION-windows-amd64.exe" ./cmd/server
fi

if [ "$WINDOWS_ONLY" = false ]; then
    echo "  - Linux ARM64 (Raspberry Pi)..."
    GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="-s -w" -o "releases/buzzcontrol-v$VERSION-linux-arm64" ./cmd/server
fi

echo "Executables built."

# 5. Afficher les resultats
echo "[4/4] Build complete!"
echo ""
echo "Generated files:"
ls -lh releases/ | tail -n +2 | awk '{print "  - " $9 " (" $5 ")"}'

echo ""
echo "========================================"
echo " Release v$VERSION ready!"
echo "========================================"
echo ""
echo "Next steps:"
echo "  1. Test the executables"
echo "  2. git add . && git commit -m 'release: vX.Y.0'"
echo "  3. git push"
echo "  4. git tag -a v$VERSION -m 'Release v$VERSION' && git push origin v$VERSION"
echo "  5. Upload to Bitbucket Downloads"
