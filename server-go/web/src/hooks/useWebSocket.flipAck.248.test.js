import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import useWebSocket from './useWebSocket'

// #248 lot 5b — useWebSocket : FLIP_MEMORY_CARD avec FLIP_ID (idempotence) et FLIP_MEMORY_CARD_ACK.
// Contrat : contracts/websocket-actions.md §FLIP_MEMORY_CARD (FLIP_ID), §FLIP_MEMORY_CARD_ACK.
// Signature : flipMemoryCard(cardId, motionCardId, playerId, flipId) — flipId optionnel (plan 5b, étape 4).
// Exposition (nom choisi par ces tests, à confirmer par dev-frontend, sur le modèle de ardoiseFinalAck) :
//   flipMemoryCardAck = { flipId, cardId, motionCardId, status, reason, duplicate } | null

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

describe('useWebSocket — FLIP_ID et FLIP_MEMORY_CARD_ACK (#248 lot 5b)', () => {
  beforeEach(() => { wsInstances = []; vi.useFakeTimers(); vi.stubGlobal('WebSocket', MockWebSocket) })
  afterEach(() => { vi.useRealTimers(); vi.unstubAllGlobals() })

  function open(endpoint = '/ws/player') {
    const hook = renderHook(() => useWebSocket(endpoint))
    const ws = wsInstances[0]
    act(() => { ws.simulateOpen() })
    return { hook, ws }
  }

  it('Critical — flipMemoryCard(cardId, undefined, playerId, flipId) : CARD_ID + ID + FLIP_ID', () => {
    const { hook, ws } = open()
    act(() => { hook.result.current.flipMemoryCard('1-1', undefined, 'bumper-42', 'f-3b9c2e') })
    expect(sentOf(ws, 'FLIP_MEMORY_CARD')).toEqual([{ CARD_ID: '1-1', ID: 'bumper-42', FLIP_ID: 'f-3b9c2e' }])
  })

  it('carte MEMOTION : CARD_ID + MOTION_CARD_ID + ID + FLIP_ID', () => {
    const { hook, ws } = open()
    act(() => { hook.result.current.flipMemoryCard('2-2', 'card-3', 'bumper-42', 'abc_DEF-123') })
    expect(sentOf(ws, 'FLIP_MEMORY_CARD')).toEqual([{ CARD_ID: '2-2', MOTION_CARD_ID: 'card-3', ID: 'bumper-42', FLIP_ID: 'abc_DEF-123' }])
  })

  it('non-régression — sans flipId : aucune clé FLIP_ID (comportement actuel, hors tour compris)', () => {
    const { hook, ws } = open()
    act(() => { hook.result.current.flipMemoryCard('1-1', undefined, 'bumper-42') })
    act(() => { hook.result.current.flipMemoryCard('1-1', 'card-3', 'bumper-42', '') })
    act(() => { hook.result.current.flipMemoryCard('1-1', 'card-3', 'bumper-42', null) })
    for (const m of sentOf(ws, 'FLIP_MEMORY_CARD')) expect('FLIP_ID' in m).toBe(false)
  })

  it('Critical — un renvoi porte le MÊME FLIP_ID (le hook n\'en fabrique jamais un nouveau)', () => {
    const { hook, ws } = open()
    act(() => { hook.result.current.flipMemoryCard('1-1', undefined, 'bumper-42', 'f-same') })
    act(() => { hook.result.current.flipMemoryCard('1-1', undefined, 'bumper-42', 'f-same') })
    const sent = sentOf(ws, 'FLIP_MEMORY_CARD')
    expect(sent).toHaveLength(2)
    expect(sent[0].FLIP_ID).toBe('f-same')
    expect(sent[1].FLIP_ID).toBe('f-same')
  })

  it('FLIP_MEMORY_CARD_ACK APPLIED reçu : exposé avec tous ses champs', () => {
    const { hook, ws } = open()
    act(() => {
      ws.simulateMessage({ ACTION: 'FLIP_MEMORY_CARD_ACK', MSG: { FLIP_ID: 'f-3b9c2e', CARD_ID: '2-1', MOTION_CARD_ID: '', STATUS: 'APPLIED', REASON: '', DUPLICATE: true } })
    })
    const a = hook.result.current.flipMemoryCardAck
    expect(a, 'le hook doit exposer flipMemoryCardAck après FLIP_MEMORY_CARD_ACK').toBeTruthy()
    expect(a).toMatchObject({ flipId: 'f-3b9c2e', cardId: '2-1', motionCardId: '', status: 'APPLIED', reason: '', duplicate: true })
  })

  it('FLIP_MEMORY_CARD_ACK IGNORED + NOT_TURN reçu : motif exposé', () => {
    const { hook, ws } = open()
    act(() => {
      ws.simulateMessage({ ACTION: 'FLIP_MEMORY_CARD_ACK', MSG: { FLIP_ID: 'f-9', CARD_ID: '1-2', MOTION_CARD_ID: 'card-3', STATUS: 'IGNORED', REASON: 'NOT_TURN', DUPLICATE: false } })
    })
    expect(hook.result.current.flipMemoryCardAck).toMatchObject({ flipId: 'f-9', motionCardId: 'card-3', status: 'IGNORED', reason: 'NOT_TURN', duplicate: false })
  })

  it('un accusé plus récent remplace le précédent (dernier accusé exposé)', () => {
    const { hook, ws } = open()
    act(() => { ws.simulateMessage({ ACTION: 'FLIP_MEMORY_CARD_ACK', MSG: { FLIP_ID: 'a', CARD_ID: '1-1', STATUS: 'APPLIED', REASON: '', DUPLICATE: false } }) })
    act(() => { ws.simulateMessage({ ACTION: 'FLIP_MEMORY_CARD_ACK', MSG: { FLIP_ID: 'b', CARD_ID: '1-2', STATUS: 'IGNORED', REASON: 'NOT_FLIPPABLE', DUPLICATE: false } }) })
    expect(hook.result.current.flipMemoryCardAck.flipId).toBe('b')
  })

  it('aucun accusé avant réception : null', () => {
    const { hook } = open()
    expect(hook.result.current.flipMemoryCardAck ?? null).toBeNull()
  })

  it('une action inconnue ne casse rien (l\'accusé reste inchangé)', () => {
    const { hook, ws } = open()
    act(() => { ws.simulateMessage({ ACTION: 'FLIP_MEMORY_CARD_ACK', MSG: { FLIP_ID: 'a', CARD_ID: '1-1', STATUS: 'APPLIED', REASON: '', DUPLICATE: false } }) })
    expect(() => act(() => { ws.simulateMessage({ ACTION: 'ACTION_QUI_NEXISTE_PAS', MSG: {} }) })).not.toThrow()
    expect(hook.result.current.flipMemoryCardAck.flipId).toBe('a')
  })
})
