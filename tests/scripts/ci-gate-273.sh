#!/usr/bin/env bash
# Preuve de la porte vitest de scripts/ci.sh (#273) — script QA.
#
# Usage : tests/scripts/ci-gate-273.sh [<ref-git>]      (défaut : HEAD)
#   Prérequis : git, bash, node/npm (comme scripts/ci.sh). Durée : plusieurs minutes (2 runs de ci.sh).
#   Variable : CI_GATE_TMPDIR = dossier où créer le worktree (défaut : dossier frère du dépôt, pas /tmp).
#              CI_GATE_REAL_GO=1 = ne pas court-circuiter les étapes Go (voir ci-dessous ; très long).
#   Par défaut, `go` et `govulncheck` sont remplacés par des shims no-op (PATH) : la porte prouvée est
#   l'étape vitest, et `go test -race ./...` (jusqu'à 20 min) la dépasse de loin. Les étapes npm
#   (ci, build, vitest, audit) et l'embed (cp) restent réelles.
#   Ne modifie JAMAIS le dépôt principal : tout se passe dans un `git worktree` jetable créé à
#   un chemin absolu (mktemp -d), supprimé en fin de script (trap EXIT, même en cas d'échec).
#   ATTENTION : ne pas lancer depuis un worktree dont l'état n'est pas commité — la ref testée
#   est un commit (les modifications non commitées ne sont pas vues).
#
# Vérifie :
#   A. run normal : la sortie contient l'étape vitest, les 4 tests fonts-embed « dist/ (build) »
#      sont `passed` (et non `skipped`), et ci.sh se termine par « CI OK » (code 0) ;
#   B. run avec un test vitest volontairement rouge : ci.sh sort en erreur (code != 0), l'échec
#      est dans l'étape vitest (avant l'embed et go build), et « CI OK » est absent.
# Code retour : 0 = porte prouvée ; 1 = preuve non établie ; 2 = erreur d'environnement.
set -uo pipefail

REF="${1:-HEAD}"
MAIN_REPO="$(git -C "$(dirname "$0")" rev-parse --show-toplevel)" || { echo "dépôt introuvable" >&2; exit 2; }
# Base du worktree : dossier frère du dépôt principal par défaut (sous WSL, le npm du PATH peut être
# le npm Windows, qui échoue hors /mnt/c). Surcharge : CI_GATE_TMPDIR=<dossier existant>.
BASE="${CI_GATE_TMPDIR:-$(dirname "$MAIN_REPO")}"
WT_PARENT="$(mktemp -d "$BASE/.ci-gate-273.XXXX")" || exit 2
WT="$WT_PARENT/wt"
SHIM="$WT_PARENT/shim"
if [ "${CI_GATE_REAL_GO:-0}" != "1" ]; then
  mkdir -p "$SHIM"
  for b in go govulncheck; do printf '#!/bin/sh\nexit 0\n' >"$SHIM/$b"; chmod +x "$SHIM/$b"; done
  export PATH="$SHIM:$PATH"
  echo "Étapes Go court-circuitées (shims) — CI_GATE_REAL_GO=1 pour les exécuter."
fi
SUFFIX=""
[ "${CI_GATE_REAL_GO:-0}" != "1" ] && SUFFIX=" (étapes Go simulées — CI_GATE_REAL_GO=1 pour le run complet)"
LOG_A="$WT_PARENT/run-normal.log"
LOG_B="$WT_PARENT/run-rouge.log"

cleanup() {
  # Garde-fou : ne jamais supprimer le dépôt principal.
  if [ -n "${WT:-}" ] && [ "$WT" != "$MAIN_REPO" ] && [ "$WT_PARENT" != "$MAIN_REPO" ] && [ -d "$WT" ]; then
    git -C "$MAIN_REPO" worktree remove --force "$WT" >/dev/null 2>&1 || true
  fi
  git -C "$MAIN_REPO" worktree prune >/dev/null 2>&1 || true
  [ "$WT_PARENT" != "$MAIN_REPO" ] && [ "$WT_PARENT" != "/" ] && rm -rf "$WT_PARENT"
}
trap cleanup EXIT

fail() { echo "ÉCHEC : $*" >&2; exit 1; }

git -C "$MAIN_REPO" worktree add --detach "$WT" "$REF" >/dev/null 2>&1 || { echo "worktree add impossible (ref $REF)" >&2; exit 2; }
echo "Worktree jetable : $WT (ref $(git -C "$WT" rev-parse --short HEAD))"

# --- A. Run normal -----------------------------------------------------------------------
echo "== A. run normal de scripts/ci.sh =="
( cd "$WT" && bash scripts/ci.sh ) >"$LOG_A" 2>&1
RC_A=$?
[ "$RC_A" -eq 0 ] || { tail -40 "$LOG_A" >&2; fail "A : ci.sh devrait réussir sur la ref $REF (code $RC_A)"; }
grep -q "CI OK" "$LOG_A" || fail "A : « CI OK » absent de la sortie"
grep -qi "vitest" "$LOG_A" || fail "A : aucune trace de vitest dans la sortie de ci.sh"
# fonts-embed : exécuté, pas sauté. Le reporter vitest affiche « fonts-embed.test.js (N tests | M skipped) ».
LINE="$(grep -E "fonts-embed\.test\.js" "$LOG_A" | tail -1)"
[ -n "$LINE" ] || fail "A : fonts-embed.test.js absent de la sortie vitest"
echo "   $LINE"
echo "$LINE" | grep -qi "skipped" && fail "A : fonts-embed contient des tests skipped (dist/ (build) non exécutés) : $LINE"
echo "   OK : fonts-embed sans test sauté, ci.sh → CI OK$SUFFIX"

# --- B. Run avec un test rouge -------------------------------------------------------------
echo "== B. run avec un test vitest volontairement rouge =="
RED="$WT/server-go/web/src/zz_ci_gate_273_red.test.js"
cat >"$RED" <<'JS'
import { it, expect } from 'vitest'
it('porte CI #273 : test volontairement rouge', () => { expect(1).toBe(2) })
JS
( cd "$WT" && bash scripts/ci.sh ) >"$LOG_B" 2>&1
RC_B=$?
[ "$RC_B" -ne 0 ] || fail "B : ci.sh a réussi malgré un test vitest rouge — la porte n'est PAS bloquante"
grep -q "CI OK" "$LOG_B" && fail "B : « CI OK » présent malgré le test rouge"
grep -q "zz_ci_gate_273_red" "$LOG_B" || fail "B : l'échec ne vient pas du test rouge injecté (cause différente)"
# L'échec doit survenir à l'étape vitest : les étapes suivantes ne doivent pas avoir démarré.
grep -qE "== \[[0-9]+/[0-9]+\] (Embed|Build|Tests \+ race|Lint|Audit)" "$LOG_B" \
  && fail "B : une étape postérieure à vitest a démarré — la porte n'arrête pas ci.sh à l'étape vitest"
echo "   OK : ci.sh sort en erreur (code $RC_B) à l'étape vitest"

echo "PORTE #273 PROUVÉE$SUFFIX"
exit 0
