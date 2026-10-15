/**
 * pointsRuleEditor — état client du barème `POINTS_RULE` d'une carte MEMOTION
 * dans l'éditeur (`QuestionsPage.jsx`, #190, contrat question-types.md §6.2).
 *
 * État d'une carte : `pointsRule: { mode, value }` où `mode: ''` = défaut du
 * type = champ `POINTS_RULE` ABSENT à l'envoi (jamais écrit). Fonctions pures,
 * sans dépendance React.
 */
import { allowedPointsRuleModes, defaultPointsRuleMode } from './motionGrid'

export const POINTS_RULE_VALUE_MIN = 1
export const POINTS_RULE_VALUE_MAX = 999

/** Libellé d'un mode de barème pour un type de carte (select + avertissements). */
export function pointsRuleModeLabel(mode, cardType) {
  const defaultMode = defaultPointsRuleMode(cardType)
  const isProgression = defaultMode === 'STARS_PRORATA'
  if (mode === '' || mode === defaultMode) return isProgression ? 'Prorata des étoiles (défaut)' : 'Étoiles (défaut)'
  switch (mode) {
    case 'STARS': return 'Étoiles pleines (tout ou rien)'
    case 'FIXED': return 'Montant fixe'
    case 'PER_UNIT': return 'Par unité'
    default: return mode
  }
}

export function isValidPointsRuleValue(v) {
  return Number.isInteger(v) && v >= POINTS_RULE_VALUE_MIN && v <= POINTS_RULE_VALUE_MAX
}

const needsValue = (mode) => mode === 'FIXED' || mode === 'PER_UNIT'

/**
 * Champs d'état d'une carte chargée depuis `card.POINTS_RULE` (forme serveur).
 * Mode absent, égal au défaut du type, ou inapplicable au type → `''`
 * (inapplicable : avertissement). Résolution serveur identique pour le défaut.
 *
 * @param {{TYPE?: string, POINTS_RULE?: {MODE?: string, VALUE?: number}|null}} card
 * @returns {{pointsRule: {mode: string, value: number|''}, pointsRuleNotice: string|null, pointsRuleError: null}}
 */
export function loadMotionCardPointsRule(card) {
  const type = card?.TYPE || 'SPEEDY'
  const mode = card?.POINTS_RULE?.MODE || ''
  const empty = { mode: '', value: '' }
  if (!mode || mode === defaultPointsRuleMode(type)) {
    return { pointsRule: empty, pointsRuleNotice: null, pointsRuleError: null }
  }
  if (!allowedPointsRuleModes(type).includes(mode)) {
    return {
      pointsRule: empty,
      pointsRuleNotice: `Le barème « ${pointsRuleModeLabel(mode, 'MEMORY')} » ne s'applique pas à ce type de carte : remis à « ${pointsRuleModeLabel('', type)} ».`,
      pointsRuleError: null,
    }
  }
  return {
    pointsRule: { mode, value: needsValue(mode) ? (card.POINTS_RULE.VALUE ?? '') : '' },
    pointsRuleNotice: null,
    pointsRuleError: null,
  }
}

/**
 * Champs à fusionner dans une carte lors d'un changement de type : règle encore
 * applicable → conservée (normalisée en `''` si elle égale le défaut du nouveau
 * type) ; inapplicable → défaut + avertissement bref.
 *
 * @param {{mode: string, value: number|''}} rule - règle actuelle de la carte
 * @param {string} newType
 * @returns {{pointsRule: {mode: string, value: number|''}, pointsRuleNotice: string|null, pointsRuleError: null}}
 */
export function adaptPointsRuleToType(rule, newType) {
  const mode = rule?.mode || ''
  if (!mode) return { pointsRule: { mode: '', value: '' }, pointsRuleNotice: null, pointsRuleError: null }
  if (!allowedPointsRuleModes(newType).includes(mode)) {
    return {
      pointsRule: { mode: '', value: '' },
      pointsRuleNotice: `Le barème « ${pointsRuleModeLabel(mode, 'MEMORY')} » n'existe pas pour ce type de carte : remis à « ${pointsRuleModeLabel('', newType)} ».`,
      pointsRuleError: null,
    }
  }
  if (mode === defaultPointsRuleMode(newType)) {
    return { pointsRule: { mode: '', value: '' }, pointsRuleNotice: null, pointsRuleError: null }
  }
  return { pointsRule: rule, pointsRuleNotice: null, pointsRuleError: null }
}

/**
 * Message d'erreur si Montant fixe / Par unité sans montant entier 1–999, sinon `null`.
 *
 * @param {{mode: string, value: number|''}} rule
 * @returns {string|null}
 */
export function pointsRuleValueError(rule) {
  const mode = rule?.mode || ''
  if (!needsValue(mode) || isValidPointsRuleValue(rule.value)) return null
  const what = mode === 'PER_UNIT' ? 'un montant par unité' : 'un montant'
  return `Indiquez ${what} (${POINTS_RULE_VALUE_MIN} à ${POINTS_RULE_VALUE_MAX}) — la carte ne peut pas être enregistrée sans.`
}

/**
 * Fragment de sérialisation (`{ POINTS_RULE }` ou `{}`) : mode `''` → rien ;
 * `STARS` / `STARS_PRORATA` → `{ MODE }` ; `FIXED` / `PER_UNIT` → `{ MODE, VALUE }`.
 *
 * @param {{mode: string, value: number|''}} rule
 * @returns {{POINTS_RULE?: {MODE: string, VALUE?: number}}}
 */
export function serializePointsRule(rule) {
  const mode = rule?.mode || ''
  if (!mode) return {}
  if (needsValue(mode)) return { POINTS_RULE: { MODE: mode, VALUE: Number(rule.value) } }
  return { POINTS_RULE: { MODE: mode } }
}

/**
 * `formData.motionConfig` (points1/2/3 saisis, camelCase) → forme serveur
 * (`POINTS_n_STAR`) attendue par `getMotionCardPoints` pour l'aide « → N pts ».
 */
export function editorMotionConfig(cfg) {
  return {
    POINTS_1_STAR: parseInt(cfg?.points1) || 1,
    POINTS_2_STAR: parseInt(cfg?.points2) || 3,
    POINTS_3_STAR: parseInt(cfg?.points3) || 5,
  }
}
