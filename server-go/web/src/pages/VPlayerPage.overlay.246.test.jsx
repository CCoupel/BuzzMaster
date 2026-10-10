import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, act, fireEvent } from '@testing-library/react'
import VPlayerPage from './VPlayerPage'
import { useGame } from '../hooks/GameContext'
import { createStorage, seedSession, makeGame, BOUND, clickKey, OLD } from './vplayer246Helpers'

// #246 (v11.2, Batch 2) — messages de la tablette selon la maquette VALIDÉE
// docs/mockup/v11.2/ui/vplayer_ardoise_keyboard__final_ack.html et docs/mockup/DECISIONS.md
// (vplayer_ardoise_keyboard) :
//  - après STOP : « Envoi de votre réponse… » tant que l'accusé n'est pas reçu ;
//  - connexion perdue : « ⚠ Connexion perdue — votre réponse sera envoyée dès le retour du réseau » ;
//  - PAUSE : « ⏸ Jeu en pause · ⏳ envoi de votre réponse… » — AUCUNE invitation à continuer ou à écrire ;
//  - clavier verrouillé et vide : « Aucune réponse », jamais « Votre réponse… » (réservé à la saisie) ;
//  - écrans de session : onglet périmé « …utilisez l'autre » ; session ailleurs « Reprendre ici ».
// Les états « reçu » dépendent de l'accusé (voir VPlayerPage.final-ack.246.test.jsx).

const mockNavigate = vi.hoisted(() => vi.fn())
vi.mock('react-router-dom', () => ({ useNavigate: () => mockNavigate }))
vi.mock('../hooks/GameContext', () => ({ useGame: vi.fn() }))
vi.mock('./VPlayerPage.css', () => ({}))
vi.mock('nosleep.js', () => ({ default: class { enable() { return Promise.resolve() } disable() {} } }))
vi.mock('./PlayerDisplay', () => ({ default: vi.fn(() => null) }))

const text = (c) => c.textContent.replace(/\s+/g, ' ')

describe('VPlayerPage — messages ARDOISE après STOP / PAUSE (#246, maquette tablette)', () => {
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
    let phase = 'STARTED', status = 'connected'
    let cur = null, key = ''
    useGame.mockImplementation(() => {
      const k = `${phase}|${status}`
      if (k !== key) {
        cur = makeGame(send, { status, playerConnectStatus: BOUND(), gameState: { phase, question: { ID: 'q42', TYPE: 'ARDOISE', ARDOISE_KEYBOARD_TYPE: 'AZERTY' } } })
        key = k
      }
      return cur
    })
    const view = render(<VPlayerPage />)
    const re = async () => { await act(async () => { view.rerender(<VPlayerPage />) }) }
    return { send, view, setPhase: async (p) => { phase = p; await re() }, setStatus: async (s) => { status = s; await re() } }
  }
  const type = async (m, str) => { for (const ch of str) await act(async () => { fireEvent.click(clickKey(m.view.container, ch)) }) }

  it('saisie (STARTED) : texte indicatif « Votre réponse… » quand le clavier est vide', async () => {
    const m = mount()
    await act(async () => { vi.advanceTimersByTime(100) })
    expect(text(m.view.container)).toMatch(/Votre réponse…/)
    expect(text(m.view.container)).not.toMatch(/Aucune réponse/)
  })

  it('Critical — clavier verrouillé et vide après STOP : « Aucune réponse », jamais « Votre réponse… »', async () => {
    const m = mount()
    await act(async () => { vi.advanceTimersByTime(100) })
    await m.setPhase('STOPPED')
    expect(text(m.view.container)).toMatch(/Aucune réponse/)
    expect(text(m.view.container)).not.toMatch(/Votre réponse…/)
  })

  it('après STOP avec un texte : « Envoi de votre réponse… » tant que le serveur n’a pas accusé réception', async () => {
    const m = mount()
    await act(async () => { vi.advanceTimersByTime(100) })
    await type(m, 'PARIS')
    await m.setPhase('STOPPED')
    const t = text(m.view.container)
    expect(t).toMatch(/Envoi de votre réponse…/)
    expect(t).not.toMatch(/Réponse reçue|Réponse envoyée/) // plus de « envoyée » affiché sans preuve
  })

  it('Critical — PAUSE, envoi en cours : « Jeu en pause · envoi de votre réponse… » sans aucune invitation à continuer ou à écrire', async () => {
    const m = mount()
    await act(async () => { vi.advanceTimersByTime(100) })
    await type(m, 'PARIS')
    await m.setPhase('PAUSED')
    const t = text(m.view.container)
    expect(t).toMatch(/Jeu en pause/)
    expect(t).toMatch(/envoi de votre réponse…/)
    expect(t).not.toMatch(/continuer|vous pourrez|Votre réponse…/i)
  })

  it('PAUSE, texte vide : « Jeu en pause » et « Aucune réponse », pas de « Votre réponse… »', async () => {
    const m = mount()
    await act(async () => { vi.advanceTimersByTime(100) })
    await m.setPhase('PAUSED')
    const t = text(m.view.container)
    expect(t).toMatch(/Jeu en pause/)
    expect(t).toMatch(/Aucune réponse/)
    expect(t).not.toMatch(/Votre réponse…|continuer/i)
  })

  it('après STOP, connexion perdue : « Connexion perdue — votre réponse sera envoyée dès le retour du réseau »', async () => {
    const m = mount()
    await act(async () => { vi.advanceTimersByTime(100) })
    await type(m, 'PARIS')
    await m.setPhase('STOPPED')
    await m.setStatus('disconnected')
    expect(text(m.view.container)).toMatch(/Connexion perdue — votre réponse sera envoyée dès le retour du réseau/)
  })

  it('le clavier est verrouillé après STOP : aucune touche ne modifie plus le texte', async () => {
    const m = mount()
    await act(async () => { vi.advanceTimersByTime(100) })
    await type(m, 'AB')
    await m.setPhase('STOPPED')
    await act(async () => { fireEvent.click(clickKey(m.view.container, 'C')) })
    expect(m.view.container.querySelector('.ardoise-keyboard-text').textContent).toBe('AB')
  })
})

