import { describe, it, expect } from 'vitest'
import { defaultTeamAward } from './pointsAward'

// #318 (v13.1.0, LOT-3) — crédit Rafale Picto : montant pré-rempli = PICTO_ROUND_POINTS (calculé par le serveur).
const q = { TYPE: 'RAFALE_PICTO', POINTS: 10 }
describe('defaultTeamAward — RAFALE_PICTO', () => {
  it('montant = PICTO_ROUND_POINTS, pas le barème par mot', () => {
    expect(defaultTeamAward({ question: q, teamName: 'A', basePoints: 10, teamBumpers: [], pictoRoundPoints: 25 })).toBe(25)
  })
  it('PICTO_ROUND_POINTS = 0 : 0 (jamais sous 0, plancher serveur)', () => {
    expect(defaultTeamAward({ question: q, teamName: 'A', basePoints: 10, teamBumpers: [], pictoRoundPoints: 0 })).toBe(0)
  })
  it('sans PICTO_ROUND_POINTS (serveur ancien) : repli sur le barème', () => {
    expect(defaultTeamAward({ question: q, teamName: 'A', basePoints: 10, teamBumpers: [] })).toBe(10)
  })
  it('le type Picto (un mot) reste au barème de la question', () => {
    expect(defaultTeamAward({ question: { TYPE: 'PICTO', POINTS: 10 }, teamName: 'A', basePoints: 10, teamBumpers: [], pictoRoundPoints: 25 })).toBe(10)
  })
  it('pénalité (base négative) : uniforme, inchangé', () => {
    expect(defaultTeamAward({ question: q, teamName: 'A', basePoints: -3, teamBumpers: [], pictoRoundPoints: 25 })).toBe(-3)
  })
})
