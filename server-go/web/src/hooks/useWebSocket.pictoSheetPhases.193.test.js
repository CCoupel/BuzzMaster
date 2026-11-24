import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import useWebSocket from './useWebSocket'

// #193 (v13.0.0, LOT-2.1) — la feuille est vierge hors manche (contrat picto.md §8.4, §6.5, §6.6) : un UPDATE en
// PREPARE, READY, COUNTDOWN ou NEW_GAME vide la feuille locale, même si l'époque n'a pas encore changé (elle ne change
// qu'au START) ; STARTED, PAUSED, STOPPED et REVEALED la CONSERVENT (« la feuille reste affichée jusqu'au changement
// de question », §6.5). Fichier à part de useWebSocket.pictoClearSheet.192.test.js (dev-frontend).

let ws = null
class MockWebSocket {
  constructor() { ws = this; this.readyState = 0; this.sent = [] }
  send(m) { this.sent.push(JSON.parse(m)) }
  close() { this.readyState = 3 }
}
MockWebSocket.OPEN = 1

const recv = (ACTION, MSG) => act(async () => { ws.onmessage({ data: JSON.stringify({ ACTION, MSG }) }) })
const update = (PHASE, epoch) => recv('UPDATE', { GAME: { PHASE, PICTO_CANVAS_EPOCH: epoch } })
const frame = (epoch, seq, id, pts) => ({ EPOCH: epoch, SEQ: seq, STROKES: [{ ID: id, PTS: pts, END: true }] })

describe('useWebSocket — feuille PICTO selon la phase (#193)', () => {
  beforeEach(() => { ws = null; vi.stubGlobal('WebSocket', MockWebSocket); vi.spyOn(console, 'log').mockImplementation(() => {}) })
  afterEach(() => { vi.unstubAllGlobals(); vi.restoreAllMocks() })
  const open = () => { const h = renderHook(() => useWebSocket('/ws/tv')); ws.readyState = 1; return h }
  const drawn = async (epoch = 2) => {
    const h = open()
    await update('STARTED', epoch)
    await recv('PICTO_STROKES', frame(epoch, 1, 1, [[1, 1], [9, 9]]))
    expect(h.result.current.pictoCanvasStore.getStrokes()).toHaveLength(1)
    return h
  }

  it.each(['PREPARE', 'READY', 'COUNTDOWN', 'NEW_GAME'])('un UPDATE en %s vide la feuille (époque inchangée)', async (phase) => {
    const { result } = await drawn()
    await update(phase, 2)
    expect(result.current.pictoCanvasStore.getStrokes()).toHaveLength(0)
    expect(result.current.pictoCanvasStore.getSeq()).toBe(0)
  })

  it.each(['STARTED', 'PAUSED', 'STOPPED', 'REVEALED'])('un UPDATE en %s conserve la feuille (§6.5)', async (phase) => {
    const { result } = await drawn()
    await update(phase, 2)
    expect(result.current.pictoCanvasStore.getStrokes()).toHaveLength(1)
  })

  it('COUNTDOWN d\'une relance : feuille vide, puis le START (nouvelle époque) repart à SEQ 1 sans demande de resynchronisation', async () => {
    const { result } = await drawn(2)
    await update('PREPARE', 2)
    await update('READY', 2)
    await update('COUNTDOWN', 2)
    expect(result.current.pictoCanvasStore.getStrokes()).toHaveLength(0)
    await update('STARTED', 3) // nouvelle époque au START (§6.2)
    await recv('PICTO_STROKES', frame(3, 1, 1, [[5, 5]]))
    expect(result.current.pictoCanvasStore.getStrokes()).toHaveLength(1)
    expect(ws.sent.filter(m => m.ACTION === 'PICTO_CANVAS_SYNC')).toHaveLength(0)
  })

  it('NEW_GAME : feuille vide, et un trait en vol de l\'ancienne époque ne réapparaît pas après le START suivant', async () => {
    const { result } = await drawn(2)
    await update('NEW_GAME', 2)
    expect(result.current.pictoCanvasStore.getStrokes()).toHaveLength(0)
    await update('STARTED', 3)
    await recv('PICTO_STROKES', frame(2, 2, 7, [[9, 9]])) // retardataire de l'époque 2
    expect(result.current.pictoCanvasStore.getStrokes()).toHaveLength(0)
  })

  it('une feuille déjà vide reste vide et ne déclenche aucune demande de resynchronisation', async () => {
    const { result } = open()
    await update('PREPARE', 0)
    await update('COUNTDOWN', 0)
    expect(result.current.pictoCanvasStore.getStrokes()).toHaveLength(0)
    expect(ws.sent.filter(m => m.ACTION === 'PICTO_CANVAS_SYNC')).toHaveLength(0)
  })
})
