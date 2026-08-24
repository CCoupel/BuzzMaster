/**
 * motionCardLock — verrouillage du type d'une carte MEMOTION (#184/B-F4).
 *
 * `contracts/question-types.md` §3.2 — prédicat exact :
 *
 * > Une carte est déverrouillée tant qu'aucun de ses `OwnedFields` ne
 * > s'écarte de sa valeur de création.
 *
 * ⚠️ **Piège explicitement documenté par le contrat, à ne pas réintroduire** :
 * plusieurs `OwnedFields` de QCM naissent avec une valeur **non vide**
 * (`qcmHintThreshold1: 0.25`, `qcmPenalty1: 0.67`… — `formData` initial de
 * `QuestionsPage.jsx`). Un prédicat « au moins un `OwnedField` non vide »
 * verrouillerait donc une carte QCM **dès sa création** — SPEEDY seul y
 * échapperait (ses deux `OwnedFields` naissent vides), ce qui rendrait le
 * sélecteur inutilisable pour tout autre type dès le premier essai. Le
 * prédicat ci-dessous compare donc à la valeur de création, jamais à la
 * non-nullité.
 *
 * Champs communs à la carte (`rectoTheme`, `rectoImage`, `difficulty`,
 * `questionText`, `questionImage`, `pointsRule` — pas encore modélisé côté
 * JS en v7.0.0) **ne verrouillent jamais**, quel que soit leur contenu : ils
 * appartiennent à la carte, pas au type (contrat §3.1).
 *
 * Champs client (camelCase) d'un item `formData.motionCards[i]` — voir
 * `QuestionsPage.jsx` (`handleAddMotionCard`, mapping de chargement) :
 * `type`, `rectoTheme`, `rectoImage`, `difficulty`, `questionText`,
 * `questionImage`, `answerText`, `answerImage` (SPEEDY), `qcmAnswers`,
 * `qcmCorrect`, `qcmHintsEnabled`, `qcmHintThreshold1/2`, `qcmPenalty1/2` (QCM).
 */

// Valeurs de création des OwnedFields QCM — DOIVENT rester synchronisées avec
// `formData` initial et `handleAddMotionCard` de `QuestionsPage.jsx`. Un écart
// entre les deux ferait apparaître une carte neuve comme déjà verrouillée.
const QCM_CREATION_VALUES = {
  qcmCorrect: '',
  qcmHintsEnabled: false,
  qcmHintThreshold1: 0.25,
  qcmHintThreshold2: 0.125,
  qcmPenalty1: 0.67,
  qcmPenalty2: 0.33,
}
const QCM_ANSWER_COLORS = ['RED', 'GREEN', 'YELLOW', 'BLUE']

/**
 * @param {Object} card - item de `formData.motionCards` (champs camelCase)
 * @returns {boolean} true si au moins un OwnedField du type courant de la
 *   carte s'écarte de sa valeur de création (type verrouillé)
 */
export function isMotionCardTypeLocked(card) {
  const type = card?.type || 'SPEEDY'

  if (type === 'SPEEDY') {
    return !!(card?.answerText) || !!(card?.answerImage)
  }

  if (type === 'QCM') {
    const answers = card?.qcmAnswers || {}
    const hasAnswerText = QCM_ANSWER_COLORS.some(color => !!(answers[color]))
    if (hasAnswerText) return true
    return Object.entries(QCM_CREATION_VALUES).some(
      ([field, creationValue]) => (card?.[field] ?? creationValue) !== creationValue
    )
  }

  // ARDOISE/MEMORY (v7.1.0, #186/#187) : pas encore nestable en carte
  // (`nestable: false`, utils/questionTypeMeta.js) — aucun OwnedField de
  // carte à comparer aujourd'hui, jamais verrouillé par cette fonction.
  return false
}

/**
 * @param {Object} card
 * @returns {string} raison humaine affichée à côté du sélecteur désactivé —
 *   indique explicitement QUOI vider pour débloquer (jamais un bouton grisé
 *   sans explication, contrat §3.2 / plan B-F4).
 */
export function motionCardLockReason(card) {
  const type = card?.type || 'SPEEDY'
  if (type === 'SPEEDY') return 'Videz la face RÉPONSE pour pouvoir changer de type.'
  if (type === 'QCM') return 'Videz les 4 réponses QCM (et les réglages d\'indices) pour pouvoir changer de type.'
  return ''
}
