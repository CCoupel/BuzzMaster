import { describe, it, expect } from 'vitest'
import {
  calcQcmPenaltyForHints,
  calcQcmPenalty,
  calcMemoryScore,
  calcArdoiseDefaultPoints,
  resolvePointsTarget,
  resolvePointsAward,
} from './pointsAward'

// ========================================
// calcQcmPenaltyForHints — pénalité par joueur au moment du buzz
// ========================================

describe('calcQcmPenaltyForHints', () => {
  const qcmQuestion = { TYPE: 'QCM', QCM_HINTS_ENABLED: true }

  it('retourne null si la question n\'est pas un QCM', () => {
    expect(calcQcmPenaltyForHints({ TYPE: 'SPEEDY', QCM_HINTS_ENABLED: true }, 10, 1)).toBeNull()
  })

  it('retourne null si les indices ne sont pas activés', () => {
    expect(calcQcmPenaltyForHints({ TYPE: 'QCM', QCM_HINTS_ENABLED: false }, 10, 1)).toBeNull()
  })

  it('0 indice → montant plein (multiplier 1)', () => {
    const result = calcQcmPenaltyForHints(qcmQuestion, 10, 0)
    expect(result).toEqual({ multiplier: 1, effectivePoints: 10, penaltyPercent: 100 })
  })

  it('1 indice → pénalité par défaut 0.67', () => {
    const result = calcQcmPenaltyForHints(qcmQuestion, 10, 1)
    expect(result.multiplier).toBe(0.67)
    expect(result.effectivePoints).toBe(7) // round(10 * 0.67)
    expect(result.penaltyPercent).toBe(67)
  })

  it('2+ indices → pénalité par défaut 0.33', () => {
    const result = calcQcmPenaltyForHints(qcmQuestion, 10, 2)
    expect(result.multiplier).toBe(0.33)
    expect(result.effectivePoints).toBe(3) // round(10 * 0.33)
  })

  it('3 indices se comporte comme 2+ (plafond)', () => {
    const two = calcQcmPenaltyForHints(qcmQuestion, 10, 2)
    const three = calcQcmPenaltyForHints(qcmQuestion, 10, 3)
    expect(three).toEqual(two)
  })

  it('respecte les pénalités configurées sur la question (QCM_PENALTY_1/2)', () => {
    const custom = { ...qcmQuestion, QCM_PENALTY_1: 0.5, QCM_PENALTY_2: 0.1 }
    expect(calcQcmPenaltyForHints(custom, 10, 1).multiplier).toBe(0.5)
    expect(calcQcmPenaltyForHints(custom, 10, 2).multiplier).toBe(0.1)
  })

  it('le montant effectif ne descend jamais sous 1 (jamais 0)', () => {
    const result = calcQcmPenaltyForHints(qcmQuestion, 1, 2) // 1 * 0.33 → round = 0
    expect(result.effectivePoints).toBe(1)
  })
})

// ========================================
// calcQcmPenalty — pénalité d'affichage courante (indices invalidés)
// ========================================

describe('calcQcmPenalty', () => {
  const qcmQuestion = { TYPE: 'QCM', QCM_HINTS_ENABLED: true }

  it('retourne null si la question n\'est pas un QCM avec indices', () => {
    expect(calcQcmPenalty({ TYPE: 'QCM', QCM_HINTS_ENABLED: false }, 10, 1)).toBeNull()
    expect(calcQcmPenalty({ TYPE: 'SPEEDY' }, 10, 1)).toBeNull()
  })

  it('retourne null si aucun indice n\'est invalidé (0)', () => {
    expect(calcQcmPenalty(qcmQuestion, 10, 0)).toBeNull()
  })

  it('1 indice invalidé → pénalité 0.67 par défaut', () => {
    const result = calcQcmPenalty(qcmQuestion, 10, 1)
    expect(result.invalidatedCount).toBe(1)
    expect(result.multiplier).toBe(0.67)
    expect(result.effectivePoints).toBe(7)
  })

  it('2+ indices invalidés → pénalité 0.33 par défaut', () => {
    const result = calcQcmPenalty(qcmQuestion, 10, 2)
    expect(result.multiplier).toBe(0.33)
    expect(result.effectivePoints).toBe(3)
  })
})

