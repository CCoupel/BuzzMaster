import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import useWebSocket from './useWebSocket'

// #319 — PLAYER_CONNECTED.TOKEN conservé avec l'ID (règle d'onglet), jamais écrit dans la console.
let ws = null
class MockWebSocket {
  constructor() { ws = this; this.readyState = 0; this.sent = [] }
  send(m) { this.sent.push(JSON.parse(m)) }
  close() { this.readyState = 3 }
}
MockWebSocket.OPEN = 1
const recv = (ACTION, MSG) => act(async () => { ws.onmessage({ data: JSON.stringify({ ACTION, MSG }) }) })

describe('useWebSocket — jeton de session VJoueur (#319)', () => {
  beforeEach(() => { ws = null; localStorage.clear(); vi.stubGlobal('WebSocket', MockWebSocket) })
  afterEach(() => { vi.unstubAllGlobals(); vi.restoreAllMocks() })

  it('premier enrôlement : ID et jeton sont conservés', async () => {
    renderHook(() => useWebSocket('/ws/player'))
    await recv('PLAYER_CONNECTED', { ID: 'b1', NAME: 'Alice', TOKEN: 'tok1' })
    expect(localStorage.getItem('vplayer_id')).toBe('b1')
    expect(localStorage.getItem('vplayer_token')).toBe('tok1')
  })
  it('reconnexion de la même place : le jeton est mis à jour', async () => {
    localStorage.setItem('vplayer_id', 'b1'); localStorage.setItem('vplayer_token', 'old')
    renderHook(() => useWebSocket('/ws/player'))
    await recv('PLAYER_CONNECTED', { ID: 'b1', NAME: 'Alice', TOKEN: 'new' })
    expect(localStorage.getItem('vplayer_token')).toBe('new')
  })
  it('règle d\'onglet : l\'ID stocké d\'une autre session empêche l\'écriture du jeton', async () => {
    localStorage.setItem('vplayer_id', 'b9'); localStorage.setItem('vplayer_token', 'tok9')
    renderHook(() => useWebSocket('/ws/player'))
    await recv('PLAYER_CONNECTED', { ID: 'b1', NAME: 'Alice', TOKEN: 'tok1' })
    expect(localStorage.getItem('vplayer_token')).toBe('tok9')
  })
  it('PLAYER_CONNECTED sans jeton (serveur antérieur) : rien n\'est écrit', async () => {
    renderHook(() => useWebSocket('/ws/player'))
    await recv('PLAYER_CONNECTED', { ID: 'b1', NAME: 'Alice' })
    expect(localStorage.getItem('vplayer_token')).toBeNull()
  })
  it('le jeton n\'apparaît jamais dans la console (réception ni envoi)', async () => {
    const log = vi.spyOn(console, 'log').mockImplementation(() => {})
    const { result } = renderHook(() => useWebSocket('/ws/player'))
    ws.readyState = 1
    await recv('PLAYER_CONNECTED', { ID: 'b1', NAME: 'Alice', TOKEN: 'SECRET-TOKEN' })
    act(() => { result.current.sendMessage('PLAYER_CONNECT', { NAME: 'Alice', ID: 'b1', TOKEN: 'SECRET-TOKEN' }) })
    expect(JSON.stringify(log.mock.calls)).not.toContain('SECRET-TOKEN')
    expect(ws.sent.some(m => m.MSG?.TOKEN === 'SECRET-TOKEN')).toBe(true) // le message part bien avec le jeton
  })
})
