#!/usr/bin/env bash
# CI_CMD — script unique de la CI (infra.md §3bis C11).
# Appelé par ci-branch.yml, release.yml (job testing), qa et deploy. Aucune étape n'existe
# uniquement en CI ou uniquement en local.
#
# SKIP_FIRMWARE=1 : saute l'étape firmware. Réservé aux jobs `testing` des workflows, où le job
# parallèle `compiling-firmware` exécute le même scripts/build-firmware.sh (pas de double
# compilation en CI). En local, l'étape tourne toujours.
set -euo pipefail
cd "$(dirname "$0")/.."

echo "== [1/8] Firmware BuzzClick (scripts/build-firmware.sh, #294) =="
if [ "${SKIP_FIRMWARE:-0}" = "1" ]; then
  echo "SKIP_FIRMWARE=1 : compilé par le job compiling-firmware"
else
  # Sortie sous .pio/ (gitignoré) : vérification seulement, les assets ne sont pas touchés.
  bash scripts/build-firmware.sh .pio/build/buzzclick/buzzclick-merged.bin
fi

echo "== [2/8] Frontend: npm ci + build =="
( cd server-go/web && npm ci && npm run build )

# Après le build : les tests fonts-embed « dist/ (build) » vérifient dist/ (sinon skipped).
echo "== [3/8] Frontend: tests (vitest, #273) =="
( cd server-go/web && npm test )

echo "== [4/8] Embed dist (go:embed all:dist, #153) =="
rm -rf server-go/cmd/server/dist
cp -r server-go/web/dist server-go/cmd/server/dist

cd server-go
echo "== [5/8] Build (commands.build) =="
go build ./cmd/server
rm -f server server.exe

echo "== [6/8] Tests + race detector =="
CGO_ENABLED=1 go test -race -timeout=20m ./...

echo "== [7/8] Lint (commands.lint) =="
go vet ./...

echo "== [8/8] Audit (commands.audit) =="
command -v govulncheck >/dev/null 2>&1 || go install golang.org/x/vuln/cmd/govulncheck@latest
govulncheck ./...
# Audit npm du frontend (#268, #270) : toutes dépendances (prod + dev), seuil high.
( cd web && npm audit --audit-level=high )

echo "CI OK"
