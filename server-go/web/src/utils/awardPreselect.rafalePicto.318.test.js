import { describe, it, expect } from 'vitest'
import { preselectAwardKeys } from './awardPreselect'
import { resolveAwardLevel } from './pointsAward'

// #318 (v13.1.0, LOT-3, T30) — présélection du crédit de fin de manche Rafale Picto (award-preselect.md §2 « RAFALE_PICTO »,
// picto.md §17.7) : l'équipe qui dessine est cochée si PICTO_ROUND_POINTS > 0 ; plancher 0 : une manche à 0 point ne
// coche rien ; le crédit se fait au niveau ÉQUIPE.

const call = (over = {}) => preselectAwardKeys({
  question: { TYPE: 'RAFALE_PICTO' }, gameState: { PICTO_TEAM: 'Bleus', PICTO_ROUND_POINTS: 25 }, bumpers: {},
  awardLevel: 'TEAM', awarded: [], amountOf: () => 25, ...over,
})

describe('preselectAwardKeys — RAFALE_PICTO (#318)', () => {
  it('l’équipe qui dessine est cochée si PICTO_ROUND_POINTS > 0', () => {
    expect(call()).toEqual(['Bleus'])
  })
  it.each([0, -3, undefined, null])('rien n’est coché si PICTO_ROUND_POINTS = %s (plancher 0, manche sans point)', (p) => {
    expect(call({ gameState: { PICTO_TEAM: 'Bleus', PICTO_ROUND_POINTS: p } })).toEqual([])
  })
  it('rien si aucune équipe n’est choisie', () => {
    expect(call({ gameState: { PICTO_TEAM: '', PICTO_ROUND_POINTS: 25 } })).toEqual([])
  })
  it('jamais une équipe déjà créditée', () => {
    expect(call({ awarded: ['Bleus'] })).toEqual([])
  })
  it('jamais un montant par défaut ≤ 0', () => {
    expect(call({ amountOf: () => 0 })).toEqual([])
  })
  it('la présélection ne dépend pas de PICTO_OUTCOME (TIMEOUT, STOPPED, POOL_END, FOUND)', () => {
    for (const o of ['TIMEOUT', 'STOPPED', 'POOL_END']) {
      expect(call({ gameState: { PICTO_TEAM: 'Bleus', PICTO_ROUND_POINTS: 10, PICTO_OUTCOME: o } })).toEqual(['Bleus'])
    }
  })
  it('contraste avec PICTO (un mot) : coché seulement si PICTO_OUTCOME = FOUND', () => {
    const base = { question: { TYPE: 'PICTO' }, gameState: { PICTO_TEAM: 'Bleus', PICTO_ROUND_POINTS: 25 } }
    expect(call({ ...base, gameState: { ...base.gameState, PICTO_OUTCOME: 'TIMEOUT' } })).toEqual([])
    expect(call({ ...base, gameState: { ...base.gameState, PICTO_OUTCOME: 'FOUND' } })).toEqual(['Bleus'])
  })
})

describe('resolveAwardLevel — famille Picto (#318)', () => {
  it('RAFALE_PICTO : toujours TEAM, le réglage POINTS_TARGET d’un ancien fichier est ignoré', () => {
    expect(resolveAwardLevel({ TYPE: 'RAFALE_PICTO' })).toBe('TEAM')
    expect(resolveAwardLevel({ TYPE: 'RAFALE_PICTO', POINTS_TARGET: 'PLAYER' })).toBe('TEAM')
  })
  it('PICTO inchangé : TEAM', () => {
    expect(resolveAwardLevel({ TYPE: 'PICTO', POINTS_TARGET: 'PLAYER' })).toBe('TEAM')
  })
})
