import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, join } from 'node:path'
import useWebSocket from './useWebSocket'

// #312 (v12.1.0) — le HOOK RÉEL porte les champs voyelles du GameState (contracts/roue.md §15.1.2) : l'écran ne voit que ce que le hook
// recopie. Scaffold de useWebSocket.roue.144.test.js (le faux WebSocket définit CONNECTING/OPEN/CLOSING/CLOSED, sinon le hook plante).
// Un test précédent (roueFields.312) n'est que textuel ; celui-ci exécute le hook, et rejoue un GameState sérialisé par le vrai moteur Go.

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

const VOWEL_KEYS = ['ROUE_BUY_VOWELS', 'ROUE_VOWEL_TIMING', 'ROUE_VOWELS_BEFORE', 'ROUE_VOWELS_AFTER', 'ROUE_VOWEL_PRICE', 'ROUE_VOWEL_GAIN',
  'ROUE_VOWELS_LEFT', 'ROUE_TURN_VOWEL', 'ROUE_STOP_VOWEL']

describe('useWebSocket — champs voyelles (hook réel)', () => {
  it('état initial : les 9 champs existent, jamais null / undefined', () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))
    for (const k of VOWEL_KEYS) {
      expect(result.current.gameState[k], k).not.toBeUndefined()
      expect(result.current.gameState[k], k).not.toBeNull()
    }
  })

  it('un UPDATE porteur des 9 champs les reprend tels quels', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))
    await update({
      ROUE_BUY_VOWELS: true, ROUE_VOWEL_TIMING: 'AFTER', ROUE_VOWELS_BEFORE: 'ONE', ROUE_VOWELS_AFTER: 'MANY',
      ROUE_VOWEL_PRICE: 300, ROUE_VOWEL_GAIN: 'REFUND', ROUE_VOWELS_LEFT: true, ROUE_TURN_VOWEL: true, ROUE_STOP_VOWEL: true,
    })
    const g = result.current.gameState
    expect(g.ROUE_BUY_VOWELS).toBe(true)
    expect(g.ROUE_VOWEL_TIMING).toBe('AFTER')
    expect(g.ROUE_VOWELS_BEFORE).toBe('ONE')
    expect(g.ROUE_VOWELS_AFTER).toBe('MANY')
    expect(g.ROUE_VOWEL_PRICE).toBe(300)
    expect(g.ROUE_VOWEL_GAIN).toBe('REFUND')
    expect(g.ROUE_VOWELS_LEFT).toBe(true)
    expect(g.ROUE_TURN_VOWEL).toBe(true)
    expect(g.ROUE_STOP_VOWEL).toBe(true)
  })

  it('critical — ROUE_BUY_VOWELS = true est conservé après un UPDATE partiel (sans champ voyelle)', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))
    await update({ ROUE_BUY_VOWELS: true, ROUE_VOWEL_TIMING: 'BOTH', ROUE_VOWELS_LEFT: true, ROUE_VOWEL_PRICE: 250 })
    await update({ CURRENT_TIME: 3 })
    const g = result.current.gameState
    expect(g.ROUE_BUY_VOWELS).toBe(true)
    expect(g.ROUE_VOWEL_TIMING).toBe('BOTH')
    expect(g.ROUE_VOWELS_LEFT).toBe(true)
    expect(g.ROUE_VOWEL_PRICE).toBe(250)
  })

  it('une valeur fausse mais définie n\'est pas écrasée par le défaut (false, 0, "")', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))
    await update({ ROUE_BUY_VOWELS: true, ROUE_VOWELS_LEFT: true, ROUE_TURN_VOWEL: true, ROUE_STOP_VOWEL: true, ROUE_VOWEL_PRICE: 250, ROUE_VOWEL_TIMING: 'BOTH' })
    await update({ ROUE_BUY_VOWELS: false, ROUE_VOWELS_LEFT: false, ROUE_TURN_VOWEL: false, ROUE_STOP_VOWEL: false, ROUE_VOWEL_PRICE: 0, ROUE_VOWEL_TIMING: '' })
    const g = result.current.gameState
    expect(g.ROUE_BUY_VOWELS).toBe(false)
    expect(g.ROUE_VOWELS_LEFT).toBe(false)
    expect(g.ROUE_TURN_VOWEL).toBe(false)
    expect(g.ROUE_STOP_VOWEL).toBe(false)
    expect(g.ROUE_VOWEL_PRICE).toBe(0)
    expect(g.ROUE_VOWEL_TIMING).toBe('')
  })
})

// ---- raccord Go → hook ----------------------------------------------------------------------------------------------

describe('useWebSocket — GameState sérialisé par le vrai moteur Go', () => {
  const here = dirname(fileURLToPath(import.meta.url))
  // Fixture gardée fidèle au moteur par internal/game/roue_gamestate_fixture_312_test.go (régénération : UPDATE_ROUE_FIXTURE=1).
  const fixture = JSON.parse(readFileSync(join(here, 'fixtures', 'roue-gamestate.312.json'), 'utf8'))

  it('la fixture porte bien les champs voyelles et BONUS du moteur', () => {
    for (const k of VOWEL_KEYS) expect(Object.keys(fixture.GAME), k).toContain(k)
    expect(fixture.GAME.ROUE_BUY_VOWELS).toBe(true)
  })

  it('critical — chaque champ ROUE_* émis par le moteur ressort à l\'identique du hook (aucun champ perdu)', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))
    await push({ ACTION: 'UPDATE', MSG: fixture })
    const g = result.current.gameState
    for (const [k, v] of Object.entries(fixture.GAME)) {
      expect(g[k], `${k} perdu ou altéré par le hook`).toEqual(v)
    }
  })

  it('ROUE_BUY_VOWELS et réglages du moteur survivent à un UPDATE partiel', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))
    await push({ ACTION: 'UPDATE', MSG: fixture })
    await update({ CURRENT_TIME: 1 })
    expect(result.current.gameState.ROUE_BUY_VOWELS).toBe(true)
    expect(result.current.gameState.ROUE_VOWEL_TIMING).toBe(fixture.GAME.ROUE_VOWEL_TIMING)
    expect(result.current.gameState.ROUE_VOWEL_PRICE).toBe(fixture.GAME.ROUE_VOWEL_PRICE)
  })
})
