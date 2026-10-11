import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import useWebSocket from './useWebSocket'

// #253 (v11.2) — verrou « Lancer la partie » côté hook : LAUNCH_LOCK {STATE: OPEN|CLOSE} (renouvellement = OPEN),
// refus ciblés LAUNCH_LOCK_REFUSED / ACTION_REFUSED exposés puis consommables. Contrat : websocket-actions.md §LAUNCH_LOCK.
let wsInstances = []
class MockWebSocket {
  constructor(url) { this.url = url; this.readyState = 0; this.sentMessages = []; wsInstances.push(this) }
  send(d) { this.sentMessages.push(d) }
  close() { if (this.readyState === 3) return; this.readyState = 3; this.onclose && this.onclose() }
  simulateOpen() { this.readyState = 1; this.onopen && this.onopen() }
  simulateMessage(d) { this.onmessage && this.onmessage({ data: JSON.stringify(d) }) }
}
MockWebSocket.CONNECTING = 0; MockWebSocket.OPEN = 1; MockWebSocket.CLOSING = 2; MockWebSocket.CLOSED = 3
const sentOf = (ws, action) => ws.sentMessages.map((m) => JSON.parse(m)).filter((m) => m.ACTION === action).map((m) => m.MSG)

describe('useWebSocket — verrou de lancement (#253)', () => {
  beforeEach(() => { wsInstances = []; vi.useFakeTimers(); vi.stubGlobal('WebSocket', MockWebSocket) })
  afterEach(() => { vi.useRealTimers(); vi.unstubAllGlobals() })
  function open(endpoint = '/ws/admin') {
    const hook = renderHook(() => useWebSocket(endpoint))
    const ws = wsInstances[0]
    act(() => { ws.simulateOpen() })
    return { hook, ws }
  }

  it('Critical — launchLock("OPEN") puis ("CLOSE") envoient LAUNCH_LOCK {STATE} ; le renouvellement est un nouvel OPEN', () => {
    const { hook, ws } = open()
    act(() => { hook.result.current.launchLock('OPEN') })
    act(() => { hook.result.current.launchLock('OPEN') })
    act(() => { hook.result.current.launchLock('CLOSE') })
    expect(sentOf(ws, 'LAUNCH_LOCK')).toEqual([{ STATE: 'OPEN' }, { STATE: 'OPEN' }, { STATE: 'CLOSE' }])
  })

  it('launchLock retourne false quand la WebSocket est fermée (rien n’est « envoyé »)', () => {
    const { hook, ws } = open()
    act(() => { ws.close() })
    let result
    act(() => { result = hook.result.current.launchLock('OPEN') })
    expect(result).toBe(false)
  })

  it('LAUNCH_LOCK_REFUSED expose le motif (LOCKED_BY_OTHER / ENTRACTE_ACTIVE) et peut être consommé', () => {
    const { hook, ws } = open()
    act(() => { ws.simulateMessage({ ACTION: 'LAUNCH_LOCK_REFUSED', MSG: { REASON: 'LOCKED_BY_OTHER' } }) })
    expect(hook.result.current.launchLockRefused).toEqual({ reason: 'LOCKED_BY_OTHER' })
    act(() => { hook.result.current.clearLaunchLockRefused() })
    expect(hook.result.current.launchLockRefused).toBeNull()
    act(() => { ws.simulateMessage({ ACTION: 'LAUNCH_LOCK_REFUSED', MSG: { REASON: 'ENTRACTE_ACTIVE' } }) })
    expect(hook.result.current.launchLockRefused).toEqual({ reason: 'ENTRACTE_ACTIVE' })
  })

  it('ACTION_REFUSED expose l’action et le motif LAUNCH_IN_PROGRESS, consommable (admin et anim)', () => {
    for (const endpoint of ['/ws/admin', '/ws/anim']) {
      wsInstances = []
      const { hook, ws } = open(endpoint)
      act(() => { ws.simulateMessage({ ACTION: 'ACTION_REFUSED', MSG: { ACTION: 'TEAM_POINTS', REASON: 'LAUNCH_IN_PROGRESS' } }) })
      expect(hook.result.current.actionRefused).toMatchObject({ action: 'TEAM_POINTS', reason: 'LAUNCH_IN_PROGRESS' })
      act(() => { hook.result.current.clearActionRefused() })
      expect(hook.result.current.actionRefused).toBeNull()
    }
  })

  it('l’état partagé LAUNCH_LOCK reçu est exposé dans gameState (jamais absent)', () => {
    const { hook, ws } = open()
    act(() => {
      ws.simulateMessage({ ACTION: 'UPDATE', MSG: { GAME: { PHASE: 'STOPPED', LAUNCH_LOCK: { ACTIVE: true, OWNER: 'c-1', SINCE: 10, EXPIRES_AT: 45010 } }, teams: {}, bumpers: {} } })
    })
    expect(hook.result.current.gameState.LAUNCH_LOCK).toEqual({ ACTIVE: true, OWNER: 'c-1', SINCE: 10, EXPIRES_AT: 45010 })
  })

  it('Critical — l’identifiant de connexion reçu du serveur (HELLO.CLIENT_ID) est exposé : il sert à se reconnaître propriétaire', () => {
    const { hook, ws } = open()
    expect(hook.result.current.connectionId).toBeNull()
    act(() => { ws.simulateMessage({ ACTION: 'HELLO', MSG: { CLIENT_ID: 'conn-7' } }) })
    expect(hook.result.current.connectionId).toBe('conn-7')
  })
})
