/**
 * prepareWaitReason — motif d'attente en phase `PREPARE` (#172/C2, plan
 * `_work/reports/plan-20260817-122307.md` §7 Bloc C).
 *
 * MIROIR client-side, en LECTURE SEULE, du prédicat serveur
 * `participantsConform(question, state)` (#172/B1, `engine.go`) et de la
 * condition de sortie de `PREPARE` (`AreAllTeamsReady() &&
 * participantsConform(...)`, #172/B2, `main.go:1456`). Ce fichier ne décide
 * RIEN — il n'est jamais consulté pour bloquer une action (le moteur reste
 * l'unique source de vérité, verrouillée côté serveur par `Engine.Start`,
 * #172/B4) : il sert uniquement à EXPLIQUER pourquoi la question reste en
 * `PREPARE`, sur `/admin` (GamePage.jsx) et `/anim` (AnimConductPanel.jsx).
 *
 * Table de règles identique à B1 (aucune branche par type en dehors de
 * cette table) :
 *   - SPEEDY, QCM, ARDOISE : déjà couvert par `AreAllTeamsReady` (≥1 équipe
 *     active) — toujours conforme ici, aucun changement de comportement.
 *   - MEMORY SOLO           : exactement une équipe sélectionnée.
 *   - MEMORY multi          : au moins deux équipes sélectionnées.
 *   - MEMOTION              : au moins une équipe sélectionnée.
 *   - RAFALE SOLO           : toujours conforme (aucune restriction).
 *   - RAFALE multi          : au moins une équipe sélectionnée (v8.0.0,
 *     #199, retour QUALIF — dev-backend SHA 393c6dc7, engine.go
 *     participantsConform : `question.Category`/`RafaleDifficulty`
 *     absents/invalides sont volontairement HORS PÉRIMÈTRE ici, déjà
 *     couverts côté admin par `rafaleBlocked`/RafalePoolAlert, contrat
 *     §7.2 — dupliquer cette partie créerait deux messages concurrents
 *     pour la même cause).
 *   - Type inconnu          : permissif par défaut.
 */

/** @param {{READY?: boolean|string}} team */
export function isTeamReady(team) {
  return team?.READY === true || team?.READY === 'TRUE'
}

/**
 * @param {{TYPE?: string, MEMORY_MODE?: string}|null} question
 * @param {string[]} participating - sélection courante (MEMORY_PARTICIPATING_TEAMS
 *   ou MEMOTION_PARTICIPATING_TEAMS selon le type)
 */
export function participantsConform(question, participating) {
  const type = question?.TYPE
  const count = (participating || []).length
  if (type === 'MEMORY') {
    const isSolo = !question.MEMORY_MODE || question.MEMORY_MODE === 'SOLO'
    return isSolo ? count === 1 : count >= 2
  }
  if (type === 'MEMOTION') {
    return count >= 1
  }
  if (type === 'RAFALE') {
    const isSolo = !question.RAFALE_MODE || question.RAFALE_MODE === 'SOLO'
    return isSolo ? true : count >= 1
  }
  // SPEEDY, QCM, ARDOISE, type inconnu — déjà couvert par AreAllTeamsReady
  // (≥1 équipe active) ou sans règle : permissif.
  return true
}

// Libellés courts (#166, style "à suivre"/"attendu"/"optionnel" —
// AnimConductPanel.anim-conduct-btn-sub, tablette, place limitée) et
// libellés complets (régie, memory-selector-label, plus de place).
function participantsReasonLabel(question, { short } = {}) {
  const type = question?.TYPE
  if (type === 'MEMORY') {
    const isSolo = !question.MEMORY_MODE || question.MEMORY_MODE === 'SOLO'
    if (short) return isSolo ? '1 équipe' : '2 équipes'
    return isSolo ? 'sélectionnez une équipe' : 'sélectionnez au moins deux équipes'
  }
  if (type === 'MEMOTION') {
    return short ? '1 équipe' : 'sélectionnez au moins une équipe'
  }
  if (type === 'RAFALE') {
    return short ? '1 équipe' : 'sélectionnez au moins une équipe participante'
  }
  return null
}

/**
 * @param {string} phase - gameState.phase
 * @param {{TYPE?: string, MEMORY_MODE?: string}|null} question - gameState.question
 * @param {Array<{READY?: boolean|string}>} activeTeams - équipes ayant ≥1 buzzer
 *   assigné (même filtre que l'affichage, cf. `AreAllTeamsReady` — "Empty
 *   teams are ignored, matching the frontend display filter")
 * @param {{MEMORY_PARTICIPATING_TEAMS?: string[], MEMOTION_PARTICIPATING_TEAMS?: string[]}} gameState
 * @param {{short?: boolean}} [opts] - `short: true` pour le libellé tablette
 *   (AnimConductPanel, espace contraint) ; sinon libellé complet (régie).
 * @returns {string|null} le motif, ou `null` hors PREPARE / si rien à expliquer
 */
export function prepareWaitReason(phase, question, activeTeams, gameState, opts = {}) {
  if (phase !== 'PREPARE') return null

  const buzzersWaiting = (activeTeams || []).some(t => !isTeamReady(t))
  if (buzzersWaiting) return opts.short ? 'buzzers' : 'Buzzers en attente'

  const participating = question?.TYPE === 'MEMOTION'
    ? (gameState?.MEMOTION_PARTICIPATING_TEAMS || [])
    : question?.TYPE === 'RAFALE'
      ? (gameState?.RAFALE_PARTICIPATING_TEAMS || [])
      : (gameState?.MEMORY_PARTICIPATING_TEAMS || [])

  if (!participantsConform(question, participating)) {
    return participantsReasonLabel(question, opts)
  }

  return null
}
