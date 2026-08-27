import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { renderHook } from '@testing-library/react'
import useWebSocket from './useWebSocket'

// ---------------------------------------------------------------------------
// useWebSocket — flipMemoryCard (#187, régression bug QUALIF cycle 7)
//
// `flipMemoryCard` a longtemps omis `payload.ID` (le bumper émetteur) —
// sans conséquence tant qu'aucune vérification serveur du tour n'existait,
// mais bloquant depuis que `handleFlipMemoryCard` (cmd/server/main.go,
// cycle 4+) résout l'émetteur par un motif 3 passes (payload.ID → msg.ID →
// clientID) pour un VJoueur, dont le 3e repli est documenté côté serveur
// comme NE correspondant PAS à la clé du bumper. Sans `ID` explicite, TOUT
// flip VJoueur était donc ignoré silencieusement, quelle que soit l'équipe.
//
// Ces tests figent le contrat de payload — mêmes conventions que
// ARDOISE_INPUT/VPLAYER_QCM_ANSWER (`{ID: bumper.id, ...}`).
// ---------------------------------------------------------------------------

let wsInstance = null

class MockWebSocket {
  constructor(url) {
    wsInstance = this
    this.url = url
    this.readyState = MockWebSocket.OPEN
    this.sent = []
  }
  send(data) {
    this.sent.push(data)
  }
  close() {
    this.readyState = MockWebSocket.CLOSED
    this.onclose && this.onclose()
  }
}
MockWebSocket.CONNECTING = 0
MockWebSocket.OPEN = 1
MockWebSocket.CLOSING = 2
MockWebSocket.CLOSED = 3

function lastSentPayload() {
  const raw = wsInstance.sent[wsInstance.sent.length - 1]
  const parsed = JSON.parse(raw)
  expect(parsed.ACTION).toBe('FLIP_MEMORY_CARD')
  return parsed.MSG
}

describe('useWebSocket — flipMemoryCard payload (#187, régression cycle 7)', () => {
  beforeEach(() => {
    wsInstance = null
    vi.stubGlobal('WebSocket', MockWebSocket)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    vi.clearAllMocks()
  })

  it('flipMemoryCard(cardId) seul (hôte question, TV/admin) — CARD_ID seulement, ni MOTION_CARD_ID ni ID', () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))
    result.current.flipMemoryCard('1-1')
    expect(lastSentPayload()).toEqual({ CARD_ID: '1-1' })
  })

  it('flipMemoryCard(cardId, motionCardId) (carte MEMOTION, TV/admin preview) — CARD_ID + MOTION_CARD_ID, pas de ID', () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))
    result.current.flipMemoryCard('1-1', 'card-3')
    expect(lastSentPayload()).toEqual({ CARD_ID: '1-1', MOTION_CARD_ID: 'card-3' })
  })

  // 🔴 Le test qui aurait détecté la régression : ID doit être transmis dès
  // qu'il est fourni par l'appelant — c'est le champ que le serveur utilise
  // en priorité (Pass 1) pour résoudre le bumper émetteur d'un VJoueur.
  it('🔴 flipMemoryCard(cardId, undefined, playerId) (VJoueur, hôte question) — CARD_ID + ID, jamais omis', () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))
    result.current.flipMemoryCard('1-1', undefined, 'bumper-42')
    expect(lastSentPayload()).toEqual({ CARD_ID: '1-1', ID: 'bumper-42' })
  })

  it('🔴 flipMemoryCard(cardId, motionCardId, playerId) (VJoueur, carte MEMOTION) — les 3 champs présents', () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))
    result.current.flipMemoryCard('1-1', 'card-3', 'bumper-42')
    expect(lastSentPayload()).toEqual({ CARD_ID: '1-1', MOTION_CARD_ID: 'card-3', ID: 'bumper-42' })
  })

  it('playerId falsy (null/undefined/"") n\'ajoute jamais la clé ID (comportement additif, pas de régression sur tv/anim)', () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))
    result.current.flipMemoryCard('1-1', 'card-3', null)
    expect(lastSentPayload()).toEqual({ CARD_ID: '1-1', MOTION_CARD_ID: 'card-3' })
  })
})
