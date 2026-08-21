/**
 * hostContext — résolution du triplet d'hôte normalisé (#184/B-F1).
 *
 * Une implémentation de type de contenu ne doit **jamais** lire
 * `gameState.phase` ni `gameState.MEMOTION_SUBPHASE` directement — elle
 * reçoit de son hôte ce triplet `{playable, revealed, timerRunning, cardId}`.
 * Spécification unique et table de dérivation : `contracts/question-types.md`
 * §4 — miroir exact du `HostContext` Go (`internal/game/engine.go`, tâche
 * B-B3 côté `dev-backend`).
 *
 * **Non sérialisé** — recalculé de part et d'autre à partir de `PHASE` et
 * `MEMOTION_SUBPHASE`, tous deux déjà présents dans `GameState` : le
 * sérialiser coûterait de la charge utile pour une information
 * intégralement dérivable (contrat §4). Contrepartie assumée : deux
 * implémentations d'une même règle, dont les cas de test doivent être
 * nommés à l'identique (voir `hostContext.test.js`) — coordination avec
 * `dev-backend` sur B-B3, sans bloquer dessus.
 *
 * `timerRunning` en sous-phase carte reprend l'heuristique déjà établie par
 * `utils/motionRules.js` (`ctx.timerRunning` = `gameState.timer > 0`, chrono
 * de la carte) plutôt que d'en inventer une nouvelle.
 */

/**
 * @typedef {Object} HostContext
 * @property {boolean} playable - les entrées sont acceptées, le contenu est en jeu
 * @property {boolean} revealed - la réponse est montrée
 * @property {boolean} timerRunning - un chronomètre décompte pour cette manche
 * @property {string} cardId - "" pour l'hôte question ; ID de carte pour l'hôte carte MEMOTION
 */

/**
 * @param {Object} gameState - état de jeu tel qu'exposé par `useWebSocket.js`
 * @param {string} [gameState.phase] - `gameState.phase` (camelCase, ex-`PHASE`)
 * @param {string} [gameState.MEMOTION_SUBPHASE] - '' | 'MEMORIZE' | 'GRID' | 'SELECTED' | 'QUESTION' | 'REVEAL'
 * @param {string} [gameState.MEMOTION_SELECTED] - ID de la carte active
 * @param {number} [gameState.timer] - temps restant courant (partagé question/carte, un seul chrono)
 * @returns {HostContext}
 */
export function resolveHostContext(gameState) {
  const phase = gameState?.phase
  const subphase = gameState?.MEMOTION_SUBPHASE || ''

  // Hôte carte MEMOTION — sous-phases où une carte est en jeu (contrat §4,
  // ligne "Carte MEMOTION").
  if (subphase === 'QUESTION' || subphase === 'REVEAL') {
    return {
      playable: subphase === 'QUESTION',
      revealed: subphase === 'REVEAL',
      timerRunning: subphase === 'QUESTION' && (gameState?.timer ?? 0) > 0,
      cardId: gameState?.MEMOTION_SELECTED || '',
    }
  }

  // Aucun hôte actif — MEMORIZE/GRID/SELECTED (contrat §4, ligne "Aucun") :
  // ni playable ni revealed, quel que soit `phase` (qui reste STARTED tout
  // au long de la manche MEMOTION). `cardId` suit `MEMOTION_SELECTED`
  // uniquement en sous-phase SELECTED (carte choisie, chrono pas encore
  // démarré) — "selon le cas" du contrat.
  if (subphase) {
    return {
      playable: false,
      revealed: false,
      timerRunning: false,
      cardId: subphase === 'SELECTED' ? (gameState?.MEMOTION_SELECTED || '') : '',
    }
  }

  // Hôte question — contrat §4, ligne "Question".
  return {
    playable: phase === 'STARTED',
    revealed: phase === 'REVEALED',
    timerRunning: phase === 'STARTED',
    cardId: '',
  }
}
