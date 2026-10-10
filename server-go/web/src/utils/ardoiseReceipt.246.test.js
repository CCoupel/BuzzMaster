import { describe, it, expect } from 'vitest'
import * as mod from './ardoiseReceipt'

// #246 (v11.2) — règle UNIQUE « équipes attendues / hors ligne / reçues » (admin et anim, miroir du serveur).
// Contrat : contracts/websocket-actions.md §REVEAL « Garde ARDOISE ».
//  - équipe attendue = clé de `teams` ayant ≥ 1 bumper IS_VPLAYER avec TEAM = cette clé ;
//  - hors ligne = TOUS ses VJoueurs CONNECTED=false — information d'affichage, NE LÈVE PAS la garde ;
//  - reçue = ARDOISE_ANSWERS[équipe].RECEIVED === true.
const receipt = mod.getArdoiseReceipt

const vp = (team, connected = true) => ({ IS_VPLAYER: true, IS_VIRTUAL: true, TEAM: team, CONNECTED: connected })
const TEAMS = { A: {}, B: {}, C: {}, D: {} }

describe('ardoiseReceipt (#246)', () => {
  it('exporte getArdoiseReceipt', () => { expect(typeof receipt).toBe('function') })

  it('une équipe sans VJoueur n’est jamais attendue (buzzer physique seul, ou aucun bumper)', () => {
    const bumpers = { 1: vp('A'), 2: { IS_VPLAYER: false, TEAM: 'B', CONNECTED: true } }
    const r = receipt(TEAMS, bumpers, {})
    expect(r.expected).toEqual(['A'])
    expect(r.total).toBe(1)
  })

  it('reçue seulement si RECEIVED === true ; une entrée absente, RECEIVED false ou truthy non booléen = en attente', () => {
    const bumpers = { 1: vp('A'), 2: vp('B'), 3: vp('C') }
    const r = receipt(TEAMS, bumpers, { A: { TEXT: 'x', RECEIVED: true }, B: { TEXT: 'y', RECEIVED: false }, C: { TEXT: 'z', RECEIVED: 'true' } })
    expect(r.byTeam).toEqual({ A: 'ok', B: 'wait', C: 'wait' })
    expect(r.receivedCount).toBe(1)
    expect(r.complete).toBe(false)
  })

  it('hors ligne = tous les VJoueurs de l’équipe déconnectés ; un seul connecté suffit à ne pas l’être', () => {
    const bumpers = { 1: vp('A', false), 2: vp('A', false), 3: vp('B', false), 4: vp('B', true) }
    const r = receipt(TEAMS, bumpers, {})
    expect(r.byTeam).toEqual({ A: 'off', B: 'wait' })
    expect(r.pending).toEqual([{ team: 'A', offline: true }, { team: 'B', offline: false }])
  })

  it('Critical — hors ligne ne lève PAS la garde : une équipe hors ligne non reçue reste en attente (complete = false)', () => {
    const r = receipt({ A: {}, B: {} }, { 1: vp('A'), 2: vp('B', false) }, { A: { RECEIVED: true } })
    expect(r.complete).toBe(false)
    expect(r.pending).toEqual([{ team: 'B', offline: true }])
  })

  it('une équipe hors ligne mais reçue compte comme reçue (sa tablette a répondu avant de se couper)', () => {
    const r = receipt({ A: {} }, { 1: vp('A', false) }, { A: { RECEIVED: true } })
    expect(r.byTeam).toEqual({ A: 'ok' })
    expect(r.complete).toBe(true)
  })

  it('tout reçu : complete, compteur n = N ; aucune équipe attendue : complete (rien à attendre)', () => {
    const r = receipt({ A: {}, B: {} }, { 1: vp('A'), 2: vp('B') }, { A: { RECEIVED: true }, B: { RECEIVED: true } })
    expect(r).toMatchObject({ total: 2, receivedCount: 2, complete: true, pending: [] })
    expect(receipt({ A: {} }, {}, {})).toMatchObject({ total: 0, complete: true })
  })

  it('robuste aux entrées absentes (teams/bumpers/answers undefined ou null)', () => {
    expect(() => receipt(undefined, undefined, undefined)).not.toThrow()
    expect(receipt(null, null, null).total).toBe(0)
  })
})
