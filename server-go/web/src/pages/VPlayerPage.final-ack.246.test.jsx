import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, act, fireEvent } from '@testing-library/react'
import VPlayerPage from './VPlayerPage'
import { useGame } from '../hooks/GameContext'
import { createStorage, seedSession, makeGame, BOUND, sent, clickKey, PAUL } from './vplayer246Helpers'

// #246 (v11.2) — BATCH 2 (F-STOP) : texte final à la sortie de STARTED. Écrits depuis le contrat
// (contracts/websocket-actions.md §ARDOISE_FINAL) ; désactivés tant que F-STOP n'est pas livré
// (TEST_246_BATCH2=1 pour les exécuter). Les tests d'interface dérivés des maquettes (overlay,
// pastilles, compteur, confirmation « Révéler quand même ») sont écrits après leur validation.
const batch2 = process.env.TEST_246_BATCH2 ? describe : describe.skip

const mockNavigate = vi.hoisted(() => vi.fn())
vi.mock('react-router-dom', () => ({ useNavigate: () => mockNavigate }))
vi.mock('../hooks/GameContext', () => ({ useGame: vi.fn() }))
vi.mock('./VPlayerPage.css', () => ({}))
vi.mock('nosleep.js', () => ({ default: class { enable() { return Promise.resolve() } disable() {} } }))
vi.mock('./PlayerDisplay', () => ({ default: vi.fn(() => null) }))

// Vecteur du contrat : FNV-1a("PARIS EN FRANCE") = 066a01e6
batch2('VPlayerPage — ARDOISE_FINAL (#246, Batch 2)', () => {
  let ls
  beforeEach(() => {
    vi.useFakeTimers()
    ls = createStorage()
    vi.stubGlobal('localStorage', ls)
    vi.stubGlobal('sessionStorage', createStorage())
    useGame.mockReset()
    seedSession(ls)
  })
  afterEach(() => { vi.useRealTimers(); vi.unstubAllGlobals() })

  function mount() {
    const send = vi.fn(() => true)
    let phase = 'STARTED', status = 'connected', bound = true
    let cur = null, key = ''
    useGame.mockImplementation(() => {
      const k = `${phase}|${status}|${bound}`
      if (k !== key) {
        cur = makeGame(send, { status, playerConnectStatus: bound ? BOUND() : null, gameState: { phase, question: { ID: 'q42', TYPE: 'ARDOISE', ARDOISE_KEYBOARD_TYPE: 'AZERTY' } } })
        key = k
      }
      return cur
    })
    const view = render(<VPlayerPage />)
    const re = async () => { await act(async () => { view.rerender(<VPlayerPage />) }) }
    return { send, view, setPhase: async (p) => { phase = p; await re() }, setStatus: async (s) => { status = s; bound = false; await re() }, bind: async () => { bound = true; await re() } }
  }
  const type = async (m, text) => {
    for (const ch of text) await act(async () => { fireEvent.click(clickKey(m.view.container, ch === ' ' ? '␣' : ch)) })
  }

  it('Critical — au STOP, envoie le texte COMPLET (debounce en attente inclus) avec QUESTION_ID et HASH FNV-1a', async () => {
    const m = mount()
    await act(async () => { vi.advanceTimersByTime(100) })
    await type(m, 'PARIS EN FRANCE') // frappe continue : aucune pause de 200 ms

    await m.setPhase('STOPPED')

    const finals = sent(m.send, 'ARDOISE_FINAL')
    expect(finals.length).toBeGreaterThanOrEqual(1)
    expect(finals[0]).toMatchObject({ ID: PAUL, QUESTION_ID: 'q42', TEXT: 'PARIS EN FRANCE', HASH: '066a01e6' })
  })

  it('renvoie toutes les 1 s tant qu’aucun accusé valide n’est reçu', async () => {
    const m = mount()
    await act(async () => { vi.advanceTimersByTime(100) })
    await type(m, 'PARIS EN FRANCE')
    await m.setPhase('STOPPED')
    const n0 = sent(m.send, 'ARDOISE_FINAL').length

    await act(async () => { vi.advanceTimersByTime(3200) })

    expect(sent(m.send, 'ARDOISE_FINAL').length - n0).toBeGreaterThanOrEqual(3)
  })

  it('PAUSE envoie aussi le final ; le clavier est gelé (plus aucun ARDOISE_INPUT après STOP)', async () => {
    const m = mount()
    await act(async () => { vi.advanceTimersByTime(100) })
    await type(m, 'AB')
    await m.setPhase('PAUSED')
    expect(sent(m.send, 'ARDOISE_FINAL').length).toBeGreaterThanOrEqual(1)

    const before = sent(m.send, 'ARDOISE_INPUT').length
    await act(async () => { fireEvent.click(clickKey(m.view.container, 'C')) })
    await act(async () => { vi.advanceTimersByTime(500) })
    expect(sent(m.send, 'ARDOISE_INPUT').length).toBe(before)
  })

  it('renvoie le final immédiatement après une nouvelle liaison de connexion', async () => {
    const m = mount()
    await act(async () => { vi.advanceTimersByTime(100) })
    await type(m, 'PARIS')
    await m.setPhase('STOPPED')
    await m.setStatus('disconnected')
    await m.setStatus('connected')
    const n = sent(m.send, 'ARDOISE_FINAL').length

    await m.bind() // PLAYER_CONNECTED de la nouvelle connexion
    await act(async () => { vi.advanceTimersByTime(50) })

    expect(sent(m.send, 'ARDOISE_FINAL').length).toBeGreaterThan(n)
  })

  it('texte vide : envoie un final vide (« rien écrit »)', async () => {
    const m = mount()
    await act(async () => { vi.advanceTimersByTime(100) })
    await m.setPhase('STOPPED')
    expect(sent(m.send, 'ARDOISE_FINAL')[0]).toMatchObject({ TEXT: '', HASH: '811c9dc5' })
  })
})
