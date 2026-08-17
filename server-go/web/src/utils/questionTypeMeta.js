/**
 * questionTypeMeta — icônes de type de question pour `/anim` (#166/F2, D4).
 * Icônes validées au GATE 2 (issue #166) : ⚡ SPEEDY, 🔠 QCM, 🖊️ ARDOISE,
 * 🃏 MEMORY, 🎞️ MEMOTION. Source unique — ne pas réécrire ce mapping
 * ailleurs (même piège que QCM_COLORS, #149/#155).
 */

export const QUESTION_TYPE_META = {
  SPEEDY: { icon: '⚡', label: 'Speedy' },
  QCM: { icon: '🔠', label: 'QCM' },
  ARDOISE: { icon: '🖊️', label: 'Ardoise' },
  MEMORY: { icon: '🃏', label: 'Memory' },
  MEMOTION: { icon: '🎞️', label: 'Memotion' },
}

const DEFAULT_TYPE_META = { icon: '⚡', label: 'Speedy' }

/**
 * @param {string} [type] - question.TYPE (ou nextQuestion.TYPE)
 * @returns {{icon: string, label: string}} - repli SPEEDY si type inconnu/absent
 *   (même convention que le repli textuel déjà en place ailleurs sur /anim :
 *   `question.TYPE || 'SPEEDY'`, #163).
 */
export function getQuestionTypeMeta(type) {
  return QUESTION_TYPE_META[type] || DEFAULT_TYPE_META
}
