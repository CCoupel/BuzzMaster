import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { useAckedSend, ACKED_SEND_INTERVAL_MS } from './useAckedSend'

// #248 lot 5b — clé de geste de useAckedSend (retournement MEMORY/MEMOTION : plusieurs gestes par question,
// chacun avec son FLIP_ID). Contrat fixé par ces tests, à confirmer par dev-frontend (le plan ne nomme pas les
// paramètres) :
//   start(action, payload, gestureKey?)  — avec clé : un envoi par (question, clé) ; sans clé : un seul appui par
//                                          question, comme au lot 5a (non-régression).
//   option du hook  refused: string|null — motif de refus reçu du serveur (ex. 'NOT_TURN') : passe l'envoi en
//                                          cours en `failed`, sans renvoi ; ignoré au repos.
//   retour          failReason: string|null — le motif de refus quand status === 'failed' par refus, sinon null.
// Les cas « sans clé » passent déjà aujourd'hui (gardes de non-régression du lot 5a).

const ACTION = 'FLIP_MEMORY_CARD'
const P1 = { CARD_ID: '1-1', ID: 'p', FLIP_ID: 'f-1' }
const P2 = { CARD_ID: '2-1', ID: 'p', FLIP_ID: 'f-2' }

function setup(initial = {}) {
  const sendBound = vi.fn(() => true)
  const props = { sendBound, connStatus: 'connected', bound: true, phase: 'STARTED', questionId: 'q1', acked: false, teamServed: false, refused: null, ...initial }
  const hook = renderHook((p) => useAckedSend(p), { initialProps: { ...props } })
  return {
    hook, sendBound, props,
    update: (patch) => { Object.assign(props, patch); hook.rerender({ ...props }) },
    advance: (ms) => act(async () => { vi.advanceTimersByTime(ms) }),
    start: (...a) => { let r; act(() => { r = hook.result.current.start(...a) }); return r },
    get: () => hook.result.current,
    sends: () => sendBound.mock.calls.map((c) => c[1]),
  }
}

describe('useAckedSend — clé de geste (#248 lot 5b)', () => {
  beforeEach(() => { vi.useFakeTimers(); Object.defineProperty(document, 'visibilityState', { value: 'visible', configurable: true }) })
  afterEach(() => { vi.useRealTimers() })

  it('non-régression 5a — sans clé : un seul appui par question', () => {
    const h = setup()
    expect(h.start(ACTION, P1)).toBe(true)
    expect(h.start(ACTION, P2)).toBe(false)
    expect(h.sends()).toEqual([P1])
  })

  it('Critical — avec clé : un 2ᵉ geste (autre clé) dans la même question est accepté après l\'accusé du 1er', () => {
    const h = setup()
    expect(h.start(ACTION, P1, 'f-1')).toBe(true)
    h.update({ acked: true })
    expect(h.get().status).toBe('acked')
    h.update({ acked: false })
    expect(h.start(ACTION, P2, 'f-2')).toBe(true)
    expect(h.get().status).toBe('sending')
    expect(h.sends()).toEqual([P1, P2])
  })

  it('même clé deux fois : le second start est refusé (un geste = un FLIP_ID)', () => {
    const h = setup()
    expect(h.start(ACTION, P1, 'f-1')).toBe(true)
    expect(h.start(ACTION, P1, 'f-1')).toBe(false)
    expect(h.sends()).toEqual([P1])
  })

  it('Critical — le geste suivant remplace l\'ancien : les renvois portent le NOUVEAU payload, jamais l\'ancien', async () => {
    const h = setup()
    h.start(ACTION, P1, 'f-1')
    h.update({ acked: true })
    h.update({ acked: false })
    h.start(ACTION, P2, 'f-2')
    await h.advance(ACKED_SEND_INTERVAL_MS + 100)
    const after = h.sends().slice(1)
    expect(after.length).toBeGreaterThanOrEqual(2) // P2 initial + 1 renvoi
    expect(after.every((p) => p.FLIP_ID === 'f-2')).toBe(true)
  })

  it('renvoi d\'un geste : toujours le MÊME payload (même FLIP_ID), jamais un identifiant neuf', async () => {
    const h = setup()
    h.start(ACTION, P1, 'f-1')
    await h.advance(ACKED_SEND_INTERVAL_MS * 3 + 100)
    expect(h.sends().length).toBe(4)
    expect(h.sends().every((p) => p.FLIP_ID === 'f-1')).toBe(true)
  })

  it('refusé (NOT_TURN) pendant l\'envoi : failed avec le motif, plus aucun renvoi', async () => {
    const h = setup()
    h.start(ACTION, P1, 'f-1')
    h.update({ refused: 'NOT_TURN' })
    expect(h.get().status).toBe('failed')
    expect(h.get().failReason).toBe('NOT_TURN')
    const n = h.sends().length
    await h.advance(ACKED_SEND_INTERVAL_MS * 5)
    expect(h.sends().length).toBe(n)
  })

  it('refus reçu au repos : ignoré (aucun état d\'échec sans geste en cours)', () => {
    const h = setup()
    h.update({ refused: 'NOT_FLIPPABLE' })
    expect(h.get().status).toBe('idle')
  })

  it('un geste refusé n\'empêche pas le geste suivant, avec une autre clé', () => {
    const h = setup()
    h.start(ACTION, P1, 'f-1')
    h.update({ refused: 'NOT_FLIPPABLE' })
    expect(h.get().status).toBe('failed')
    h.update({ refused: null })
    expect(h.start(ACTION, P2, 'f-2')).toBe(true)
    expect(h.get().status).toBe('sending')
    expect(h.get().failReason ?? null).toBeNull()
  })

  it('question changée : un nouvel appui est possible avec la même clé de geste (clé par question)', () => {
    const h = setup()
    h.start(ACTION, P1, 'f-1')
    h.update({ questionId: 'q2', phase: 'PREPARE' })
    h.update({ phase: 'STARTED' })
    expect(h.start(ACTION, P1, 'f-1')).toBe(true)
  })
})
