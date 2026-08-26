# Procédure de Test — PAUSE/CONTINUER pendant une manche MEMOTION (#195, cycle 6 de #187)

**Version** : v7.1.0 (branche `milestone/v7.1.0`)
**Date** : 2026-08-26
**Testeur** : QA
**Issue** : #195 — bug préexistant (confirmé antérieur à #151/v6.4.1, **pas** une régression #187) :
PAUSE pendant un chrono actif d'une manche MEMOTION (chrono de carte OU chrono MEMORIZE/Secret Mode)
tuait définitivement le ticker serveur ; CONTINUER ne relançait jamais rien, le chrono restait
bloqué. Découvert lors de la QUALIF de la carte MEMORY (#187) mais **applicable à toute carte
MEMOTION** (SPEEDY/QCM/MEMORY) et au timer MEMORIZE global — signalé par `code-reviewer` comme seul
point non-bloquant de sa revue du cycle 6.
**Référence** : `_work/handoff/dev-backend-20260826-185334.md` (SHA `4af21933`),
`docs/GAME_STATE_MACHINE.md` §"Secret Mode — Subphase MEMORIZE"

> **Pourquoi un fichier séparé de `memotion-memory-card-187.md`** : ce bug touche le moteur MEMOTION
> général (chrono de carte de n'importe quel type, et chrono MEMORIZE), pas spécifiquement la carte
> MEMORY — le classer dans la procédure MEMORY laisserait croire à un périmètre plus étroit qu'il ne
> l'est réellement. Ce fichier est donc **transverse** aux procédures `memotion-*`.

---

## Prérequis

- [ ] Environnement : QUALIF ou LOCAL
- [ ] Un quiz contenant une question MEMOTION avec un chrono de carte configuré (`TIME > 0`) et au
      moins 2 cartes (au moins une SPEEDY ou QCM, pour le Scénario 1)
- [ ] Un quiz contenant une question MEMOTION avec `MOTION_MEMORIZE_DURATION > 0` (Secret Mode activé
      — cf. `docs/GAME_STATE_MACHINE.md`), pour le Scénario 2
- [ ] Postes `/anim` (tablette) et `/admin` (régie, bouton PAUSE/CONTINUER) ouverts

---

## Scénario 1 — PAUSE/CONTINUER pendant le chrono d'une carte MEMOTION active

**Objectif** : Vérifier que le chrono d'une carte MEMOTION (n'importe quel type) survit à une pause
et reprend normalement à la reprise — c'est le scénario exact rapporté en QUALIF, reproduit ici sur
plusieurs types de carte pour couvrir la portée réelle du bug (préexistant, pas MEMORY-spécifique).

**Garde-fou déjà verrouillé par test automatisé**
(`TestProcessMotionCardTick_PausedTick_IsInertNotGuardFailed`,
`TestStartMotionCardTimer_PauseThenContinue_ResumesCountdown` — ce dernier avec un **vrai ticker**,
pas un appel direct, pour prouver que la goroutine elle-même survit — et
`TestStartMotionCardTimer_StopDuringCard_TickerStaysStopped` pour la non-régression STOP,
`internal/game/engine_motion_pause_resume_187_test.go`) — ce scénario en est la **confirmation
visuelle/manuelle**.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sélectionner une carte (SPEEDY ou QCM) avec chrono de carte, DÉMARRER | Sous-phase `QUESTION`, chrono en décompte visible sur `/anim` et `/tv` | | |
| 2 | Laisser le chrono descendre de quelques secondes, puis appuyer PAUSE (`/admin`) | Le chrono **s'arrête** (gelé à sa valeur courante), l'interface passe en état pause | | |
| 3 | Attendre plusieurs secondes en pause (≥ 5s) | Le chrono reste **strictement figé** à la même valeur pendant toute la pause | | |
| 4 | Appuyer CONTINUER | Le chrono **reprend son décompte** normalement, à partir de la valeur où il avait été figé — pas de saut, pas de blocage | | |
| 5 | Laisser le chrono aller jusqu'à 0 (ou agir manuellement) | Comportement de fin de carte normal pour le type joué (SPEEDY/QCM), sans anomalie résiduelle liée à la pause | | |
| 6 | Répéter les étapes 1-4 sur une carte **MEMORY** (si disponible dans le même quiz ou un autre) | Même résultat : pause fige le chrono, continuer le relance — comportement identique aux autres types de carte | | |
| 7 | Répéter avec **plusieurs cycles** PAUSE→CONTINUER→PAUSE→CONTINUER sur la même carte | Le chrono reprend à chaque fois, aucune dégradation après plusieurs cycles | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 2 — PAUSE/CONTINUER pendant le chrono MEMORIZE (Secret Mode)

**Objectif** : Vérifier que le même correctif s'applique au chrono MEMORIZE (sous-phase globale
`MEMORIZE`, toutes les cartes face RECTO visibles avant la manche) — corrigé **proactivement** par
dev-backend en même temps que le chrono de carte, même garde vulnérable, même risque.

