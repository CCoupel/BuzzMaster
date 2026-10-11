import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import useWebSocket from './useWebSocket'

// #253 — LAUNCH_LOCK (état + action), LAUNCH_LOCK_REFUSED, ACTION_REFUSED, identifiant de connexion.

let ws
class MockWebSocket {
  constructor() { ws = this; this.readyState = 0; this.sent = [] }
  send(m) { this.sent.push(JSON.parse(m)) }
  close() { this.readyState = 3 }
}
MockWebSocket.OPEN = 1
const msg = (ACTION, MSG) => act(() => { ws.onmessage({ data: JSON.stringify({ ACTION, MSG }) }) })

describe('useWebSocket — verrou « Lancer la partie » (#253)', () => {
  beforeEach(() => vi.stubGlobal('WebSocket', MockWebSocket))
  afterEach(() => vi.unstubAllGlobals())

  it('LAUNCH_LOCK : valeur par défaut, suit l\'UPDATE, conservé si absent', () => {
    const { result } = renderHook(() => useWebSocket('/ws/admin'))
    expect(result.current.gameState.LAUNCH_LOCK).toEqual({ ACTIVE: false, OWNER: '', SINCE: 0, EXPIRES_AT: 0 })
    msg('UPDATE', { GAME: { PHASE: 'REVEALED', LAUNCH_LOCK: { ACTIVE: true, OWNER: 'c1', SINCE: 1, EXPIRES_AT: 46000 } } })
    expect(result.current.gameState.LAUNCH_LOCK).toEqual({ ACTIVE: true, OWNER: 'c1', SINCE: 1, EXPIRES_AT: 46000 })
    msg('UPDATE', { GAME: { PHASE: 'REVEALED' } })
    expect(result.current.gameState.LAUNCH_LOCK.ACTIVE).toBe(true)
  })

  it('launchLock(state) envoie LAUNCH_LOCK {STATE} et retourne le booléen d\'envoi', () => {
    const { result } = renderHook(() => useWebSocket('/ws/admin'))
    let ok
    act(() => { ok = result.current.launchLock('OPEN') })
    expect(ok).toBe(false)
    ws.readyState = 1
    act(() => { ok = result.current.launchLock('OPEN'); result.current.launchLock('CLOSE') })
    expect(ok).toBe(true)
    expect(ws.sent.filter(m => m.ACTION === 'LAUNCH_LOCK').map(m => m.MSG)).toEqual([{ STATE: 'OPEN' }, { STATE: 'CLOSE' }])
  })

  it('LAUNCH_LOCK_REFUSED et ACTION_REFUSED exposés puis consommés', () => {
    const { result } = renderHook(() => useWebSocket('/ws/admin'))
    msg('LAUNCH_LOCK_REFUSED', { REASON: 'LOCKED_BY_OTHER' })
    expect(result.current.launchLockRefused).toEqual({ reason: 'LOCKED_BY_OTHER' })
    act(() => result.current.clearLaunchLockRefused())
    expect(result.current.launchLockRefused).toBeNull()
    msg('ACTION_REFUSED', { ACTION: 'START', REASON: 'LAUNCH_IN_PROGRESS' })
    expect(result.current.actionRefused).toMatchObject({ action: 'START', reason: 'LAUNCH_IN_PROGRESS' })
    act(() => result.current.clearActionRefused())
    expect(result.current.actionRefused).toBeNull()
  })

  it('identifiant de connexion retenu dès qu\'un message porte CLIENT_ID', () => {
    const { result } = renderHook(() => useWebSocket('/ws/admin'))
    expect(result.current.connectionId).toBeNull()
    msg('HELLO', { CLIENT_ID: 'conn-42' })
    expect(result.current.connectionId).toBe('conn-42')
  })
})
