/**
 * motionCardLock — verrouillage du type d'une carte MEMOTION (#184/B-F4,
 * étendu #187).
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
 * `qcmCorrect`, `qcmHintsEnabled`, `qcmHintThreshold1/2`, `qcmPenalty1/2` (QCM),
 * `memoryMode`, `memoryPairs`, `memoryConfig` (MEMORY, #187).
 *
 * ⚠️ **MEMORY_PAIRS n'est pas listé dans le tableau de verrouillage du
 * contrat §3.2** (seuls `MEMORY_MODE` et `MEMORY_CONFIG` y figurent), mais il
 * appartient bien aux `OwnedFields` MEMORY (§3.1) et suit le même motif que
 * `QCM_ANSWERS` : une carte dont au moins une paire porte du contenu
 * (texte/image) doit verrouiller, sinon la destruction silencieuse que le
 * verrou existe pour empêcher redeviendrait possible (ajouter 8 paires
 * illustrées puis basculer librement vers SPEEDY). Traité ici comme un
 * `OwnedField` à part entière — écart assumé par rapport à la lettre du
 * tableau §3.2, cohérent avec son esprit (§3.1) et avec `QCM_ANSWERS`.
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

// Valeurs de création des OwnedFields MEMORY (#187) — contrat §3.2 :
// MEMORY_MODE naît à "SOLO", MEMORY_CONFIG naît avec ses 8 réglages par
// défaut (DOIT rester synchronisé avec `formData`/`handleAddMotionCard` de
// `QuestionsPage.jsx`, même piège que QCM_CREATION_VALUES ci-dessus).
const MEMORY_MODE_CREATION_VALUE = 'SOLO'
const MEMORY_CONFIG_CREATION_VALUES = {
  flipDelay: 3,
  pointsPerPair: 10,
  errorPenalty: 0,
  completionBonus: 0,
  useTimer: true,
  memorizeTime: 5,
  showDuringMemorize: true,
  revealDelay: 0.5,
}

// Valeurs de création des OwnedFields RAFALE (#217, milestone v9.0.0) —
// DOIT rester synchronisé avec `formData`/`handleAddMotionCard` de
// QuestionsPage.jsx, même piège que QCM/MEMORY ci-dessus. Pas de
// rafaleMode ni rafalePointsByDifficulty ici : mode SOLO forcé, et barème
// par difficulté sans objet en carte (contrat rafale.md §14.2 — STARS_PRORATA
// sur le DIFFICULTY commun de la carte, comme MEMORY) — ni l'un ni l'autre
// n'est un champ de formulaire pour une carte, rien à comparer.
const RAFALE_CREATION_VALUES = {
  rafaleRoundTime: 120,
  rafaleQuestionTime: 3,
  rafaleMaxQuestions: 100,
}

// Une paire est "vide" (valeur de création) si aucune de ses deux cartes ne
// porte de texte ni d'image — même notion que "réponse QCM saisie" pour
// QCM_ANSWERS.
function isMemoryPairEmpty(pair) {
  const side1Empty = !pair?.card1?.text && !pair?.card1?.image
  const side2Empty = !pair?.card2?.text && !pair?.card2?.image
  return side1Empty && side2Empty
}

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

  if (type === 'MEMORY') {
    const hasPairContent = (card?.memoryPairs || []).some(pair => !isMemoryPairEmpty(pair))
    if (hasPairContent) return true
    if ((card?.memoryMode ?? MEMORY_MODE_CREATION_VALUE) !== MEMORY_MODE_CREATION_VALUE) return true
    const config = card?.memoryConfig || {}
    return Object.entries(MEMORY_CONFIG_CREATION_VALUES).some(
      ([field, creationValue]) => (config[field] ?? creationValue) !== creationValue
    )
  }

  if (type === 'RAFALE') {
    if ((card?.rafaleCategories || []).length > 0) return true
    if ((card?.rafaleDifficulties || []).length > 0) return true
    return Object.entries(RAFALE_CREATION_VALUES).some(
      ([field, creationValue]) => (card?.[field] ?? creationValue) !== creationValue
    )
  }

  // ARDOISE (#186 fermée « not planned ») : jamais nestable
  // (`nestable: false`, utils/questionTypeMeta.js) — aucun OwnedField de
  // carte à comparer, jamais verrouillé par cette fonction.
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
  if (type === 'MEMORY') return 'Videz les paires MEMORY (et la configuration) pour pouvoir changer de type.'
  if (type === 'RAFALE') return 'Videz les catégories/difficultés RAFALE (et la configuration) pour pouvoir changer de type.'
  return ''
}
