/**
 * phaseRules — règles d'activation dérivées de `gameState.phase`, source
 * UNIQUE pour `/admin` (GamePage.jsx) ET `/anim` (AnimConductPanel.jsx,
 * AnimNextButton.jsx, AnimAnswerZone.jsx) — #166/F4b.
 *
 * Extraction pure de logique déjà en place, dupliquée à trois endroits
 * de GamePage.jsx (373-378, 463, 723) et miroir de la garde serveur
 * `Engine.Ready` (`engine.go:585-593`, "Allow from: STOPPED, REVEALED,
 * PREPARE, READY, NEW_GAME"). Aucun comportement `/admin` ne doit changer
 * en consommant ce fichier — c'est un déplacement de code, pas une
 * réécriture (point de revue R1-b, #166).
 *
 * Toute nouvelle condition d'activation sur `/anim` (matrice L1 +
 * "à suivre", #166/F5/F4) doit être dérivée d'ici — jamais réécrite
 * localement dans un composant (point de revue R1-a, #166).
 */

// Phases où le moteur accepte Ready() (sélection/enchaînement de question) —
// miroir exact de engine.go:585-593.
export const SELECT_QUESTION_PHASES = ['STOPPED', 'REVEALED', 'PREPARE', 'READY', 'NEW_GAME']

/** @param {string} phase - gameState.phase */
export function canSelectQuestion(phase) {
  return SELECT_QUESTION_PHASES.includes(phase)
}

/** @param {string} phase - gameState.phase */
export function canStart(phase) {
  return phase === 'READY'
}

/** @param {string} phase - gameState.phase */
export function isPlaying(phase) {
  return phase === 'STARTED' || phase === 'PAUSED'
}

/**
 * @param {string} phase - gameState.phase
 * @param {{STATUS?: string}|null} [question] - gameState.question
 */
export function canReveal(phase, question) {
  return phase === 'STOPPED' && question?.STATUS === 'STOPPED'
}

/** @param {string} phase - gameState.phase */
export function isRevealed(phase) {
  return phase === 'REVEALED'
}

// ---------------------------------------------------------------------------
// #166/F5/F4 — états de bouton dérivés (matrice de la maquette
// _work/mockups/anim-conduct-permanent-166.html, 10 phases × 5 boutons +
// "à suivre"). 'go' = vert (action attendue), 'optional' = bleu, 'danger' =
// rouge, 'off'/'inert' = gris, non cliquable, n'émet rien.
// ---------------------------------------------------------------------------

/** @param {string} phase - gameState.phase */
export function startButtonState(phase) {
  return canStart(phase) ? 'go' : 'off'
}

/** @param {string} phase - gameState.phase */
export function pauseButtonState(phase) {
  return phase === 'STARTED' ? 'optional' : 'off'
}

/** @param {string} phase - gameState.phase */
export function continueButtonState(phase) {
  return phase === 'PAUSED' ? 'go' : 'off'
}

/** @param {string} phase - gameState.phase */
export function stopButtonState(phase) {
  return isPlaying(phase) ? 'danger' : 'off'
}

/**
 * @param {string} phase - gameState.phase
 * @param {{STATUS?: string}|null} [question] - gameState.question
 */
export function revealButtonState(phase, question) {
  return canReveal(phase, question) ? 'go' : 'off'
}

/**
 * Bouton "à suivre" (AnimNextButton, #166/F4) — trois états :
 *   - 'inert'    : phase où Ready() est refusé côté moteur (ENROLL,
 *                  COUNTDOWN, STARTED, PAUSED) — présent mais n'émet rien.
 *   - 'optional' : une AUTRE action est déjà proposée à côté — LANCER en
 *                  READY, ou RÉPONSE en STOPPED "jouée" (question.STATUS
 *                  === 'STOPPED') — "à suivre" court-circuite cette autre
 *                  action plutôt que d'être le seul geste possible (bleu).
 *   - 'go'       : "à suivre" est la SEULE action possible pour faire
 *                  progresser la partie — NEW_GAME, PREPARE, STOPPED "non
 *                  jouée", REVEALED (vert).
 *
 * Correction (revue test-writer, coordination #166) : STOPPED est une
 * phase UNIQUE qui recouvre deux lignes distinctes de la matrice
 * ("jouée"/"non jouée", distinguées par `question.STATUS`) — la seule
 * façon de les séparer est de tester `canReveal`, pas seulement `phase`.
 * D'où la signature à deux paramètres (comme `revealButtonState`).
 *
 * @param {string} phase - gameState.phase
 * @param {{STATUS?: string}|null} [question] - gameState.question (la
 *   question COURANTE, pas `nextQuestion` — nécessaire pour `canReveal`)
 */
export function nextButtonState(phase, question) {
  if (!canSelectQuestion(phase)) return 'inert'
  return (canStart(phase) || canReveal(phase, question)) ? 'optional' : 'go'
}
