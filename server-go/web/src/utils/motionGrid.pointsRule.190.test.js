import { describe, it, expect } from 'vitest'
import {
  allowedPointsRuleModes,
  defaultPointsRuleMode,
  resolvePointsRuleMode,
  getMotionCardPointsForRule,
  describePointsRule,
} from './motionGrid'

// ---------------------------------------------------------------------------
// motionGrid — barème par règle POINTS_RULE (#190, LOT-5, v11.2.2, contrat
// question-types.md §6.2 « Applicabilité par type et validation à
// l'enregistrement »).
//
// Classe de risque R5 (cellule dérivable des DEUX côtés Go/JS) : les 8 cas
// ci-dessous portent EXACTEMENT les mêmes noms que ceux de
// internal/game/engine_points_rule_190_test.go (dev-backend), sur
// `motionCardPointsForOutcome` — si l'un des deux côtés dérive, le cas nommé
// identique échoue des deux côtés :
//   STARS_PRORATA_5points_8pairs  FIXED_10_won  FIXED_10_lost  PER_UNIT_2x3
//   DEFAULT_SPEEDY_is_STARS  DEFAULT_MEMORY_is_STARS_PRORATA
//   DEFAULT_RAFALE_is_STARS_PRORATA  EXPLICIT_STARS_on_MEMORY_full_value
//
// `card` = carte telle que sérialisée par le serveur (TYPE, DIFFICULTY,
// POINTS_RULE: { MODE, VALUE }) ; sans MOTION_CONFIG le barème étoiles est le
// repli 1/3/5 (★ = 1, ★★ = 3, ★★★ = 5).
// ---------------------------------------------------------------------------

const card = (over = {}) => ({ ID: 'c1', DIFFICULTY: 2, ...over })

describe('getMotionCardPointsForRule — 8 cas nommés à l\'identique du Go (R5)', () => {
  it('STARS_PRORATA_5points_8pairs', () => {
    // Carte MEMORY ★★★ (5 pts), 8 paires — multiplier AVANT de diviser.
    const c = card({ TYPE: 'MEMORY', DIFFICULTY: 3, POINTS_RULE: { MODE: 'STARS_PRORATA' } })
    expect(getMotionCardPointsForRule(c, null, { units: 4, unitsTotal: 8 })).toBe(2)
    expect(getMotionCardPointsForRule(c, null, { units: 8, unitsTotal: 8 })).toBe(5)
    expect(getMotionCardPointsForRule(c, null, { units: 1, unitsTotal: 8 })).toBe(0)
  })

  it('FIXED_10_won', () => {
    const c = card({ TYPE: 'QCM', DIFFICULTY: 1, POINTS_RULE: { MODE: 'FIXED', VALUE: 10 } })
    expect(getMotionCardPointsForRule(c, null, { units: 1 })).toBe(10)
  })

  it('FIXED_10_lost', () => {
    const c = card({ TYPE: 'QCM', DIFFICULTY: 1, POINTS_RULE: { MODE: 'FIXED', VALUE: 10 } })
    expect(getMotionCardPointsForRule(c, null, { units: 0 })).toBe(0)
  })

  it('PER_UNIT_2x3', () => {
    const c = card({ TYPE: 'RAFALE', POINTS_RULE: { MODE: 'PER_UNIT', VALUE: 2 } })
    expect(getMotionCardPointsForRule(c, null, { units: 3, unitsTotal: 5 })).toBe(6)
  })

  it('DEFAULT_SPEEDY_is_STARS', () => {
    // Aucune POINTS_RULE : carte SPEEDY ★★ = 3 pts (barème étoiles), avec ou sans TYPE.
    expect(getMotionCardPointsForRule(card({ TYPE: 'SPEEDY' }), null)).toBe(3)
    expect(getMotionCardPointsForRule(card({}), null)).toBe(3)
  })

  it('DEFAULT_MEMORY_is_STARS_PRORATA', () => {
    // Aucune POINTS_RULE : le défaut du TYPE MEMORY est STARS_PRORATA.
    const c = card({ TYPE: 'MEMORY', DIFFICULTY: 3 })
    expect(getMotionCardPointsForRule(c, null, { units: 4, unitsTotal: 8 })).toBe(2)
  })

  it('DEFAULT_RAFALE_is_STARS_PRORATA', () => {
    // Aucune POINTS_RULE : le défaut du TYPE RAFALE est STARS_PRORATA (rafale.md §14.4).
    const c = card({ TYPE: 'RAFALE', DIFFICULTY: 2 })
    expect(getMotionCardPointsForRule(c, null, { units: 2, unitsTotal: 3 })).toBe(2)
  })

  it('EXPLICIT_STARS_on_MEMORY_full_value', () => {
    // STARS explicite sur MEMORY = tout-ou-rien : valeur étoiles PLEINE même grille incomplète.
    const c = card({ TYPE: 'MEMORY', DIFFICULTY: 3, POINTS_RULE: { MODE: 'STARS' } })
    expect(getMotionCardPointsForRule(c, null, { units: 1, unitsTotal: 8 })).toBe(5)
  })
})

