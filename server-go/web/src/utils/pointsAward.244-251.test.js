import { describe, it, expect } from 'vitest'
import * as pa from './pointsAward'

// #244/#251 — le montant par défaut d'une équipe dans l'attribution groupée est EXACTEMENT celui du clic unitaire sur
// la carte d'équipe : une seule fonction partagée (`defaultTeamAward`, utils/pointsAward.js). Contrat :
// websocket-actions.md §TEAM_POINTS_BATCH « Montants par défaut ».

describe('defaultTeamAward (#244/#251) — source unique du montant par défaut', () => {
  it('est exportée', () => { expect(typeof pa.defaultTeamAward).toBe('function') })

  it('SPEEDY, ARDOISE, QCM sans indices, MEMOTION : le montant de base B', () => {
    for (const type of ['SPEEDY', 'ARDOISE', 'MEMOTION', 'QCM']) {
      expect(pa.defaultTeamAward({ question: { TYPE: type, QCM_HINTS_ENABLED: false }, teamName: 'A', basePoints: 4, teamBumpers: [] }), type).toBe(4)
    }
  })

  it('QCM avec indices actifs : la pénalité d’indices de l’équipe (calcQcmTeamAward), jamais plus que B', () => {
    const question = { TYPE: 'QCM', QCM_HINTS_ENABLED: true, POINTS: '3', QCM_ANSWERS: { RED: 'a', GREEN: 'b', YELLOW: 'c', BLUE: 'd' }, QCM_CORRECT: 'GREEN' }
    const bumpers = [{ HINTS_AT_BUZZ: 2, ANSWER_COLOR: 'GREEN' }]
    const expected = pa.calcQcmTeamAward(question, 3, bumpers, 0).amount
    expect(pa.defaultTeamAward({ question, teamName: 'A', basePoints: 3, teamBumpers: bumpers, invalidatedCount: 0 })).toBe(expected)
    expect(expected).toBeLessThanOrEqual(3)
  })

  it('MEMORY multi-équipes : le score des paires de l’équipe (calcMemoryScore)', () => {
    const question = { TYPE: 'MEMORY', MEMORY_MODE: 'MULTI', POINTS: '10', MEMORY_PAIRS: [{}, {}, {}, {}], MEMORY_CONFIG: {} }
    const expected = pa.calcMemoryScore(question, 3, 1)
    const got = pa.defaultTeamAward({ question, teamName: 'A', basePoints: 10, teamBumpers: [], memoryTeamPairs: { A: 3 }, memoryTeamErrors: { A: 1 } })
    expect(got).toBe(expected ? expected.score : 10)
  })

  it('RAFALE : le compteur de l’équipe (calcRafaleTeamAward)', () => {
    const question = { TYPE: 'RAFALE', POINTS: '2' }
    const counter = pa.rafaleCounterForTeam(question, 'A', { A: 5 }, { A: 7 })
    const award = pa.calcRafaleTeamAward(question, 2, counter)
    expect(pa.defaultTeamAward({ question, teamName: 'A', basePoints: 2, teamBumpers: [], rafaleCounters: { A: 5 }, rafaleBest: { A: 7 } })).toBe(award ? award.amount : 2)
  })
})
