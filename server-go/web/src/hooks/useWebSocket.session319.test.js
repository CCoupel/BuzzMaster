import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import useWebSocket from './useWebSocket'

// #319 (v13.0.0, LOT-2.2) — hook : réception du jeton de session (contracts/vplayer-session.md §4.3, §5). Le jeton de
// PLAYER_CONNECTED est stocké sous `vplayer_token` avec la règle d'onglet de l'ID (#246 F-ID), n'est jamais écrit dans
// la console (§3), et PLAYER_REJECTED {INVALID_SESSION} est relayé tel quel à la page (motif de redirection).

let ws = null
class MockWebSocket {
  constructor() { ws = this; this.readyState = 0; this.sent = [] }
  send(m) { this.sent.push(JSON.parse(m)) }
  close() { this.readyState = 3 }
}
MockWebSocket.OPEN = 1

const TOKEN = 'dG9rZW4tZGUtdGVzdC0zMi1vY3RldHMtYmFzZTY0dXJs'
const recv = (ACTION, MSG) => act(async () => { ws.onmessage({ data: JSON.stringify({ ACTION, MSG }) }) })

function storage() {
  let s = {}
  return { getItem: k => (k in s ? s[k] : null), setItem: (k, v) => { s[k] = String(v) }, removeItem: k => { delete s[k] }, clear: () => { s = {} } }
}

describe('useWebSocket — jeton de session (#319)', () => {
  let ls
  let log
  beforeEach(() => {
    ws = null
    ls = storage()
    vi.stubGlobal('WebSocket', MockWebSocket)
    vi.stubGlobal('localStorage', ls)
    log = vi.spyOn(console, 'log').mockImplementation(() => {})
  })
  afterEach(() => { vi.unstubAllGlobals(); vi.restoreAllMocks() })
  const open = () => { const h = renderHook(() => useWebSocket('/ws/player')); ws.readyState = 1; return h }

  it('PLAYER_CONNECTED {ID, TOKEN} : l\'ID et le jeton sont stockés (premier enrôlement)', async () => {
    const { result } = open()
    await recv('PLAYER_CONNECTED', { ID: 'b1', NAME: 'Alice', TEAM: '', TOKEN })
    expect(ls.getItem('vplayer_id')).toBe('b1')
    expect(ls.getItem('vplayer_token')).toBe(TOKEN)
    expect(result.current.playerConnectStatus).toMatchObject({ status: 'connected', id: 'b1', name: 'Alice' })
  })

  it('le jeton n\'est jamais exposé dans l\'état React renvoyé par le hook', async () => {
    const { result } = open()
    await recv('PLAYER_CONNECTED', { ID: 'b1', NAME: 'Alice', TOKEN })
    expect(JSON.stringify(result.current.playerConnectStatus)).not.toContain(TOKEN)
    expect(JSON.stringify(result.current.gameState)).not.toContain(TOKEN)
  })

  it('chaque PLAYER_CONNECTED apporte le jeton : un client qui l\'avait perdu le retrouve (§4.3)', async () => {
    open()
    ls.setItem('vplayer_id', 'b1') // jeton effacé, ID resté
    await recv('PLAYER_CONNECTED', { ID: 'b1', NAME: 'Alice', TOKEN })
    expect(ls.getItem('vplayer_token')).toBe(TOKEN)
  })

  it('règle d\'onglet (#246 F-ID) : un jeton n\'écrase pas celui d\'un autre onglet dont l\'ID stocké est différent', async () => {
    open()
    ls.setItem('vplayer_id', 'b-autre-onglet')
    ls.setItem('vplayer_token', 'jeton-autre-onglet')
    await recv('PLAYER_CONNECTED', { ID: 'b1', NAME: 'Alice', TOKEN })
    expect(ls.getItem('vplayer_token')).toBe('jeton-autre-onglet')
  })

  it('PLAYER_CONNECTED sans TOKEN (serveur sans jeton) : le jeton stocké est conservé, aucune valeur « undefined » écrite', async () => {
    open()
    ls.setItem('vplayer_id', 'b1')
    ls.setItem('vplayer_token', TOKEN)
    await recv('PLAYER_CONNECTED', { ID: 'b1', NAME: 'Alice', TEAM: '' })
    expect(ls.getItem('vplayer_token')).toBe(TOKEN)
    await recv('PLAYER_CONNECTED', { ID: 'b2', NAME: 'Bob' })
    expect(ls.getItem('vplayer_token')).not.toBe('undefined')
  })

  it('Critical — le jeton n\'est jamais écrit dans la console (réception ni envoi)', async () => {
    const { result } = open()
    await recv('PLAYER_CONNECTED', { ID: 'b1', NAME: 'Alice', TOKEN })
    act(() => { result.current.sendMessage('PLAYER_CONNECT', { NAME: 'Alice', ID: 'b1', TOKEN }) })
    const logged = [...log.mock.calls, ...console.error.mock?.calls ?? []].map(c => c.map(x => (typeof x === 'string' ? x : JSON.stringify(x))).join(' '))
    expect(logged.filter(l => l.includes(TOKEN))).toEqual([])
    expect(ws.sent.find(m => m.ACTION === 'PLAYER_CONNECT').MSG.TOKEN).toBe(TOKEN) // l'envoi lui-même est intact
  })

  it('PLAYER_REJECTED {INVALID_SESSION} est relayé tel quel à la page', async () => {
    const { result } = open()
    await recv('PLAYER_REJECTED', { REASON: 'INVALID_SESSION' })
    expect(result.current.playerConnectStatus).toEqual({ status: 'rejected', reason: 'INVALID_SESSION' })
    expect(ls.getItem('vplayer_token')).toBeNull() // le hook ne purge rien : c'est la page qui décide (règle d'onglet)
  })
})