describe('getMotionCardPointsForRule — variantes et gardes', () => {
  it('units vaut 1 par défaut (type binaire : gagné)', () => {
    expect(getMotionCardPointsForRule(card({ TYPE: 'QCM', POINTS_RULE: { MODE: 'FIXED', VALUE: 7 } }), null)).toBe(7)
    expect(getMotionCardPointsForRule(card({ TYPE: 'SPEEDY', DIFFICULTY: 3 }), null)).toBe(5)
  })

  it('STARS_PRORATA : unitsTotal <= 0 → 0 point, jamais de division par zéro (garde contrat §6.2)', () => {
    const c = card({ TYPE: 'RAFALE', DIFFICULTY: 3 })
    expect(getMotionCardPointsForRule(c, null, { units: 0, unitsTotal: 0 })).toBe(0)
    expect(getMotionCardPointsForRule(c, null, { units: 2, unitsTotal: 0 })).toBe(0)
  })

  it('MOTION_CONFIG personnalisé : le barème étoiles suit POINTS_<n>_STAR pour STARS et STARS_PRORATA', () => {
    const cfg = { POINTS_1_STAR: 2, POINTS_2_STAR: 8, POINTS_3_STAR: 15 }
    expect(getMotionCardPointsForRule(card({ TYPE: 'SPEEDY', DIFFICULTY: 2 }), cfg)).toBe(8)
    const memory = card({ TYPE: 'MEMORY', DIFFICULTY: 3 })
    expect(getMotionCardPointsForRule(memory, cfg, { units: 4, unitsTotal: 8 })).toBe(7) // floor(15*4/8)
  })

  it('FIXED et PER_UNIT ignorent la difficulté et MOTION_CONFIG (« quelle que soit la valeur ★ »)', () => {
    const cfg = { POINTS_1_STAR: 99, POINTS_2_STAR: 99, POINTS_3_STAR: 99 }
    const fixed = card({ TYPE: 'SPEEDY', DIFFICULTY: 3, POINTS_RULE: { MODE: 'FIXED', VALUE: 4 } })
    expect(getMotionCardPointsForRule(fixed, cfg, { units: 1 })).toBe(4)
    const perUnit = card({ TYPE: 'MEMORY', DIFFICULTY: 1, POINTS_RULE: { MODE: 'PER_UNIT', VALUE: 3 } })
    expect(getMotionCardPointsForRule(perUnit, cfg, { units: 2, unitsTotal: 8 })).toBe(6)
  })

  it('PER_UNIT : 0 unité → 0 point', () => {
    const c = card({ TYPE: 'MEMORY', POINTS_RULE: { MODE: 'PER_UNIT', VALUE: 5 } })
    expect(getMotionCardPointsForRule(c, null, { units: 0, unitsTotal: 8 })).toBe(0)
  })

  it('règle explicite = défaut du type : même montant que la carte sans règle (idempotence)', () => {
    const implicit = card({ TYPE: 'MEMORY', DIFFICULTY: 3 })
    const explicit = card({ TYPE: 'MEMORY', DIFFICULTY: 3, POINTS_RULE: { MODE: 'STARS_PRORATA' } })
    const outcome = { units: 5, unitsTotal: 8 }
    expect(getMotionCardPointsForRule(explicit, null, outcome)).toBe(getMotionCardPointsForRule(implicit, null, outcome))
  })

  it('carte sans POINTS_RULE ni TYPE (question MEMOTION antérieure à #184) : comportement historique, barème étoiles', () => {
    expect(getMotionCardPointsForRule({ ID: 'old', DIFFICULTY: 1 }, null)).toBe(1)
    expect(getMotionCardPointsForRule({ ID: 'old', DIFFICULTY: 3 }, null)).toBe(5)
  })
})

