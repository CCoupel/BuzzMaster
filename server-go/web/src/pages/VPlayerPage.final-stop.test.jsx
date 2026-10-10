import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, act, fireEvent } from '@testing-library/react'
import VPlayerPage from './VPlayerPage'
import { useGame } from '../hooks/GameContext'
import { createStorage, seedSession, makeGame, BOUND, sent, clickKey } from './vplayer246Helpers'

// #246 (F-STOP) — tests de l'implémentation tablette : texte final, accusé, états de l'overlay.
// (Complète les tests de spécification du test-writer, sans les modifier.)

const mockNavigate = vi.hoisted(() => vi.fn())
vi.mock('react-router-dom', () => ({ useNavigate: () => mockNavigate }))
vi.mock('../hooks/GameContext', () => ({ useGame: vi.fn() }))
vi.mock('./VPlayerPage.css', () => ({}))
vi.mock('nosleep.js', () => ({ default: class { enable() { return Promise.resolve() } disable() {} } }))
vi.mock('./PlayerDisplay', () => ({ default: vi.fn(() => null) }))

const txt = (c) => c.textContent.replace(/\s+/g, ' ')

describe('VPlayerPage — ARDOISE_FINAL et accusé (#246, F-STOP)', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    const ls = createStorage()
    vi.stubGlobal('localStorage', ls)
    vi.stubGlobal('sessionStorage', createStorage())
    useGame.mockReset()
    seedSession(ls)
  })
  afterEach(() => { vi.useRealTimers(); vi.unstubAllGlobals() })

  function mount() {
    const send = vi.fn(() => true)
    const st = { phase: 'STARTED', ack: null, status: 'connected' }
    useGame.mockImplementation(() => {
      const k = JSON.stringify({ phase: st.phase, ack: st.ack, status: st.status })
      if (!st.cur || st.cur.__k !== k) {
        st.cur = {
          ...makeGame(send, {
            status: st.status,
            playerConnectStatus: BOUND(),
            ardoiseFinalAck: st.ack,
            gameState: { phase: st.phase, question: { ID: 'q42', TYPE: 'ARDOISE', ARDOISE_KEYBOARD_TYPE: 'AZERTY' } },
          }),
          ardoiseFinalAck: st.ack,
          __k: k,
        }
      }
      return st.cur
    })
    const view = render(<VPlayerPage />)
    const set = async (patch) => { Object.assign(st, patch); await act(async () => { view.rerender(<VPlayerPage />) }) }
    return { send, view, set }
  }
  const type = async (m, s) => { for (const ch of s) await act(async () => { fireEvent.click(clickKey(m.view.container, ch === ' ' ? '␣' : ch)) }) }

  it('STOP : final avec QUESTION_ID, texte complet et hash ; « Envoi de votre réponse… »', async () => {
    const m = mount()
    await act(async () => { vi.advanceTimersByTime(100) })
    await type(m, 'PARIS EN FRANCE')
    await m.set({ phase: 'STOPPED' })
    expect(sent(m.send, 'ARDOISE_FINAL')[0]).toMatchObject({ QUESTION_ID: 'q42', TEXT: 'PARIS EN FRANCE', HASH: '066a01e6' })
    expect(txt(m.view.container)).toMatch(/Envoi de votre réponse…/)
  })

  it('accusé valide → « ✓ Réponse reçue » et arrêt des renvois ; accusé invalide ignoré', async () => {
    const m = mount()
    await act(async () => { vi.advanceTimersByTime(100) })
    await type(m, 'PARIS')
    await m.set({ phase: 'STOPPED' })
    await m.set({ ack: { questionId: 'q42', hash: 'deadbeef' } })
    expect(txt(m.view.container)).not.toMatch(/Réponse reçue/)
    await m.set({ ack: { questionId: 'q42', hash: 'e6d82fc0' } })
    expect(txt(m.view.container)).toMatch(/✓ Réponse reçue/)
    const n = sent(m.send, 'ARDOISE_FINAL').length
    await act(async () => { vi.advanceTimersByTime(5000) })
    expect(sent(m.send, 'ARDOISE_FINAL').length).toBe(n)
  })

  it('texte vide accusé : « ✓ Reçu — aucune réponse écrite » + « Aucune réponse »', async () => {
    const m = mount()
    await act(async () => { vi.advanceTimersByTime(100) })
    await m.set({ phase: 'STOPPED' })
    await m.set({ ack: { questionId: 'q42', hash: '811c9dc5' } })
    const t = txt(m.view.container)
    expect(t).toMatch(/✓ Reçu — aucune réponse écrite/)
    expect(t).toMatch(/Aucune réponse/)
  })

  it('PAUSE + accusé : « ⏸ Jeu en pause · ✓ réponse reçue » ; CONTINUE efface l\'état', async () => {
    const m = mount()
    await act(async () => { vi.advanceTimersByTime(100) })
    await type(m, 'PARIS')
    await m.set({ phase: 'PAUSED' })
    await m.set({ ack: { questionId: 'q42', hash: 'e6d82fc0' } })
    expect(txt(m.view.container)).toMatch(/Jeu en pause · ✓ réponse reçue/)
    await m.set({ phase: 'STARTED' })
    expect(txt(m.view.container)).not.toMatch(/réponse reçue/)
  })

  it('renvoie toutes les 1 s sans accusé ; connexion perdue → message dédié', async () => {
    const m = mount()
    await act(async () => { vi.advanceTimersByTime(100) })
    await type(m, 'AB')
    await m.set({ phase: 'STOPPED' })
    const n = sent(m.send, 'ARDOISE_FINAL').length
    await act(async () => { vi.advanceTimersByTime(3100) })
    expect(sent(m.send, 'ARDOISE_FINAL').length - n).toBeGreaterThanOrEqual(3)
    await m.set({ status: 'disconnected' })
    expect(txt(m.view.container)).toMatch(/Connexion perdue — votre réponse sera envoyée dès le retour du réseau/)
  })

  it('page montée après le STOP (rechargement) : aucun final vide envoyé', async () => {
    const send = vi.fn(() => true)
    const game = makeGame(send, { playerConnectStatus: BOUND(), gameState: { phase: 'STOPPED', question: { ID: 'q42', TYPE: 'ARDOISE' } } })
    useGame.mockImplementation(() => game)
    render(<VPlayerPage />)
    await act(async () => { vi.advanceTimersByTime(3000) })
    expect(sent(send, 'ARDOISE_FINAL')).toHaveLength(0)
  })
})
