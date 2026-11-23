import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import useWebSocket from './useWebSocket'

// #192/#193 (v13.0.0, LOT-2.1) — raccord comportemental du hook : état initial, UPDATE, PICTO_WORD, PICTO_ERROR,
// PICTO_STROKES (hors console.log), PICTO_CANVAS, trou de SEQ → PICTO_CANVAS_SYNC (contrat picto.md §5, §8, §9).

let ws = null
let sent = []
class MockWebSocket {
  constructor(url) { ws = this; this.url = url; this.readyState = 0 }
  send(d) { sent.push(JSON.parse(d)) }
  close() { this.readyState = 3; this.onclose && this.onclose() }
}
MockWebSocket.CONNECTING = 0; MockWebSocket.OPEN = 1; MockWebSocket.CLOSING = 2; MockWebSocket.CLOSED = 3

const deliver = async (action, msg) => { await act(async () => { ws.onmessage({ data: JSON.stringify({ ACTION: action, MSG: msg }) }) }) }
const update = (game) => deliver('UPDATE', { GAME: { PHASE: 'STARTED', ...game } })
const syncs = () => sent.filter(m => m.ACTION === 'PICTO_CANVAS_SYNC')
const frame = (epoch, seq, id, pts) => ({ EPOCH: epoch, SEQ: seq, STROKES: [{ ID: id, PTS: pts, END: true }] })

