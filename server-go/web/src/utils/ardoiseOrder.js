/**
 * ardoiseOrder — tri et délai ARDOISE, source unique consommée par
 * `/admin` (GamePage.jsx) ET `/anim` (AnimArdoiseList.jsx, #158/F1).
 * Extraction pure — comportement `/admin` strictement inchangé (même
 * piège déjà évité par #149, #155, #165, #170 : ne jamais dupliquer une
 * règle déjà écrite).
 */

/**
 * Délai depuis le lancement de la question, à partir de l'horodatage de
 * première frappe (#117). Reprend la convention temps de réaction des
 * buzzers (microsecondes -> secondes, 3 décimales).
 *
 * @param {{STARTED_AT?: number}|null} answer - gameState.ARDOISE_ANSWERS[teamName]
 * @param {number} gameTime - gameState.gameTime (référence de départ de la question)
 * @returns {string|null} "X.XXX s", ou `null` si rien de significatif à afficher
 *   (pas de STARTED_AT, pas de gameTime de référence, ou délai négatif —
 *   resynchronisation/rejeu)
 */
export function formatArdoiseDelay(answer, gameTime) {
  if (!answer || !answer.STARTED_AT || !gameTime) return null
  const delaySeconds = (answer.STARTED_AT - gameTime) / 1000000
  if (!Number.isFinite(delaySeconds) || delaySeconds < 0) return null
  return `${delaySeconds.toFixed(3)} s`
}

/**
 * Équipes avec réponse d'abord, triées par ordre de première frappe ;
 * repli sur SUBMITTED_AT quand STARTED_AT vaut 0 (réponses enregistrées
 * avant ce correctif) ; équipes sans réponse en fin, dans l'ordre de la
 * liste d'équipes fournie.
 *
 * @param {Array<{name?: string, NAME?: string}>} teams - liste d'équipes (déjà filtrée VJoueur)
 * @param {Object} [ardoiseAnswers] - gameState.ARDOISE_ANSWERS (Map teamName -> {STARTED_AT, SUBMITTED_AT, TEXT})
 * @returns {Array<{team: Object, teamName: string, answer: Object|null}>}
 */
export function sortArdoiseEntries(teams, ardoiseAnswers) {
  const answers = ardoiseAnswers || {}
  const answered = []
  const unanswered = []
  teams.forEach((team, idx) => {
    const teamName = team.NAME || team.name
    const answer = answers[teamName]
    if (answer) {
      const orderKey = answer.STARTED_AT > 0 ? answer.STARTED_AT : answer.SUBMITTED_AT
      answered.push({ team, teamName, answer, orderKey, idx })
    } else {
      unanswered.push({ team, teamName, answer: null, idx })
    }
  })
  answered.sort((a, b) => (a.orderKey - b.orderKey) || (a.idx - b.idx))
  return [...answered, ...unanswered]
}