// ========================================
// calcMemoryScore — MEMORY complet / incomplet / avec erreurs
// ========================================

describe('calcMemoryScore', () => {
  const memoryQuestion = {
    TYPE: 'MEMORY',
    MEMORY_PAIRS: [1, 2, 3, 4], // totalPairs = 4
    MEMORY_CONFIG: { POINTS_PER_PAIR: 10, ERROR_PENALTY: 2, COMPLETION_BONUS: 5 },
  }

  it('retourne null si la question n\'est pas MEMORY', () => {
    expect(calcMemoryScore({ TYPE: 'QCM' }, 2, 0)).toBeNull()
  })

  it('incomplet, sans erreur : paires × points/paire', () => {
    const result = calcMemoryScore(memoryQuestion, 2, 0)
    expect(result.score).toBe(20)
    expect(result.isComplete).toBe(false)
  })

  it('complet : ajoute le bonus de complétion', () => {
    const result = calcMemoryScore(memoryQuestion, 4, 0)
    expect(result.isComplete).toBe(true)
    expect(result.score).toBe(4 * 10 + 5) // 45
  })

  it('avec erreurs : soustrait la pénalité par erreur', () => {
    const result = calcMemoryScore(memoryQuestion, 3, 2)
    expect(result.score).toBe(3 * 10 - 2 * 2) // 26
  })

  it('complet avec erreurs : bonus ET pénalité s\'appliquent', () => {
    const result = calcMemoryScore(memoryQuestion, 4, 3)
    expect(result.score).toBe(4 * 10 + 5 - 3 * 2) // 39
  })

  it('le score ne descend jamais sous 0', () => {
    const result = calcMemoryScore(memoryQuestion, 1, 100)
    expect(result.score).toBe(0)
  })

  it('utilise les valeurs par défaut si MEMORY_CONFIG est absent', () => {
    const question = { TYPE: 'MEMORY', MEMORY_PAIRS: [1, 2] }
    const result = calcMemoryScore(question, 2, 0)
    expect(result.pointsPerPair).toBe(10)
    expect(result.errorPenalty).toBe(0)
    expect(result.completionBonus).toBe(0)
    expect(result.score).toBe(20)
  })

  it('0 paire totale (MEMORY_PAIRS vide) n\'est jamais "complet"', () => {
    const question = { TYPE: 'MEMORY', MEMORY_PAIRS: [], MEMORY_CONFIG: { COMPLETION_BONUS: 5 } }
    const result = calcMemoryScore(question, 0, 0)
    expect(result.isComplete).toBe(false)
    expect(result.score).toBe(0)
  })
})

// ========================================
// calcArdoiseDefaultPoints — défaut ARDOISE
// ========================================

describe('calcArdoiseDefaultPoints', () => {
  it('utilise question.POINTS quand défini', () => {
    expect(calcArdoiseDefaultPoints({ POINTS: '5' }, 1)).toBe(5)
  })

  it('replie sur le montant de base si POINTS est absent', () => {
    expect(calcArdoiseDefaultPoints({}, 3)).toBe(3)
  })

  it('replie sur le montant de base si POINTS est invalide (NaN)', () => {
    expect(calcArdoiseDefaultPoints({ POINTS: 'abc' }, 3)).toBe(3)
  })

  it('replie sur le montant de base si POINTS vaut 0 (falsy)', () => {
    expect(calcArdoiseDefaultPoints({ POINTS: '0' }, 3)).toBe(3)
  })
})

// ========================================
// resolvePointsTarget — discriminant POINTS_TARGET
// ========================================

