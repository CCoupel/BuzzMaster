import { describe, it, expect } from 'vitest'
import {
  calcQcmPenaltyForHints,
  calcQcmPenalty,
  calcQcmTeamAward,
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

// ========================================
// calcQcmTeamAward — #157 T1, montant QCM de niveau équipe (interface
// animateur, zone C) — mutualisé depuis GamePage.jsx:272-290
// (qcmTeamAcquiredPoints) + GamePage.jsx:1072-1078 (repli), plan
// _work/reports/plan-20260813-151543.md §1.2/T1.
//
// Trois branches à reproduire à l'identique (repli compris, sous peine de
// divergence de montant avec /admin — plan §6 R1) :
//   1. Un bumper de l'équipe a buzzé (TIME > 0) avec ANSWER_COLOR ===
//      QCM_CORRECT → pénalité PAR JOUEUR (calcQcmPenaltyForHints, son propre
//      HINTS_AT_BUZZ). S'il y en a plusieurs, garde le MEILLEUR montant
//      (GamePage.jsx:284-288 "Keep the best (highest) points").
//   2. Aucun bumper correct → repli sur la pénalité des indices COURANTS
//      (calcQcmPenalty, invalidatedCount — PAS le HINTS_AT_BUZZ d'un buzz).
//   3. Ni l'un ni l'autre applicable (indices désactivés, ou aucun indice
//      invalidé) → montant de base tel quel.
//
// hasCorrectAnswer distingue la branche 1 des branches 2/3 (consommé par T4
// pour le marqueur ✓/✗ en zone C) — vrai dès qu'un bumper correct est
// trouvé, MÊME si les indices sont désactivés (auquel cas le montant reste
// le montant de base malgré tout — piège identifié en lisant
// GamePage.jsx:279-280 : `pts = penalty ? penalty.effectivePoints :
// pointsInput`, `penalty` peut être null tout en ayant trouvé un bumper
// correct).
// ========================================

describe('calcQcmTeamAward', () => {
  const qcmQuestion = (overrides = {}) => ({
    TYPE: 'QCM',
    QCM_CORRECT: 'RED',
    QCM_HINTS_ENABLED: true,
    ...overrides,
  })

  describe('branche 1 — buzzer correct dans l\'équipe', () => {
    it('0 indice au buzz → montant plein, hasCorrectAnswer=true', () => {
      const teamBumpers = [{ TIME: 100, ANSWER_COLOR: 'RED', HINTS_AT_BUZZ: 0 }]
      const result = calcQcmTeamAward(qcmQuestion(), 10, teamBumpers, 0)
      expect(result).toEqual({ amount: 10, hasCorrectAnswer: true })
    })

    it('1 indice au buzz → pénalité 0.67, hasCorrectAnswer=true', () => {
      const teamBumpers = [{ TIME: 100, ANSWER_COLOR: 'RED', HINTS_AT_BUZZ: 1 }]
      const result = calcQcmTeamAward(qcmQuestion(), 10, teamBumpers, 0)
      expect(result).toEqual({ amount: 7, hasCorrectAnswer: true })
    })

    it('2+ indices au buzz → pénalité 0.33, hasCorrectAnswer=true', () => {
      const teamBumpers = [{ TIME: 100, ANSWER_COLOR: 'RED', HINTS_AT_BUZZ: 2 }]
      const result = calcQcmTeamAward(qcmQuestion(), 10, teamBumpers, 0)
      expect(result).toEqual({ amount: 3, hasCorrectAnswer: true })
    })

    it('plusieurs bumpers corrects dans l\'équipe → garde le meilleur montant (le moins d\'indices)', () => {
      const teamBumpers = [
        { TIME: 100, ANSWER_COLOR: 'RED', HINTS_AT_BUZZ: 2 }, // 3 pts
        { TIME: 150, ANSWER_COLOR: 'RED', HINTS_AT_BUZZ: 0 }, // 10 pts — le meilleur
      ]
      const result = calcQcmTeamAward(qcmQuestion(), 10, teamBumpers, 0)
      expect(result).toEqual({ amount: 10, hasCorrectAnswer: true })
    })

    it('un bumper avec la mauvaise couleur ne compte pas comme correct', () => {
      const teamBumpers = [{ TIME: 100, ANSWER_COLOR: 'BLUE', HINTS_AT_BUZZ: 0 }]
      const result = calcQcmTeamAward(qcmQuestion(), 10, teamBumpers, 0)
      expect(result.hasCorrectAnswer).toBe(false)
    })

    it('un bumper correct mais n\'ayant pas buzzé (TIME=0) ne compte pas', () => {
      const teamBumpers = [{ TIME: 0, ANSWER_COLOR: 'RED', HINTS_AT_BUZZ: 0 }]
      const result = calcQcmTeamAward(qcmQuestion(), 10, teamBumpers, 0)
      expect(result.hasCorrectAnswer).toBe(false)
    })
  })

  describe('branche 2 — aucun buzzer correct : repli sur les indices courants', () => {
    it('mauvaise couleur buzzée, 1 indice courant invalidé → pénalité 0.67 (PAS le HINTS_AT_BUZZ du buzz)', () => {
      const teamBumpers = [{ TIME: 100, ANSWER_COLOR: 'BLUE', HINTS_AT_BUZZ: 2 }] // HINTS_AT_BUZZ ignoré ici
      const result = calcQcmTeamAward(qcmQuestion(), 10, teamBumpers, 1)
      expect(result).toEqual({ amount: 7, hasCorrectAnswer: false })
    })

    it('2+ indices courants invalidés → pénalité 0.33', () => {
      const teamBumpers = [{ TIME: 100, ANSWER_COLOR: 'BLUE', HINTS_AT_BUZZ: 0 }]
      const result = calcQcmTeamAward(qcmQuestion(), 10, teamBumpers, 3)
      expect(result).toEqual({ amount: 3, hasCorrectAnswer: false })
    })
  })

  describe('branche — équipe sans buzz (aucun bumper, ou aucun avec TIME>0)', () => {
    it('teamBumpers vide → repli sur les indices courants comme une équipe ayant mal répondu', () => {
      const result = calcQcmTeamAward(qcmQuestion(), 10, [], 2)
      expect(result).toEqual({ amount: 3, hasCorrectAnswer: false })
    })

    it('teamBumpers vide et aucun indice invalidé → montant de base', () => {
      const result = calcQcmTeamAward(qcmQuestion(), 10, [], 0)
      expect(result).toEqual({ amount: 10, hasCorrectAnswer: false })
    })

    it('teamBumpers undefined/non fourni → ne plante pas, se comporte comme vide', () => {
      const result = calcQcmTeamAward(qcmQuestion(), 10, undefined, 0)
      expect(result).toEqual({ amount: 10, hasCorrectAnswer: false })
    })
  })

  describe('branche 3 — QCM sans indices activés (QCM_HINTS_ENABLED=false)', () => {
    it('avec un bumper correct → montant de base tel quel, MAIS hasCorrectAnswer reste true', () => {
      const question = qcmQuestion({ QCM_HINTS_ENABLED: false })
      const teamBumpers = [{ TIME: 100, ANSWER_COLOR: 'RED', HINTS_AT_BUZZ: 2 }]
      const result = calcQcmTeamAward(question, 10, teamBumpers, 3)
      expect(result).toEqual({ amount: 10, hasCorrectAnswer: true })
    })

    it('sans bumper correct → montant de base, hasCorrectAnswer=false', () => {
      const question = qcmQuestion({ QCM_HINTS_ENABLED: false })
      const result = calcQcmTeamAward(question, 10, [], 3)
      expect(result).toEqual({ amount: 10, hasCorrectAnswer: false })
    })
  })

  it('non-QCM (ex: SPEEDY) : repli neutre sur le montant de base', () => {
    const result = calcQcmTeamAward({ TYPE: 'SPEEDY' }, 10, [], 0)
    expect(result).toEqual({ amount: 10, hasCorrectAnswer: false })
  })

  it('respecte des seuils QCM_PENALTY_1/2 personnalisés, dans les deux branches', () => {
    const question = qcmQuestion({ QCM_PENALTY_1: 0.5, QCM_PENALTY_2: 0.1 })
    const correctBranch = calcQcmTeamAward(question, 10, [{ TIME: 100, ANSWER_COLOR: 'RED', HINTS_AT_BUZZ: 1 }], 0)
    expect(correctBranch.amount).toBe(5)
    const fallbackBranch = calcQcmTeamAward(question, 10, [], 1)
    expect(fallbackBranch.amount).toBe(5)
  })
})
