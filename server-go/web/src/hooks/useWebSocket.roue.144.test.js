import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import useWebSocket from './useWebSocket'

// #144 (v12.0.0, LOT-1.2) — useWebSocket : champs ROUE_* (contracts/roue.md §4) et ROUE_ERROR (§8.3).
// Scaffold calqué sur useWebSocket.ardoise.test.js.

let wsInstance = null
class MockWebSocket {
  constructor(url) { wsInstance = this; this.url = url; this.readyState = MockWebSocket.CONNECTING }
  send() {}
  close() { this.readyState = MockWebSocket.CLOSED; this.onclose && this.onclose() }
}
MockWebSocket.CONNECTING = 0; MockWebSocket.OPEN = 1; MockWebSocket.CLOSING = 2; MockWebSocket.CLOSED = 3

const push = async (msg) => { await act(async () => { wsInstance.onmessage({ data: JSON.stringify(msg) }) }) }
const update = (game) => push({ ACTION: 'UPDATE', MSG: { GAME: { PHASE: 'STARTED', ...game } } })

beforeEach(() => { wsInstance = null; vi.stubGlobal('WebSocket', MockWebSocket) })
afterEach(() => { vi.unstubAllGlobals(); vi.clearAllMocks() })

const ROUE_KEYS = ['ROUE_SUBPHASE', 'ROUE_STEP', 'ROUE_TEAMS', 'ROUE_CURRENT_TEAM', 'ROUE_CURRENT_TEAM_COLOR', 'ROUE_NEXT_TEAM',
  'ROUE_NEXT_DEFAULT', 'ROUE_TEAM_BANK', 'ROUE_TURN_GAIN', 'ROUE_BOARD', 'ROUE_LETTERS', 'ROUE_SHOW_LETTERS', 'ROUE_CONSONANTS_LEFT',
  'ROUE_WHEEL', 'ROUE_LAST_EVENT', 'ROUE_WINNER', 'ROUE_AWARDS']

describe('useWebSocket — état initial ROUE', () => {
  it('tous les champs ROUE_* existent, jamais null / undefined', () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))
    for (const k of ROUE_KEYS) {
      expect(result.current.gameState[k], k).not.toBeUndefined()
      expect(result.current.gameState[k], k).not.toBeNull()
    }
  })

  it('valeurs initiales sûres pour le rendu : listes vides, plateau 14 colonnes, roue sans case, CURRENT = -1', () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))
    const g = result.current.gameState
    expect(g.ROUE_TEAMS).toEqual([]); expect(g.ROUE_LETTERS).toEqual([]); expect(g.ROUE_TEAM_BANK).toEqual({}); expect(g.ROUE_AWARDS).toEqual({})
    expect(g.ROUE_BOARD.COLS).toBe(14); expect(g.ROUE_BOARD.ROWS).toEqual([])
    expect(g.ROUE_WHEEL.SEGMENTS).toEqual([]); expect(g.ROUE_WHEEL.CURRENT).toBe(-1)
    expect(g.ROUE_SUBPHASE).toBe('')
  })
})

