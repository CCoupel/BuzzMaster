#!/usr/bin/env bash
# Preuve de la porte vitest de scripts/ci.sh (#273) — script QA.
#
# Usage : tests/scripts/ci-gate-273.sh [<ref-git>]      (défaut : HEAD)
#   Prérequis : git, bash, node/npm (comme scripts/ci.sh). Durée : plusieurs minutes (2 runs de ci.sh).
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
WT_PARENT="$(mktemp -d)" || exit 2
WT="$WT_PARENT/wt"
LOG_A="$WT_PARENT/run-normal.log"
LOG_B="$WT_PARENT/run-rouge.log"

cleanup() {
  # Garde-fou : ne jamais supprimer le dépôt principal.
  if [ -n "${WT:-}" ] && [ "$WT" != "$MAIN_REPO" ] && [ -d "$WT" ]; then
    git -C "$MAIN_REPO" worktree remove --force "$WT" >/dev/null 2>&1 || true
  fi
  git -C "$MAIN_REPO" worktree prune >/dev/null 2>&1 || true
  rm -rf "$WT_PARENT"
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
echo "   OK : fonts-embed sans test sauté, ci.sh → CI OK"

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

echo "PORTE #273 PROUVÉE"
exit 0