describe('useWebSocket — PICTO (#192/#193)', () => {
  let logSpy
  beforeEach(() => {
    ws = null; sent = []
    vi.stubGlobal('WebSocket', MockWebSocket)
    logSpy = vi.spyOn(console, 'log').mockImplementation(() => {})
  })
  afterEach(() => { vi.unstubAllGlobals(); vi.restoreAllMocks() })

  const mount = async () => {
    const r = renderHook(() => useWebSocket('/ws/test'))
    await act(async () => { ws.readyState = 1; ws.onopen && ws.onopen() })
    sent = []
    return r
  }

  it('état initial : valeurs neutres, jamais undefined (tableaux vides, booléen faux)', async () => {
    const { result } = await mount()
    const g = result.current.gameState
    expect(g.PICTO_TEAM).toBe('')
    expect(g.PICTO_DRAWER).toBe('')
    expect(g.PICTO_DRAWER_NAME).toBe('')
    expect(g.PICTO_DRAWER_CONNECTED).toBe(false)
    expect(g.PICTO_CANDIDATES).toEqual([])
    expect(g.PICTO_TEAM_COLOR).toEqual([])
    expect(g.PICTO_WORDS_COUNT).toBe(0)
    expect(g.PICTO_CANVAS_EPOCH).toBe(0)
    expect(g.PICTO_OUTCOME).toBe('')
    expect(g.PICTO_REVEALED_WORD).toBe('')
    expect(g.PICTO_REVEALED_CATEGORY).toBe('')
    expect(result.current.pictoWord).toBeNull()
    expect(result.current.pictoError).toBeNull()
  })

  it('UPDATE : les 11 champs PICTO_* sont recopiés, puis conservés par un UPDATE qui n\'en parle pas', async () => {
    const { result } = await mount()
    const cands = [{ ID: 'p-1', NAME: 'Alice', CONNECTED: true }, { ID: 'p-2', NAME: 'Bob', CONNECTED: false }]
    await update({
      PICTO_TEAM: 'Rouges', PICTO_TEAM_COLOR: [255, 0, 0], PICTO_DRAWER: 'p-1', PICTO_DRAWER_NAME: 'Alice',
      PICTO_DRAWER_CONNECTED: true, PICTO_CANDIDATES: cands, PICTO_WORDS_COUNT: 3, PICTO_CANVAS_EPOCH: 2,
      PICTO_OUTCOME: 'FOUND', PICTO_REVEALED_WORD: 'Zèbre', PICTO_REVEALED_CATEGORY: 'ANIMALS',
    })
    let g = result.current.gameState
    expect(g).toMatchObject({
      PICTO_TEAM: 'Rouges', PICTO_TEAM_COLOR: [255, 0, 0], PICTO_DRAWER: 'p-1', PICTO_DRAWER_NAME: 'Alice',
      PICTO_DRAWER_CONNECTED: true, PICTO_WORDS_COUNT: 3, PICTO_CANVAS_EPOCH: 2, PICTO_OUTCOME: 'FOUND',
      PICTO_REVEALED_WORD: 'Zèbre', PICTO_REVEALED_CATEGORY: 'ANIMALS',
    })
    expect(g.PICTO_CANDIDATES).toEqual(cands)
    await update({ PHASE: 'PAUSED' })
    g = result.current.gameState
    expect(g.PICTO_TEAM).toBe('Rouges')
    expect(g.PICTO_CANDIDATES).toEqual(cands)
    expect(g.PICTO_CANVAS_EPOCH).toBe(2)
  })

  it('UPDATE : une valeur vide du serveur remplace la précédente (remise à vide à la relance)', async () => {
    const { result } = await mount()
    await update({ PICTO_DRAWER: 'p-1', PICTO_REVEALED_WORD: 'Zèbre', PICTO_OUTCOME: 'FOUND', PICTO_CANDIDATES: [{ ID: 'p-1', NAME: 'A', CONNECTED: true }] })
    await update({ PICTO_DRAWER: '', PICTO_REVEALED_WORD: '', PICTO_OUTCOME: '', PICTO_CANDIDATES: [] })
    const g = result.current.gameState
    expect([g.PICTO_DRAWER, g.PICTO_REVEALED_WORD, g.PICTO_OUTCOME]).toEqual(['', '', ''])
    expect(g.PICTO_CANDIDATES).toEqual([])
  })

  it('PICTO_WORD : le mot, la catégorie et l\'époque sont exposés (pictoWord), pas dans gameState', async () => {
    const { result } = await mount()
    await deliver('PICTO_WORD', { WORD: 'Zèbre', CATEGORY: 'ANIMALS', EPOCH: 4 })
    expect(result.current.pictoWord).toEqual({ WORD: 'Zèbre', CATEGORY: 'ANIMALS', EPOCH: 4 })
    expect(JSON.stringify(result.current.gameState)).not.toContain('Zèbre')
  })

  it('PICTO_ERROR : code et message exposés, SEQ croissant pour distinguer deux refus identiques, effaçable', async () => {
    const { result } = await mount()
    await deliver('PICTO_ERROR', { CODE: 'NO_DRAWER', MESSAGE: 'Désignez un dessinateur' })
    expect(result.current.pictoError).toMatchObject({ CODE: 'NO_DRAWER', MESSAGE: 'Désignez un dessinateur', SEQ: 1 })
    await deliver('PICTO_ERROR', { CODE: 'NO_DRAWER', MESSAGE: 'Désignez un dessinateur' })
    expect(result.current.pictoError.SEQ).toBe(2)
    act(() => result.current.clearPictoError())
    expect(result.current.pictoError).toBeNull()
  })

  it('PICTO_STROKES est exclu du console.log systématique, en réception comme en envoi ; les autres actions restent journalisées', async () => {
    const { result } = await mount()
    await update({ PICTO_CANVAS_EPOCH: 1 })
    logSpy.mockClear()
    await deliver('PICTO_STROKES', frame(1, 1, 1, [[10, 10]]))
    act(() => { result.current.sendMessage('PICTO_STROKES', { EPOCH: 1, STROKES: [] }) })
    const logged = logSpy.mock.calls.map(c => c.join(' '))
    expect(logged.filter(l => l.includes('PICTO_STROKES'))).toEqual([])
    expect(sent.some(m => m.ACTION === 'PICTO_STROKES')).toBe(true) // l'envoi a bien lieu
    await deliver('PICTO_ERROR', { CODE: 'WRONG_PHASE', MESSAGE: 'x' })
    expect(logSpy.mock.calls.some(c => c.join(' ').includes('PICTO_ERROR'))).toBe(true)
  })

  it('PICTO_STROKES de la bonne époque, SEQ 1, 2 : appliqué au store, sans PICTO_CANVAS_SYNC', async () => {
    const { result } = await mount()
    await update({ PICTO_CANVAS_EPOCH: 3 })
    await deliver('PICTO_STROKES', frame(3, 1, 1, [[10, 10], [20, 20]]))
    await deliver('PICTO_STROKES', frame(3, 2, 2, [[30, 30]]))
    const store = result.current.pictoCanvasStore
    expect(store.getEpoch()).toBe(3)
    expect(store.getSeq()).toBe(2)
    expect(store.getStrokes().map(s => s.ID)).toEqual([1, 2])
    expect(syncs()).toHaveLength(0)
  })

  it('trou de SEQ : le hook envoie PICTO_CANVAS_SYNC {} (une seule fois), puis PICTO_CANVAS rétablit la feuille', async () => {
    const { result } = await mount()
    await update({ PICTO_CANVAS_EPOCH: 3 })
    await deliver('PICTO_STROKES', frame(3, 1, 1, [[10, 10]]))
    await deliver('PICTO_STROKES', frame(3, 4, 4, [[40, 40]])) // 2 et 3 perdues
    expect(syncs()).toHaveLength(1)
    expect(syncs()[0].MSG).toEqual({})
    await deliver('PICTO_STROKES', frame(3, 5, 5, [[50, 50]])) // pendant l'attente : pas de 2e demande
    expect(syncs()).toHaveLength(1)
    await deliver('PICTO_CANVAS', { EPOCH: 3, SEQ: 4, STROKES: [1, 2, 3, 4].map(i => ({ ID: i, PTS: [[i, i]], END: true })) })
    const store = result.current.pictoCanvasStore
    expect(store.isSyncing()).toBe(false)
    expect(store.getStrokes().map(s => s.ID)).toEqual([1, 2, 3, 4, 5]) // la trame 5 mise de côté est rejouée
  })

  it('trame d\'une autre époque : ignorée, aucune demande de resynchronisation', async () => {
    const { result } = await mount()
    await update({ PICTO_CANVAS_EPOCH: 3 })
    await deliver('PICTO_STROKES', frame(2, 9, 1, [[1, 1]]))
    await deliver('PICTO_STROKES', frame(4, 1, 1, [[1, 1]]))
    expect(result.current.pictoCanvasStore.getStrokes()).toEqual([])
    expect(syncs()).toHaveLength(0)
  })

  it('un changement de PICTO_CANVAS_EPOCH dans un UPDATE efface la feuille locale ; la même époque non', async () => {
    const { result } = await mount()
    await update({ PICTO_CANVAS_EPOCH: 3 })
    await deliver('PICTO_STROKES', frame(3, 1, 1, [[10, 10]]))
    await update({ PICTO_CANVAS_EPOCH: 3 })
    expect(result.current.pictoCanvasStore.getStrokes()).toHaveLength(1)
    await update({ PICTO_CANVAS_EPOCH: 4 })
    expect(result.current.pictoCanvasStore.getStrokes()).toEqual([])
    expect(result.current.pictoCanvasStore.getSeq()).toBe(0)
  })

  it('PICTO_CANVAS au rechargement (avant tout UPDATE) : la feuille est affichée et l\'époque adoptée', async () => {
    const { result } = await mount()
    await deliver('PICTO_CANVAS', { EPOCH: 6, SEQ: 2, STROKES: [{ ID: 1, PTS: [[1, 1], [2, 2]], END: true }] })
    const store = result.current.pictoCanvasStore
    expect(store.getEpoch()).toBe(6)
    expect(store.getStrokes()).toHaveLength(1)
    await update({ PICTO_CANVAS_EPOCH: 6 }) // l'UPDATE qui suit ne doit pas effacer la feuille reçue
    expect(store.getStrokes()).toHaveLength(1)
  })

  it('socket fermée : un trou de SEQ ne provoque pas d\'envoi ni d\'exception', async () => {
    const { result } = await mount()
    await update({ PICTO_CANVAS_EPOCH: 3 })
    await deliver('PICTO_STROKES', frame(3, 1, 1, [[10, 10]]))
    ws.readyState = 3
    await deliver('PICTO_STROKES', frame(3, 5, 5, [[50, 50]]))
    expect(syncs()).toHaveLength(0)
    expect(result.current.pictoCanvasStore.isSyncing()).toBe(true)
  })
})
