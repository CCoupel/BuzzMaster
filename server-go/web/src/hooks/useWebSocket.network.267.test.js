import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import useWebSocket from './useWebSocket'

// ---------------------------------------------------------------------------
// #267 (v11.2.2) — état réseau du serveur côté hook : GameState.NETWORK
//   { ONLY_LOCALHOST, IPS[], IGNORED_INTERFACES[{NAME, IP}], CHECKED_AT }
// Contrat : contracts/game-state.md §`NETWORK` (toujours sérialisé, listes jamais null,
// CHECKED_AT en ms epoch) ; NETWORK_ONLY_LOCALHOST = miroir hérité (#96, v5.6.2).
//
// Le bandeau #96 n'a JAMAIS été câblé côté client (le hook ne copiait aucun champ réseau) :
// ces tests ancrent le mapping NETWORK, son défaut, son repli sur un serveur ancien
// (NETWORK_ONLY_LOCALHOST seul) et sa conservation sur les UPDATE partiels.
// RED attendu jusqu'à l'implémentation du mapping dans useWebSocket.js.
// ---------------------------------------------------------------------------

let wsInstances = []
class MockWebSocket {
  constructor(url) { this.url = url; this.readyState = 0; this.sentMessages = []; wsInstances.push(this) }
  send(d) { this.sentMessages.push(d) }
  close() { if (this.readyState === 3) return; this.readyState = 3; this.onclose && this.onclose() }
  simulateOpen() { this.readyState = 1; this.onopen && this.onopen() }
  simulateMessage(d) { this.onmessage && this.onmessage({ data: JSON.stringify(d) }) }
}
MockWebSocket.CONNECTING = 0; MockWebSocket.OPEN = 1; MockWebSocket.CLOSING = 2; MockWebSocket.CLOSED = 3

const DEFAULT_NETWORK = { ONLY_LOCALHOST: false, IPS: [], IGNORED_INTERFACES: [], CHECKED_AT: 0 }

const LOST = {
  ONLY_LOCALHOST: true,
  IPS: [],
  IGNORED_INTERFACES: [{ NAME: 'vEthernet (WSL)', IP: '172.21.0.1' }],
  CHECKED_AT: 1760436000000,
}
const OK = {
  ONLY_LOCALHOST: false,
  IPS: ['192.168.1.84'],
  IGNORED_INTERFACES: [],
  CHECKED_AT: 1760436030000,
}