**Garde-fou déjà verrouillé par test automatisé**
(`TestProcessMotionMemorizeTick_PausedTick_IsInertNotGuardFailed`,
`TestStartMotionMemorizeTimer_PauseThenContinue_ResumesCountdown` — vrai ticker) — ce scénario en
est la **confirmation visuelle/manuelle**.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Lancer une question MEMOTION avec `MOTION_MEMORIZE_DURATION` configuré | Sous-phase `MEMORIZE` : toutes les cartes visibles face RECTO, chrono en décompte, sélection de carte impossible | | |
| 2 | Laisser le chrono descendre de quelques secondes, appuyer PAUSE | Le chrono MEMORIZE **s'arrête**, les cartes restent affichées face RECTO (pas de changement de sous-phase) | | |
| 3 | Attendre plusieurs secondes en pause | Le chrono reste figé | | |
| 4 | Appuyer CONTINUER | Le chrono MEMORIZE **reprend** son décompte normalement | | |
| 5 | Laisser le chrono MEMORIZE atteindre 0 | Transition automatique `MEMORIZE → GRID`, comme en dehors de toute pause (comportement inchangé, cf. `GAME_STATE_MACHINE.md`) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 3 — Non-régression : STOP pendant une carte reste un arrêt définitif

**Objectif** : Vérifier que le correctif ne confond pas PAUSE (temporaire) avec un véritable arrêt de
carte (STOP/révélation/carte terminée) — ces derniers doivent continuer à arrêter le ticker
**pour de bon**, sans reprise possible.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Carte MEMOTION active, chrono en décompte | Sous-phase `QUESTION` | | |
| 2 | Taper STOP CHRONO (arrêt réel de la carte, pas PAUSE) | La carte s'arrête normalement (comportement pré-existant, inchangé) | | |
| 3 | Vérifier qu'aucun décompte ne reprend seul par la suite | Le chrono reste à l'arrêt — ce n'est pas devenu une pause récupérable | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Critères de Validation Globale

- [ ] PAUSE fige le chrono d'une carte MEMOTION (tout type), CONTINUER le relance normalement
- [ ] Comportement stable sur plusieurs cycles PAUSE/CONTINUER successifs
- [ ] PAUSE/CONTINUER fonctionne identiquement sur le chrono MEMORIZE (Secret Mode)
- [ ] STOP reste un arrêt définitif, non confondu avec une pause récupérable

---

## Non-Régression (suite automatisée, à exécuter avant validation manuelle)

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | `cd server-go && go build ./... && go test ./... -race` | Build OK, tous les tests PASS, y compris `engine_motion_pause_resume_187_test.go` (5 tests) | | |
| 2 | `go test ./internal/game/... -run 'PauseThenContinue\|PausedTick\|StopDuringCard' -race -v` | Les 5 tests du cycle 6 PASS, y compris les 2 tests à **ticker réel** (StartMotionCardTimer/StartMotionMemorizeTimer) qui prouvent que la goroutine survit à la pause | | |
| 3 | `go test ./internal/game/... -run 'PanicRecovery' -v` | Non-régression #151 (récupération de panique du ticker) toujours verte | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Notes QA

[Espace pour observations]
