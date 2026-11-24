import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import useWebSocket from './useWebSocket'

// PICTO (#192/#193) — feuille vierge hors manche, trou de SEQ → PICTO_CANVAS_SYNC, PICTO_STROKES hors console.log.
let ws = null
class MockWebSocket {
  constructor() { ws = this; this.readyState = 0; this.sent = [] }
  send(m) { this.sent.push(JSON.parse(m)) }
  close() { this.readyState = 3 }
}
MockWebSocket.OPEN = 1

const recv = (ACTION, MSG) => act(async () => { ws.onmessage({ data: JSON.stringify({ ACTION, MSG }) }) })
const update = (PHASE, epoch) => recv('UPDATE', { GAME: { PHASE, PICTO_CANVAS_EPOCH: epoch } })

describe('useWebSocket — PICTO', () => {
  beforeEach(() => { ws = null; vi.stubGlobal('WebSocket', MockWebSocket) })
  afterEach(() => { vi.unstubAllGlobals(); vi.restoreAllMocks() })

  const open = () => { const h = renderHook(() => useWebSocket('/ws/tv')); ws.readyState = 1; return h }

  it('une feuille dessinée est effacée au retour en PREPARE / READY, avant même le nouveau START', async () => {
    const { result } = open()
    await update('STARTED', 2)
    await recv('PICTO_STROKES', { EPOCH: 2, SEQ: 1, STROKES: [{ ID: 1, PTS: [[1, 1], [9, 9]], END: true }] })
    expect(result.current.pictoCanvasStore.getStrokes()).toHaveLength(1)
    await update('PREPARE', 2) // relance : l'époque ne change qu'au START
    expect(result.current.pictoCanvasStore.getStrokes()).toHaveLength(0)
    await update('READY', 2)
    expect(result.current.pictoCanvasStore.getStrokes()).toHaveLength(0)
  })

  it('un trou de SEQ demande la feuille entière une seule fois', async () => {
    const { result } = open()
    await update('STARTED', 1)
    await recv('PICTO_STROKES', { EPOCH: 1, SEQ: 1, STROKES: [{ ID: 1, PTS: [[0, 0]], END: false }] })
    await recv('PICTO_STROKES', { EPOCH: 1, SEQ: 5, STROKES: [{ ID: 1, PTS: [[4, 4]], END: false }] })
    await recv('PICTO_STROKES', { EPOCH: 1, SEQ: 6, STROKES: [{ ID: 1, PTS: [[5, 5]], END: false }] })
    expect(ws.sent.filter(m => m.ACTION === 'PICTO_CANVAS_SYNC')).toHaveLength(1)
    await recv('PICTO_CANVAS', { EPOCH: 1, SEQ: 4, STROKES: [{ ID: 1, PTS: [[0, 0], [1, 1], [2, 2], [3, 3]], END: false }] })
    expect(result.current.pictoCanvasStore.getSeq()).toBe(6)
  })

  it('PICTO_WORD et PICTO_ERROR sont exposés ; PICTO_STROKES n\'est pas journalisé', async () => {
    const log = vi.spyOn(console, 'log').mockImplementation(() => {})
    const { result } = open()
    await recv('PICTO_WORD', { WORD: 'Maison', CATEGORY: 'X', EPOCH: 2 })
    await recv('PICTO_ERROR', { CODE: 'NO_DRAWER', MESSAGE: 'm' })
    expect(result.current.pictoWord).toEqual({ WORD: 'Maison', CATEGORY: 'X', EPOCH: 2 })
    expect(result.current.pictoError.CODE).toBe('NO_DRAWER')
    log.mockClear()
    await recv('PICTO_STROKES', { EPOCH: 9, SEQ: 1, STROKES: [] })
    expect(log.mock.calls.some(c => c[1] === 'PICTO_STROKES')).toBe(false)
  })
})
