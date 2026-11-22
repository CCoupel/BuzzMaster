import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, join } from 'node:path'
import useWebSocket from './useWebSocket'

// #316/#317 (v12.2.0) — raccord Go → hook (leçon #312) : le HOOK RÉEL, nourri d'un GameState sérialisé par le vrai moteur Go
// (fixture roue-gamestate.312.json, régénérée par internal/game/roue_gamestate_fixture_312_test.go), expose ROUE_BONUS_MODE
// et SEGMENTS[].BONUS (contracts/roue.md §17.6). L'écran ne voit que ce que le hook recopie.
const here = dirname(fileURLToPath(import.meta.url))
const fixture = JSON.parse(readFileSync(join(here, 'fixtures', 'roue-gamestate.312.json'), 'utf8'))

let wsInstance = null
class MockWebSocket {
  constructor(url) { wsInstance = this; this.url = url; this.readyState = MockWebSocket.CONNECTING }
  send() {}
  close() { this.readyState = MockWebSocket.CLOSED; this.onclose && this.onclose() }
}
MockWebSocket.CONNECTING = 0; MockWebSocket.OPEN = 1; MockWebSocket.CLOSING = 2; MockWebSocket.CLOSED = 3

const push = async (msg) => { await act(async () => { wsInstance.onmessage({ data: JSON.stringify(msg) }) }) }

beforeEach(() => { wsInstance = null; vi.stubGlobal('WebSocket', MockWebSocket) })
afterEach(() => { vi.unstubAllGlobals(); vi.clearAllMocks() })

describe('useWebSocket — BONUS sur case à points (hook réel, fixture du moteur Go)', () => {
  it('la fixture porte ROUE_BONUS_MODE et au moins une case marquée (BONUS: true sur une case value)', () => {
    expect(Object.keys(fixture.GAME)).toContain('ROUE_BONUS_MODE')
    expect(['EPHEMERAL', 'PERMANENT']).toContain(fixture.GAME.ROUE_BONUS_MODE)
    const segs = fixture.GAME.ROUE_WHEEL.SEGMENTS
    expect(segs.length).toBeGreaterThan(0)
    segs.forEach((s) => expect(typeof s.BONUS, s.ID).toBe('boolean'))
    const marked = segs.filter((s) => s.BONUS)
    expect(marked.length).toBeGreaterThan(0)
    marked.forEach((s) => expect(s.KIND).toBe('value'))
    expect(segs.some((s) => s.KIND === 'bonus')).toBe(false)
  })

  it('état initial : ROUE_BONUS_MODE existe, jamais null / undefined', () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))
    expect(result.current.gameState.ROUE_BONUS_MODE).not.toBeUndefined()
    expect(result.current.gameState.ROUE_BONUS_MODE).not.toBeNull()
  })

  it('critical — un UPDATE de la fixture expose ROUE_BONUS_MODE et SEGMENTS[].BONUS tels que le moteur les a émis', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))
    await push({ ACTION: 'UPDATE', MSG: fixture })
    const g = result.current.gameState
    expect(g.ROUE_BONUS_MODE).toBe(fixture.GAME.ROUE_BONUS_MODE)
    expect(g.ROUE_WHEEL.SEGMENTS.map((s) => [s.ID, s.KIND, s.VALUE, s.BONUS]))
      .toEqual(fixture.GAME.ROUE_WHEEL.SEGMENTS.map((s) => [s.ID, s.KIND, s.VALUE, s.BONUS]))
    expect(g.ROUE_WHEEL.SEGMENTS.some((s) => s.BONUS === true)).toBe(true)
  })

  it('ROUE_BONUS_MODE est conservé après un UPDATE partiel ; PERMANENT puis "" (hors manche) sont repris tels quels', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))
    await push({ ACTION: 'UPDATE', MSG: fixture })
    await push({ ACTION: 'UPDATE', MSG: { GAME: { PHASE: 'STARTED', CURRENT_TIME: 3 } } })
    expect(result.current.gameState.ROUE_BONUS_MODE).toBe(fixture.GAME.ROUE_BONUS_MODE)
    await push({ ACTION: 'UPDATE', MSG: { GAME: { PHASE: 'STARTED', ROUE_BONUS_MODE: 'PERMANENT' } } })
    expect(result.current.gameState.ROUE_BONUS_MODE).toBe('PERMANENT')
    await push({ ACTION: 'UPDATE', MSG: { GAME: { PHASE: 'STOPPED', ROUE_BONUS_MODE: '' } } })
    expect(result.current.gameState.ROUE_BONUS_MODE).toBe('')
  })

  it('retrait éphémère : un UPDATE republiant SEGMENTS avec BONUS = false est repris (jamais figé sur true)', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))
    await push({ ACTION: 'UPDATE', MSG: fixture })
    const cleared = fixture.GAME.ROUE_WHEEL.SEGMENTS.map((s) => ({ ...s, BONUS: false }))
    await push({ ACTION: 'UPDATE', MSG: { GAME: { PHASE: 'STARTED', ROUE_WHEEL: { ...fixture.GAME.ROUE_WHEEL, SEGMENTS: cleared } } } })
    expect(result.current.gameState.ROUE_WHEEL.SEGMENTS.some((s) => s.BONUS)).toBe(false)
    expect(result.current.gameState.ROUE_WHEEL.SEGMENTS.length).toBe(cleared.length)
  })
})