describe('allowedPointsRuleModes / defaultPointsRuleMode — miroir de AllowedPointsRuleModes (Go)', () => {
  it.each(['SPEEDY', 'QCM'])('type binaire %s : STARS (défaut) puis FIXED', (type) => {
    expect(allowedPointsRuleModes(type)).toEqual(['STARS', 'FIXED'])
    expect(defaultPointsRuleMode(type)).toBe('STARS')
  })

  it.each(['MEMORY', 'RAFALE'])('type à progression %s : STARS_PRORATA (défaut), STARS, FIXED, PER_UNIT', (type) => {
    expect(allowedPointsRuleModes(type)).toEqual(['STARS_PRORATA', 'STARS', 'FIXED', 'PER_UNIT'])
    expect(defaultPointsRuleMode(type)).toBe('STARS_PRORATA')
  })

  it('TYPE absent / vide ⇒ SPEEDY (binaire)', () => {
    expect(allowedPointsRuleModes(undefined)).toEqual(['STARS', 'FIXED'])
    expect(allowedPointsRuleModes('')).toEqual(['STARS', 'FIXED'])
    expect(defaultPointsRuleMode(undefined)).toBe('STARS')
  })

  it('le défaut est toujours le premier mode proposé, et fait partie des modes acceptés', () => {
    for (const type of ['SPEEDY', 'QCM', 'MEMORY', 'RAFALE']) {
      const modes = allowedPointsRuleModes(type)
      expect(modes[0]).toBe(defaultPointsRuleMode(type))
      expect(modes).toContain(defaultPointsRuleMode(type))
    }
  })

  it('STARS_PRORATA et PER_UNIT ne sont jamais proposés à un type binaire', () => {
    for (const type of ['SPEEDY', 'QCM']) {
      expect(allowedPointsRuleModes(type)).not.toContain('STARS_PRORATA')
      expect(allowedPointsRuleModes(type)).not.toContain('PER_UNIT')
    }
  })
})

describe('resolvePointsRuleMode — miroir de la résolution moteur (explicite > défaut du TYPE)', () => {
  it('POINTS_RULE.MODE explicite l\'emporte sur le défaut du type', () => {
    expect(resolvePointsRuleMode(card({ TYPE: 'MEMORY', POINTS_RULE: { MODE: 'FIXED', VALUE: 3 } }))).toBe('FIXED')
    expect(resolvePointsRuleMode(card({ TYPE: 'SPEEDY', POINTS_RULE: { MODE: 'FIXED', VALUE: 3 } }))).toBe('FIXED')
  })

  it('sans règle : défaut du TYPE courant (résolu à chaque appel, jamais figé)', () => {
    expect(resolvePointsRuleMode(card({ TYPE: 'MEMORY' }))).toBe('STARS_PRORATA')
    expect(resolvePointsRuleMode(card({ TYPE: 'RAFALE' }))).toBe('STARS_PRORATA')
    expect(resolvePointsRuleMode(card({ TYPE: 'QCM' }))).toBe('STARS')
    expect(resolvePointsRuleMode(card({}))).toBe('STARS')
  })

  it('POINTS_RULE vide ou sans MODE : défaut du type', () => {
    expect(resolvePointsRuleMode(card({ TYPE: 'MEMORY', POINTS_RULE: {} }))).toBe('STARS_PRORATA')
    expect(resolvePointsRuleMode(card({ TYPE: 'MEMORY', POINTS_RULE: null }))).toBe('STARS_PRORATA')
  })
})

