/**
 * motionRules — source UNIQUE des gestes de la ligne L2 (« gestes propres au
 * mode ») pour une manche MEMOTION conduite depuis `/anim`, #160/F3.
 *
 * Même philosophie que `phaseRules.js` (#166/F4b) : `AnimMotionActions` ne
 * réécrit JAMAIS une condition d'activation localement — il rend ce que
 * `motionGestures()` lui donne. Toute nouvelle règle sur les gestes MEMOTION
 * se dérive d'ici, jamais d'un composant.
 *
 * Divergence assumée avec `/admin` (`GamePage.jsx`) : la régie MASQUE
 * STOP TIMER / RÉVÉLER selon l'état du chrono (rendu conditionnel). `/anim`
 * les ÉTEINT sans les démonter — même « renversement de principe » que L1
 * (#166/#171) : la ligne garde toujours le même nombre d'emplacements pour
 * une sous-phase donnée, seul l'état de chaque bouton varie. Voir maquette
 * `docs/mockups/anim-memotion-160.html` §4.
 *
 * `action` porte le NOM D'ACTION WEBSOCKET LITTÉRAL
 * (`contracts/websocket-actions.md`, les `.emits` de la maquette) plutôt
 * qu'un identifiant interne inventé : c'est la forme la moins ambiguë pour
 * qu'`AnimMotionActions.jsx` (#160/F6) sache quel émetteur `useWebSocket.js`
 * (#160/F2) appeler, et elle documente elle-même sa correspondance au
 * contrat. `payload` est le MSG exact tel que décrit par la maquette (ex.
 * `MEMOTION_DONE { CARD_ID, WINNER_TEAM }`).
 *
 * Matrice complète (référence : maquette §"Ce que la maquette engage") :
 *
 * | Sous-phase | Gestes                                                    |
 * |------------|------------------------------------------------------------|
 * | MEMORIZE   | []  — aucun geste, bandeau d'attente rendu par le composant |
 * | GRID       | []  — le geste est la carte elle-même, en L3                |
 * | SELECTED   | DÉMARRER (go) · ANNULER (optional)                          |
 * | QUESTION   | STOP CHRONO (optional/off) · RÉVÉLER (off/go) · SANS VAINQUEUR (optional) |
 * | REVEAL     | <équipe courante> (go, couleur équipe) · PERSONNE (optional) |
 */

/**
 * @typedef {Object} MotionGesture
 * @property {string} key
 * @property {string} label
 * @property {string|null} subLabel
 * @property {'go'|'optional'|'danger'|'off'} state
 * @property {'MEMOTION_FLIP'|'MEMOTION_STOP_TIMER'|'MEMOTION_REVEAL'|'MEMOTION_DONE'} action -
 *   nom d'action WebSocket littéral (contracts/websocket-actions.md) — présent
 *   même pour un geste `off` (donnée pure, jamais exécuté par cette fonction ;
 *   c'est au consommateur de ne pas l'invoquer pour un bouton désactivé)
 * @property {Object} payload - MSG exact (`{}` pour flip/stopTimer/reveal,
 *   `{CARD_ID, WINNER_TEAM}` pour done — CARD_ID vient TOUJOURS de
 *   `selectedCardId`, y compris pour "annuler"/"sans vainqueur", R7 du plan)
 * @property {Array<number>|null} [color] - RGB de l'équipe (bouton REVEAL
 *   uniquement), à consommer via `getRgbColor`/`getContrastColor`
 */

/**
 * @param {string} subphase - `gameState.MEMOTION_SUBPHASE`
 * @param {Object} ctx
 * @param {boolean} ctx.timerRunning - `gameState.timer > 0` (chrono de la carte)
 * @param {string} ctx.currentTeam - `gameState.MEMOTION_CURRENT_TEAM` (peut être vide, mode SOLO)
 * @param {Array<number>|null} [ctx.currentTeamColor] - `gameState.MEMOTION_CURRENT_TEAM_COLOR`
 * @param {string} ctx.selectedCardId - `gameState.MEMOTION_SELECTED`
 * @param {number} [ctx.cardPoints] - `getMotionCardPoints(selectedCard.DIFFICULTY, question.MOTION_CONFIG)`
 * @returns {MotionGesture[]}
 */
export function motionGestures(subphase, {
  timerRunning = false,
  currentTeam = '',
  currentTeamColor = null,
  selectedCardId = '',
  cardPoints = 0,
} = {}) {
  // R7 — CARD_ID doit TOUJOURS être renseigné pour MEMOTION_DONE, y compris
  // pour « annuler »/« sans vainqueur » : le moteur se fie à MEMOTION_SELECTED
  // côté serveur, mais l'interface envoie systématiquement la valeur connue.
  const donePayload = (winnerTeam) => ({ CARD_ID: selectedCardId, WINNER_TEAM: winnerTeam })

  if (subphase === 'SELECTED') {
    return [
      {
        key: 'start',
        label: 'DÉMARRER',
        subLabel: 'lance le chrono',
        state: 'go',
        action: 'MEMOTION_FLIP',
        payload: {},
        color: null,
      },
      {
        key: 'cancel',
        label: 'ANNULER',
        subLabel: 'rend la carte',
        state: 'optional',
        action: 'MEMOTION_DONE',
        payload: donePayload(''),
        color: null,
      },
    ]
  }

  if (subphase === 'QUESTION') {
    return [
      {
        key: 'stopTimer',
        label: 'STOP CHRONO',
        subLabel: timerRunning ? null : 'chrono à zéro',
        state: timerRunning ? 'optional' : 'off',
        action: 'MEMOTION_STOP_TIMER',
        payload: {},
        color: null,
      },
      {
        key: 'reveal',
        label: 'RÉVÉLER',
        subLabel: timerRunning ? 'chrono en cours' : null,
        state: timerRunning ? 'off' : 'go',
        action: 'MEMOTION_REVEAL',
        payload: {},
        color: null,
      },
      {
        key: 'noWinner',
        label: 'SANS VAINQUEUR',
        subLabel: null,
        state: 'optional',
        action: 'MEMOTION_DONE',
        payload: donePayload(''),
        color: null,
      },
    ]
  }

  if (subphase === 'REVEAL') {
    const gestures = []
    // ⚠️ Mode SOLO (currentTeam vide/absente) : ne retourner que PERSONNE —
    // pas de bouton "équipe courante" à afficher, même à l'état 'off'
    // (aucune équipe à nommer, et aucune AUTRE équipe n'est jamais proposée
    // — c'est la règle du jeu tenue par l'interface).
    if (currentTeam) {
      gestures.push({
        key: 'award',
        label: currentTeam,
        subLabel: `+${cardPoints} pt${cardPoints > 1 ? 's' : ''}`,
        state: 'go',
        action: 'MEMOTION_DONE',
        payload: donePayload(currentTeam),
        color: currentTeamColor,
      })
    }
    gestures.push({
      key: 'nobody',
      label: 'PERSONNE',
      subLabel: '0 pt',
      state: 'optional',
      action: 'MEMOTION_DONE',
      payload: donePayload(''),
      color: null,
    })
    return gestures
  }

  // MEMORIZE / GRID / sous-phase inconnue : aucun geste, le bandeau
  // d'information est rendu par AnimMotionActions lui-même.
  return []
}
