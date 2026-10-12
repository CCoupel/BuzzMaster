import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { useAckedSend, ACKED_SEND_INTERVAL_MS, ACKED_SEND_MAX } from './useAckedSend'

// #248 — contrat du hook useAckedSend (signature figée par le commit 1c1ce700) :
//   useAckedSend({ sendBound, connStatus, bound, phase, questionId, acked, teamServed })
//     -> { status, resending, start(action, payload), clear() }
// États : idle / sending / acked / served / failed. Horloge simulée, aucune attente réelle.
// Les cas qui passent déjà aujourd'hui sont des gardes de non-régression ; le défaut majeur
// relevé en revue (équipe déjà servie à l'appui) est le cas « RED ».

const ACTION = 'VPLAYER_QCM_ANSWER'
const PAYLOAD = { ID: 'p1', ANSWER_COLOR: 'GREEN' }

function setup(initial = {}) {
  const sendBound = vi.fn(() => true)
  const base = { sendBound, connStatus: 'connected', bound: true, phase: 'STARTED', questionId: 'q1', acked: false, teamServed: false }
  const hook = renderHook((props) => useAckedSend(props), { initialProps: { ...base, ...initial } })
  const props = { ...base, ...initial }
  const update = (patch) => { Object.assign(props, patch); hook.rerender({ ...props }) }
  const advance = (ms) => act(async () => { vi.advanceTimersByTime(ms) })
  const start = () => { let r; act(() => { r = hook.result.current.start(ACTION, PAYLOAD) }); return r }
  const sends = () => sendBound.mock.calls.length
  return { hook, sendBound, update, advance, start, sends, get: () => hook.result.current }
}