describe('describePointsRule — libellés de l\'aide « → … » (maquette admin_question_editor__memotion_card_points_rule)', () => {
  it('STARS : « 3 pts (★★) »', () => {
    expect(describePointsRule(card({ TYPE: 'SPEEDY', DIFFICULTY: 2 }), null)).toBe('3 pts (★★)')
    expect(describePointsRule(card({ TYPE: 'QCM', DIFFICULTY: 3 }), null)).toBe('5 pts (★★★)')
  })

  it('STARS suit MOTION_CONFIG', () => {
    expect(describePointsRule(card({ TYPE: 'SPEEDY', DIFFICULTY: 2 }), { POINTS_2_STAR: 8 })).toBe('8 pts (★★)')
  })

  it('FIXED : « 10 pts, quelle que soit la valeur ★ »', () => {
    const c = card({ TYPE: 'QCM', DIFFICULTY: 1, POINTS_RULE: { MODE: 'FIXED', VALUE: 10 } })
    expect(describePointsRule(c, null)).toBe('10 pts, quelle que soit la valeur ★')
  })

  it('STARS_PRORATA MEMORY : « jusqu\'à 5 pts, au prorata des paires trouvées (8 paires) »', () => {
    const c = card({ TYPE: 'MEMORY', DIFFICULTY: 3 })
    expect(describePointsRule(c, null, { unitsTotal: 8 })).toBe('jusqu\'à 5 pts, au prorata des paires trouvées (8 paires)')
  })

  it('STARS_PRORATA RAFALE : « jusqu\'à 5 pts, au prorata des bonnes réponses » (pas de total de paires)', () => {
    const c = card({ TYPE: 'RAFALE', DIFFICULTY: 3 })
    expect(describePointsRule(c, null)).toBe('jusqu\'à 5 pts, au prorata des bonnes réponses')
  })

  it('PER_UNIT RAFALE : « 2 pts × bonnes réponses »', () => {
    const c = card({ TYPE: 'RAFALE', POINTS_RULE: { MODE: 'PER_UNIT', VALUE: 2 } })
    expect(describePointsRule(c, null)).toBe('2 pts × bonnes réponses')
  })

  it('PER_UNIT MEMORY : « 2 pts × paires trouvées »', () => {
    const c = card({ TYPE: 'MEMORY', POINTS_RULE: { MODE: 'PER_UNIT', VALUE: 2 } })
    expect(describePointsRule(c, null, { unitsTotal: 8 })).toBe('2 pts × paires trouvées')
  })

  it('FIXED / PER_UNIT sans VALUE valide (saisie en cours) : « N pts … », jamais NaN ni undefined', () => {
    for (const rule of [{ MODE: 'FIXED', VALUE: '' }, { MODE: 'FIXED' }, { MODE: 'FIXED', VALUE: 0 }, { MODE: 'FIXED', VALUE: 1000 }]) {
      const text = describePointsRule(card({ TYPE: 'QCM', POINTS_RULE: rule }), null)
      expect(text).toMatch(/^N pts, quelle que soit la valeur ★$/)
      expect(text).not.toMatch(/NaN|undefined/)
    }
    expect(describePointsRule(card({ TYPE: 'RAFALE', POINTS_RULE: { MODE: 'PER_UNIT', VALUE: '' } }), null)).toBe('N pts × bonnes réponses')
  })
})
