#!/usr/bin/env bash
# CI_CMD — script unique de la CI (infra.md §3bis C11).
# Appelé par ci-branch.yml, release.yml (job testing), qa et deploy. Aucune étape n'existe
# uniquement en CI ou uniquement en local.
set -euo pipefail
cd "$(dirname "$0")/.."

echo "== [1/6] Frontend: npm ci + build =="
( cd server-go/web && npm ci && npm run build )

echo "== [2/6] Embed dist (go:embed all:dist, #153) =="
rm -rf server-go/cmd/server/dist
cp -r server-go/web/dist server-go/cmd/server/dist

cd server-go
echo "== [3/6] Build (commands.build) =="
go build ./cmd/server
rm -f server server.exe

echo "== [4/6] Tests + race detector =="
CGO_ENABLED=1 go test -race -timeout=20m ./...

echo "== [5/6] Lint (commands.lint) =="
go vet ./...

echo "== [6/6] Audit (commands.audit) =="
command -v govulncheck >/dev/null 2>&1 || go install golang.org/x/vuln/cmd/govulncheck@latest
govulncheck ./...

echo "CI OK"