describe('resolvePointsTarget', () => {
  it('retourne TEAM quand POINTS_TARGET vaut TEAM', () => {
    expect(resolvePointsTarget({ POINTS_TARGET: 'TEAM' })).toBe('TEAM')
  })

  it('retourne PLAYER par défaut (absent, PLAYER, ou toute autre valeur)', () => {
    expect(resolvePointsTarget({})).toBe('PLAYER')
    expect(resolvePointsTarget({ POINTS_TARGET: 'PLAYER' })).toBe('PLAYER')
    expect(resolvePointsTarget(null)).toBe('PLAYER')
  })
})

// ========================================
// resolvePointsAward — cas SPEEDY (équipe/joueur) consommé par #156/F6
// ========================================

describe('resolvePointsAward — SPEEDY (question par défaut, sans QCM ni MEMORY)', () => {
  it('cible le joueur avec le montant de base quand POINTS_TARGET est absent', () => {
    const result = resolvePointsAward({ TYPE: 'SPEEDY' }, 7, {})
    expect(result).toEqual({ amount: 7, target: 'PLAYER' })
  })

  it('cible l\'équipe avec le montant de base quand POINTS_TARGET vaut TEAM', () => {
    const result = resolvePointsAward({ TYPE: 'SPEEDY', POINTS_TARGET: 'TEAM' }, 7, {})
    expect(result).toEqual({ amount: 7, target: 'TEAM' })
  })

  it('question absente (undefined) : montant de base, cible joueur', () => {
    const result = resolvePointsAward(undefined, 4, {})
    expect(result).toEqual({ amount: 4, target: 'PLAYER' })
  })
})

describe('resolvePointsAward — QCM', () => {
  const qcmQuestion = { TYPE: 'QCM', QCM_HINTS_ENABLED: true }

  it('applique la pénalité par joueur (hintsAtBuzz) au montant, cible joueur', () => {
    const result = resolvePointsAward(qcmQuestion, 10, { hintsAtBuzz: 1 })
    expect(result).toEqual({ amount: 7, target: 'PLAYER' })
  })

  it('QCM + POINTS_TARGET TEAM : pénalité appliquée, créditée à l\'équipe', () => {
    const question = { ...qcmQuestion, POINTS_TARGET: 'TEAM' }
    const result = resolvePointsAward(question, 10, { hintsAtBuzz: 2 })
    expect(result).toEqual({ amount: 3, target: 'TEAM' })
  })

  it('QCM sans indices activés : montant de base tel quel', () => {
    const result = resolvePointsAward({ TYPE: 'QCM', QCM_HINTS_ENABLED: false }, 10, { hintsAtBuzz: 2 })
    expect(result).toEqual({ amount: 10, target: 'PLAYER' })
  })
})

describe('resolvePointsAward — MEMORY', () => {
  const memoryQuestion = {
    TYPE: 'MEMORY',
    MEMORY_PAIRS: [1, 2, 3, 4],
    MEMORY_CONFIG: { POINTS_PER_PAIR: 10, ERROR_PENALTY: 2, COMPLETION_BONUS: 5 },
  }

  it('utilise le score MEMORY calculé, cible joueur par défaut', () => {
    const result = resolvePointsAward(memoryQuestion, 999, { memory: { matchedPairs: 2, errors: 0 } })
    expect(result).toEqual({ amount: 20, target: 'PLAYER' })
  })

  it('MEMORY + POINTS_TARGET TEAM : score MEMORY crédité à l\'équipe', () => {
    const question = { ...memoryQuestion, POINTS_TARGET: 'TEAM' }
    const result = resolvePointsAward(question, 999, { memory: { matchedPairs: 4, errors: 1 } })
    expect(result).toEqual({ amount: 4 * 10 + 5 - 2, target: 'TEAM' }) // 43
  })

  it('MEMORY sans contexte memory fourni : replie sur le montant de base', () => {
    const result = resolvePointsAward(memoryQuestion, 8, {})
    expect(result).toEqual({ amount: 8, target: 'PLAYER' })
  })
})
