/**
 * nextQuestionFormat — formatage partagé de la question suivante (#163/F2,
 * #165), format imposé GATE 2 (D1) : "#<ID> <type>: <énoncé> <points>pt
 * <délai>s". Consommé par AnimPage.jsx (puce "Suivante", zone contexte) ET
 * AnimConductPanel.jsx (bouton "À suivre", zone conduite) — source unique,
 * jamais dupliquée (même donnée `nextQuestion`, cf. contrat NEXT_QUESTION).
 *
 * Séparé en deux fonctions (énoncé / points-délai) plutôt qu'une seule
 * chaîne : les deux appelants ont besoin d'aligner le second bloc à droite,
 * séparément du premier qui seul est tronqué (ellipsis).
 */

/**
 * @param {{ID?: string, TYPE?: string, QUESTION?: string}|null} nextQuestion
 * @returns {string} "#<ID> <type>: <énoncé>" — chaîne vide si pas de question suivante
 */
export function formatNextQuestionStatement(nextQuestion) {
  if (!nextQuestion?.ID) return ''
  return `#${nextQuestion.ID} ${nextQuestion.TYPE || 'SPEEDY'}: ${nextQuestion.QUESTION || ''}`
}

/**
 * @param {{ID?: string, POINTS?: number, TIME?: number}|null} nextQuestion
 * @returns {string} "<points>pt <délai>s" — chaîne vide si pas de question suivante
 */
export function formatNextQuestionMeta(nextQuestion) {
  if (!nextQuestion?.ID) return ''
  return `${nextQuestion.POINTS ?? 0}pt ${nextQuestion.TIME ?? 0}s`
}
