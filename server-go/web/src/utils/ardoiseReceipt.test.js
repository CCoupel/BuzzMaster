import { describe, it, expect } from 'vitest'
import { getArdoiseReceipt, getExpectedArdoiseTeams, isArdoiseTeamOffline } from './ardoiseReceipt'

const vp = (team, connected = true) => ({ TEAM: team, IS_VPLAYER: true, CONNECTED: connected })
const teams = { A: {}, B: {}, C: {}, D: {} }
const bumpers = { a1: vp('A'), a2: vp('A', false), b1: vp('B', false), c1: vp('C'), x: { TEAM: 'D', IS_VPLAYER: false } }

describe('ardoiseReceipt (#246)', () => {
  it('équipes attendues = clés de teams avec ≥ 1 bumper IS_VPLAYER', () => {
    expect(getExpectedArdoiseTeams(teams, bumpers)).toEqual(['A', 'B', 'C'])
  })
  it('hors ligne = tous les VJoueurs CONNECTED=false (un seul connecté suffit à ne pas l\'être)', () => {
    expect(isArdoiseTeamOffline(bumpers, 'A')).toBe(false)
    expect(isArdoiseTeamOffline(bumpers, 'B')).toBe(true)
  })
  it('reçues / en attente / hors ligne ; équipe sans entrée = non reçue ; hors ligne ne lève pas la garde', () => {
    const r = getArdoiseReceipt(teams, bumpers, { A: { RECEIVED: true }, B: { RECEIVED: false } })
    expect(r.receivedCount).toBe(1)
    expect(r.total).toBe(3)
    expect(r.complete).toBe(false)
    expect(r.byTeam).toEqual({ A: 'ok', B: 'off', C: 'wait' })
    expect(r.pending).toEqual([{ team: 'B', offline: true }, { team: 'C', offline: false }])
  })
  it('complet quand toutes les équipes attendues sont reçues', () => {
    const r = getArdoiseReceipt(teams, bumpers, { A: { RECEIVED: true }, B: { RECEIVED: true }, C: { RECEIVED: true } })
    expect(r.complete).toBe(true)
  })
})