describe('useAckedSend (#248)', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    Object.defineProperty(document, 'visibilityState', { value: 'visible', configurable: true })
  })
  afterEach(() => { vi.useRealTimers() })

  it('repos : idle, aucun envoi', () => {
    const h = setup()
    expect(h.get().status).toBe('idle')
    expect(h.get().resending).toBe(false)
    expect(h.sends()).toBe(0)
  })

  it('start : renvoie true, passe en sending et envoie une fois l\'action et le payload', () => {
    const h = setup()
    expect(h.start()).toBe(true)
    expect(h.get().status).toBe('sending')
    expect(h.sendBound).toHaveBeenCalledTimes(1)
    expect(h.sendBound).toHaveBeenCalledWith(ACTION, PAYLOAD)
  })

  it('un appui par question : le second start renvoie false et n\'envoie rien', () => {
    const h = setup()
    expect(h.start()).toBe(true)
    expect(h.start()).toBe(false)
    expect(h.sends()).toBe(1)
  })

  it('une nouvelle question accepte de nouveau un appui', () => {
    const h = setup()
    h.start()
    h.update({ acked: true })
    h.update({ questionId: 'q2', acked: false })
    expect(h.start()).toBe(true)
    expect(h.sends()).toBe(2)
  })

  it('cadence : rien avant 4 s, un renvoi par intervalle de 4 s, sans accusé', async () => {
    const h = setup()
    h.start()
    await h.advance(ACKED_SEND_INTERVAL_MS - 100)
    expect(h.sends()).toBe(1)
    await h.advance(200)
    expect(h.sends()).toBe(2)
    await h.advance(ACKED_SEND_INTERVAL_MS)
    expect(h.sends()).toBe(3)
    expect(h.get().status).toBe('sending')
  })

  it('plafond : 30 envois au total puis failed, plus aucun envoi', async () => {
    const h = setup()
    h.start()
    await h.advance(ACKED_SEND_INTERVAL_MS * (ACKED_SEND_MAX + 10))
    expect(h.sends()).toBe(ACKED_SEND_MAX)
    expect(h.get().status).toBe('failed')
    await h.advance(ACKED_SEND_INTERVAL_MS * 5)
    expect(h.sends()).toBe(ACKED_SEND_MAX)
  })

  it('accusé : passe en acked et n\'envoie plus jamais', async () => {
    const h = setup()
    h.start()
    h.update({ acked: true })
    expect(h.get().status).toBe('acked')
    await h.advance(ACKED_SEND_INTERVAL_MS * 10)
    expect(h.sends()).toBe(1)
  })

  it('équipe servie PENDANT l\'attente : passe en served et n\'envoie plus', async () => {
    const h = setup()
    h.start()
    h.update({ teamServed: true })
    expect(h.get().status).toBe('served')
    await h.advance(ACKED_SEND_INTERVAL_MS * 10)
    expect(h.sends()).toBe(1)
  })

  it('Critical (défaut majeur de revue) — équipe DÉJÀ servie à l\'appui : état served, jamais failed, même après 30 intervalles', async () => {
    const h = setup({ teamServed: true })
    h.start()
    await h.advance(0)
    expect(h.get().status).toBe('served')
    await h.advance(ACKED_SEND_INTERVAL_MS * (ACKED_SEND_MAX + 5))
    expect(h.get().status).toBe('served')
    expect(h.get().status).not.toBe('failed')
    expect(h.sends()).toBeLessThanOrEqual(1) // au plus l'envoi initial, aucun renvoi
  })

  it('pas de liaison : aucun envoi ; à la liaison, envoi et resending = true', async () => {
    const h = setup({ bound: false })
    h.start()
    expect(h.sends()).toBe(0)
    await h.advance(ACKED_SEND_INTERVAL_MS * 3)
    expect(h.sends()).toBe(0)
    h.update({ bound: true })
    expect(h.sends()).toBe(1)
    expect(h.get().status).toBe('sending')
  })

  it('reliaison après un premier envoi : renvoi immédiat marqué resending', async () => {
    const h = setup()
    h.start()
    h.update({ connStatus: 'disconnected', bound: false })
    await h.advance(ACKED_SEND_INTERVAL_MS * 3)
    expect(h.sends()).toBe(1)
    h.update({ connStatus: 'connected', bound: true })
    expect(h.sends()).toBe(2)
    expect(h.get().resending).toBe(true)
  })

  it('retour au premier plan (visibilitychange) : renvoi immédiat', () => {
    const h = setup()
    h.start()
    act(() => { document.dispatchEvent(new Event('visibilitychange')) })
    expect(h.sends()).toBe(2)
  })

  it('PAUSED : aucun envoi pendant la pause (reste sending) ; retour en STARTED : renvoi', async () => {
    const h = setup()
    h.start()
    h.update({ phase: 'PAUSED' })
    await h.advance(ACKED_SEND_INTERVAL_MS * 5)
    expect(h.sends()).toBe(1)
    expect(h.get().status).toBe('sending')
    h.update({ phase: 'STARTED' })
    expect(h.sends()).toBe(2)
  })

  it('phase terminale sans accusé (REVEALED, STOPPED) : failed et plus aucun envoi', async () => {
    for (const phase of ['REVEALED', 'STOPPED']) {
      const h = setup()
      h.start()
      h.update({ phase })
      expect(h.get().status).toBe('failed')
      await h.advance(ACKED_SEND_INTERVAL_MS * 3)
      expect(h.sends()).toBe(1)
    }
  })

  it('question changée sans accusé : failed, aucun envoi vers la nouvelle question', async () => {
    const h = setup()
    h.start()
    h.update({ questionId: 'q2' })
    expect(h.get().status).toBe('failed')
    await h.advance(ACKED_SEND_INTERVAL_MS * 3)
    expect(h.sends()).toBe(1)
  })

  it('PREPARE (question suivante) : retour à idle', () => {
    const h = setup()
    h.start()
    h.update({ phase: 'REVEALED' })
    expect(h.get().status).toBe('failed')
    h.update({ phase: 'PREPARE', questionId: 'q2' })
    expect(h.get().status).toBe('idle')
  })

  it('clear : retour à idle, plus aucun envoi', async () => {
    const h = setup()
    h.start()
    act(() => { h.get().clear() })
    expect(h.get().status).toBe('idle')
    await h.advance(ACKED_SEND_INTERVAL_MS * 3)
    expect(h.sends()).toBe(1)
  })
})
