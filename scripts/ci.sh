#!/usr/bin/env bash
# CI_CMD — script unique de la CI (infra.md §3bis C11).
# Appelé par ci-branch.yml, release.yml (job testing), qa et deploy. Aucune étape n'existe
# uniquement en CI ou uniquement en local.
set -euo pipefail
cd "$(dirname "$0")/.."

echo "== [1/7] Frontend: npm ci + build =="
( cd server-go/web && npm ci && npm run build )

# Après le build : les tests fonts-embed « dist/ (build) » vérifient dist/ (sinon skipped).
echo "== [2/7] Frontend: tests (vitest, #273) =="
( cd server-go/web && npm test )

echo "== [3/7] Embed dist (go:embed all:dist, #153) =="
rm -rf server-go/cmd/server/dist
cp -r server-go/web/dist server-go/cmd/server/dist

cd server-go
echo "== [4/7] Build (commands.build) =="
go build ./cmd/server
rm -f server server.exe

echo "== [5/7] Tests + race detector =="
CGO_ENABLED=1 go test -race -timeout=20m ./...

echo "== [6/7] Lint (commands.lint) =="
go vet ./...

echo "== [7/7] Audit (commands.audit) =="
command -v govulncheck >/dev/null 2>&1 || go install golang.org/x/vuln/cmd/govulncheck@latest
govulncheck ./...
# Audit npm du frontend (#268, #270) : toutes dépendances (prod + dev), seuil high.
( cd web && npm audit --audit-level=high )

echo "CI OK"