describe('useWebSocket — UPDATE ROUE', () => {
  it('reprend les champs ROUE_* d\'un UPDATE', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))
    await update({ ROUE_SUBPHASE: 'LETTER', ROUE_STEP: 4, ROUE_TEAMS: ['red', 'blue'], ROUE_CURRENT_TEAM: 'red',
      ROUE_TEAM_BANK: { red: 300, blue: 0 }, ROUE_BOARD: { COLS: 14, ROWS: ['__N'] }, ROUE_CONSONANTS_LEFT: false,
      ROUE_LETTERS: [{ LETTER: 'N', TEAM: 'red', HIT: true }], ROUE_WINNER: '', ROUE_AWARDS: {} })
    const g = result.current.gameState
    expect(g.ROUE_SUBPHASE).toBe('LETTER'); expect(g.ROUE_STEP).toBe(4)
    expect(g.ROUE_TEAM_BANK).toEqual({ red: 300, blue: 0 })
    expect(g.ROUE_BOARD.ROWS).toEqual(['__N'])
    expect(g.ROUE_CONSONANTS_LEFT).toBe(false)
    expect(g.ROUE_LETTERS).toHaveLength(1)
  })

  it('un UPDATE sans champ ROUE_* conserve les valeurs précédentes (ex. UPDATE_TIMER partiel)', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))
    await update({ ROUE_SUBPHASE: 'SPINNING', ROUE_CURRENT_TEAM: 'blue' })
    await update({ CURRENT_TIME: 3 })
    expect(result.current.gameState.ROUE_SUBPHASE).toBe('SPINNING')
    expect(result.current.gameState.ROUE_CURRENT_TEAM).toBe('blue')
  })

  it('une valeur fausse mais définie n\'est pas écrasée par le défaut (false, 0, "")', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))
    await update({ ROUE_CONSONANTS_LEFT: true, ROUE_SHOW_LETTERS: true, ROUE_TURN_GAIN: 50, ROUE_CURRENT_TEAM: 'red' })
    await update({ ROUE_CONSONANTS_LEFT: false, ROUE_SHOW_LETTERS: false, ROUE_TURN_GAIN: 0, ROUE_CURRENT_TEAM: '' })
    const g = result.current.gameState
    expect(g.ROUE_CONSONANTS_LEFT).toBe(false); expect(g.ROUE_SHOW_LETTERS).toBe(false)
    expect(g.ROUE_TURN_GAIN).toBe(0); expect(g.ROUE_CURRENT_TEAM).toBe('')
  })

  it('remise à vide en fin de manche : la roue revient à CURRENT = -1 et la sous-phase à ""', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))
    await update({ ROUE_SUBPHASE: 'SOLVED', ROUE_WHEEL: { LAYOUT: 'arc', SEGMENTS: [{ ID: 's0', KIND: 'value', VALUE: 1 }], CURRENT: 0, STOP: null } })
    await update({ ROUE_SUBPHASE: '', ROUE_WHEEL: { LAYOUT: 'strip', SEGMENTS: [], CURRENT: -1, STOP: null }, ROUE_BOARD: { COLS: 14, ROWS: [] } })
    expect(result.current.gameState.ROUE_SUBPHASE).toBe('')
    expect(result.current.gameState.ROUE_WHEEL.CURRENT).toBe(-1)
    expect(result.current.gameState.ROUE_BOARD.ROWS).toEqual([])
  })
})

describe('useWebSocket — ROUE_ERROR', () => {
  it('expose le refus (CODE, MESSAGE) et clearRoueError le retire', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))
    expect(result.current.roueError).toBeNull()
    await push({ ACTION: 'ROUE_ERROR', MSG: { CODE: 'VOWEL_NOT_AVAILABLE', MESSAGE: 'voyelle non disponible' } })
    expect(result.current.roueError).toMatchObject({ CODE: 'VOWEL_NOT_AVAILABLE', MESSAGE: 'voyelle non disponible' })
    act(() => result.current.clearRoueError())
    expect(result.current.roueError).toBeNull()
  })

  it('deux refus identiques consécutifs restent distinguables (SEQ croissant)', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))
    await push({ ACTION: 'ROUE_ERROR', MSG: { CODE: 'WRONG_SUBPHASE', MESSAGE: 'x' } })
    const first = result.current.roueError.SEQ
    await push({ ACTION: 'ROUE_ERROR', MSG: { CODE: 'WRONG_SUBPHASE', MESSAGE: 'x' } })
    expect(result.current.roueError.SEQ).toBeGreaterThan(first)
  })

  it('un ROUE_ERROR ne modifie pas l\'état de jeu', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))
    await update({ ROUE_SUBPHASE: 'LETTER' })
    await push({ ACTION: 'ROUE_ERROR', MSG: { CODE: 'NOT_A_LETTER', MESSAGE: 'x' } })
    expect(result.current.gameState.ROUE_SUBPHASE).toBe('LETTER')
  })
})
