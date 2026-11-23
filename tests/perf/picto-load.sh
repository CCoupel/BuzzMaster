#!/usr/bin/env bash
# Mesure de charge du relais de dessin PICTO (#193) — voir tests/perf/picto-load.md
set -euo pipefail
cd "$(dirname "$0")/../../server-go"
export PICTO_LOAD=1
exec go test ./cmd/server -run 'TestPictoLoad193' -v -timeout 20m -count=1
