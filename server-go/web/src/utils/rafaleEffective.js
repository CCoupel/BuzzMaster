/**
 * Rétro-compatibilité mono → liste pour le filtre de manche RAFALE (#216,
 * milestone v9.0.0, réouverture assumée de #107 — contrat rafale.md §3.3).
 *
 * Précédent côté Go : `Question.EffectiveRafaleCategories()`/
 * `EffectiveRafaleDifficulties()` — une question RAFALE enregistrée avant
 * #216 (CATEGORY/RAFALE_DIFFICULTY mono, RAFALE_CATEGORIES/RAFALE_DIFFICULTIES
 * absents ou vides) doit continuer de fonctionner à l'identique sans
 * reconfiguration (216-Q6). Toute consommation côté frontend d'un filtre
 * RAFALE (QuestionsPage.jsx à l'édition, GamePage.jsx avant/pendant le
 * lancement, QuestionCard.jsx à l'affichage) passe par ces deux fonctions —
 * jamais par les champs bruts directement — même discipline que côté
 * serveur (contrat §11/§12, point 15 : "jamais les champs bruts").
 *
 * @param {Object} question - une Question (ou objet compatible : au moins
 *   CATEGORY, RAFALE_DIFFICULTY, RAFALE_CATEGORIES, RAFALE_DIFFICULTIES)
 * @returns {string[]}
 */
export function effectiveRafaleCategories(question) {
  if (question?.RAFALE_CATEGORIES && question.RAFALE_CATEGORIES.length > 0) {
    return question.RAFALE_CATEGORIES
  }
  return question?.CATEGORY ? [question.CATEGORY] : []
}

/**
 * @param {Object} question
 * @returns {number[]}
 */
export function effectiveRafaleDifficulties(question) {
  if (question?.RAFALE_DIFFICULTIES && question.RAFALE_DIFFICULTIES.length > 0) {
    return question.RAFALE_DIFFICULTIES
  }
  const d = question?.RAFALE_DIFFICULTY
  return (d >= 1 && d <= 3) ? [d] : []
}