describe('VPlayerPage — écrans de session (#246, maquette tablette)', () => {
  let ls
  beforeEach(() => {
    vi.useFakeTimers()
    ls = createStorage()
    vi.stubGlobal('localStorage', ls)
    vi.stubGlobal('sessionStorage', createStorage())
    useGame.mockReset()
  })
  afterEach(() => { vi.useRealTimers(); vi.unstubAllGlobals() })

  it('onglet périmé : « Cette page n’est plus active » … « Fermez cet onglet et utilisez l’autre »', async () => {
    seedSession(ls, { name: 'Mamie', id: OLD })
    const send = vi.fn(() => true)
    const game = makeGame(send, { status: 'connected' })
    useGame.mockImplementation(() => game)
    const view = render(<VPlayerPage />)
    seedSession(ls, { name: 'Paul', session: '2' }) // un autre onglet a écrit sa session
    await act(async () => { window.dispatchEvent(new StorageEvent('storage', { key: 'vplayer_id' })) })

    const t = text(view.container)
    expect(t).toMatch(/Cette page n’est plus active|Cette page n'est plus active/)
    expect(t).toMatch(/Fermez cet onglet et utilisez l’autre|Fermez cet onglet et utilisez l'autre/)
    expect(t).not.toMatch(/continuez dans l’autre|continuez dans l'autre/)
  })

  it('session reprise ailleurs : « Votre session est ouverte ailleurs », « Ici, plus rien n’est envoyé », bouton « Reprendre ici »', async () => {
    seedSession(ls)
    const send = vi.fn(() => true)
    const game = makeGame(send, { playerEvictedStatus: { reason: 'DUPLICATE_SESSION' } })
    useGame.mockImplementation(() => game)
    const view = render(<VPlayerPage />)
    await act(async () => { vi.advanceTimersByTime(200) })

    const t = text(view.container)
    expect(t).toMatch(/Votre session est ouverte ailleurs/)
    expect(t).toMatch(/Ici, plus rien n’est envoyé|Ici, plus rien n'est envoyé/)
    expect(t).toMatch(/Reprendre ici/)
  })
})