describe('useWebSocket — état réseau du serveur (#267)', () => {
  beforeEach(() => { wsInstances = []; vi.useFakeTimers(); vi.stubGlobal('WebSocket', MockWebSocket) })
  afterEach(() => { vi.useRealTimers(); vi.unstubAllGlobals() })

  function open(endpoint = '/ws/admin') {
    const hook = renderHook(() => useWebSocket(endpoint))
    const ws = wsInstances[0]
    act(() => { ws.simulateOpen() })
    return { hook, ws }
  }
  const update = (ws, game) => act(() => {
    ws.simulateMessage({ ACTION: 'UPDATE', MSG: { GAME: { PHASE: 'STOPPED', ...game }, teams: {}, bumpers: {} } })
  })

  it('défaut avant tout message : NETWORK complet, réseau présumé présent, jamais undefined', () => {
    const { hook } = open()
    expect(hook.result.current.gameState.NETWORK).toEqual(DEFAULT_NETWORK)
  })

  it('Critical — NETWORK reçu est exposé tel quel dans gameState (4 champs, listes comprises)', () => {
    const { hook, ws } = open()
    update(ws, { NETWORK: LOST, NETWORK_ONLY_LOCALHOST: true })
    expect(hook.result.current.gameState.NETWORK).toEqual(LOST)
    expect(hook.result.current.gameState.NETWORK.ONLY_LOCALHOST).toBe(true)
    expect(hook.result.current.gameState.NETWORK.IGNORED_INTERFACES).toEqual([{ NAME: 'vEthernet (WSL)', IP: '172.21.0.1' }])
  })

  it('Critical — transitions : perte puis retour du réseau, état suivi à chaque UPDATE', () => {
    const { hook, ws } = open()
    update(ws, { NETWORK: LOST, NETWORK_ONLY_LOCALHOST: true })
    expect(hook.result.current.gameState.NETWORK.ONLY_LOCALHOST).toBe(true)
    update(ws, { NETWORK: OK, NETWORK_ONLY_LOCALHOST: false })
    expect(hook.result.current.gameState.NETWORK).toEqual(OK)
    update(ws, { NETWORK: LOST, NETWORK_ONLY_LOCALHOST: true })
    expect(hook.result.current.gameState.NETWORK).toEqual(LOST)
  })

  it('un admin qui se connecte pendant la panne reçoit l\'état complet dès le premier UPDATE (HELLO)', () => {
    const { hook, ws } = open()
    expect(hook.result.current.gameState.NETWORK.ONLY_LOCALHOST).toBe(false)
    update(ws, { NETWORK: LOST, NETWORK_ONLY_LOCALHOST: true })
    expect(hook.result.current.gameState.NETWORK.ONLY_LOCALHOST).toBe(true)
    expect(hook.result.current.gameState.NETWORK.CHECKED_AT).toBe(LOST.CHECKED_AT)
  })

  it('un UPDATE partiel sans NETWORK conserve le dernier état réseau connu', () => {
    const { hook, ws } = open()
    update(ws, { NETWORK: LOST, NETWORK_ONLY_LOCALHOST: true })
    update(ws, { CURRENT_TIME: 12 }) // UPDATE de jeu sans champ réseau
    expect(hook.result.current.gameState.NETWORK).toEqual(LOST)
  })

  it('repli serveur ancien : NETWORK_ONLY_LOCALHOST seul (true) → NETWORK.ONLY_LOCALHOST vrai, listes vides, jamais undefined', () => {
    const { hook, ws } = open()
    update(ws, { NETWORK_ONLY_LOCALHOST: true }) // pas de NETWORK : serveur < 11.2.2
    const n = hook.result.current.gameState.NETWORK
    expect(n.ONLY_LOCALHOST).toBe(true)
    expect(n.IPS).toEqual([])
    expect(n.IGNORED_INTERFACES).toEqual([])
    expect(n.CHECKED_AT).toEqual(expect.any(Number))
  })

  it('repli serveur ancien : NETWORK_ONLY_LOCALHOST seul (false) → réseau présent', () => {
    const { hook, ws } = open()
    update(ws, { NETWORK_ONLY_LOCALHOST: false })
    expect(hook.result.current.gameState.NETWORK.ONLY_LOCALHOST).toBe(false)
  })

  it('NETWORK prime sur le miroir hérité quand les deux sont présents', () => {
    const { hook, ws } = open()
    // Cas incohérent volontaire : le contrat dit « miroir exact », mais NETWORK reste la source.
    update(ws, { NETWORK: OK, NETWORK_ONLY_LOCALHOST: true })
    expect(hook.result.current.gameState.NETWORK).toEqual(OK)
  })

  it('les listes vides du serveur (jamais null) sont conservées vides, pas remplacées', () => {
    const { hook, ws } = open()
    update(ws, { NETWORK: { ONLY_LOCALHOST: false, IPS: [], IGNORED_INTERFACES: [], CHECKED_AT: 42 } })
    const n = hook.result.current.gameState.NETWORK
    expect(n.IPS).toEqual([])
    expect(n.IGNORED_INTERFACES).toEqual([])
    expect(n.CHECKED_AT).toBe(42)
  })

  it.each(['/ws/tv', '/ws/player', '/ws/anim'])('%s : le champ est mappé sans erreur (diffusé à tous, seul l\'admin l\'affiche)', (endpoint) => {
    const { hook, ws } = open(endpoint)
    update(ws, { NETWORK: LOST, NETWORK_ONLY_LOCALHOST: true })
    expect(hook.result.current.gameState.NETWORK).toEqual(LOST)
  })
})
