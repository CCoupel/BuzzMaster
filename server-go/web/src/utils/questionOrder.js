/**
 * questionOrder — shared question-ordering logic (#149, plan tâche 25.1).
 *
 * Mutualise le tri auparavant dupliqué à l'identique dans
 * QuestionsPage.jsx et GamePage.jsx (tri sur ORDER, repli sur ID) — toute
 * option de #149 devait d'abord mutualiser ce tri, faute de quoi un
 * correctif divergerait entre les deux pages.
 */

/**
 * Sorts questions by their ORDER field, falling back to ID when ORDER is
 * absent (questions créées avant l'introduction du champ, ou jamais
 * réordonnées).
 *
 * @param {Object.<string, object>} questionsById - `questions` map from useGame()
 * @returns {Array} Questions triées, uniquement celles avec un ID valide
 */
export function sortQuestionsByOrder(questionsById) {
  return Object.values(questionsById || {})
    .filter(q => q && q.ID)
    .sort((a, b) => {
      const orderA = a.ORDER !== undefined ? parseInt(a.ORDER) : parseInt(a.ID)
      const orderB = b.ORDER !== undefined ? parseInt(b.ORDER) : parseInt(b.ID)
      return orderA - orderB
    })
}

/**
 * Unbiased Fisher-Yates shuffle — returns a NEW array, never mutates the
 * input. Contrairement au mélange seedé de PlayerDisplay.jsx:731-741 (choisi
 * là-bas pour être reproductible entre clients à partir du même ID de
 * question), ce mélange n'a pas besoin de seed : la reproductibilité entre
 * écrans vient ici de la PERSISTANCE de l'ordre choisi (REORDER_QUESTIONS),
 * pas du générateur — chaque appel doit donc être un tirage réellement
 * aléatoire, pas déterministe.
 *
 * @param {Array} array
 * @returns {Array} A shuffled copy of `array`
 */
export function shuffleArray(array) {
  const result = [...array]
  for (let i = result.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1))
    ;[result[i], result[j]] = [result[j], result[i]]
  }
  return result
}
