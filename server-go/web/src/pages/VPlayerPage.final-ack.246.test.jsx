import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, act, fireEvent } from '@testing-library/react'
import VPlayerPage from './VPlayerPage'
import { useGame } from '../hooks/GameContext'
import { createStorage, seedSession, makeGame, BOUND, sent, clickKey, PAUL } from './vplayer246Helpers'

// #246 (v11.2, Batch 2 — F-STOP) : texte final à la sortie de STARTED et accusé de réception.
// Contrat : contracts/websocket-actions.md §ARDOISE_FINAL, §ARDOISE_FINAL_ACK. Les messages affichés
// (maquette validée) sont dans VPlayerPage.overlay.246.test.jsx ; ici : émission, renvoi, accusé.

const mockNavigate = vi.hoisted(() => vi.fn())
vi.mock('react-router-dom', () => ({ useNavigate: () => mockNavigate }))
vi.mock('../hooks/GameContext', () => ({ useGame: vi.fn() }))
vi.mock('./VPlayerPage.css', () => ({}))
vi.mock('nosleep.js', () => ({ default: class { enable() { return Promise.resolve() } disable() {} } }))
vi.mock('./PlayerDisplay', () => ({ default: vi.fn(() => null) }))

// Vecteur du contrat : FNV-1a("PARIS EN FRANCE") = 066a01e6
describe('VPlayerPage — ARDOISE_FINAL (#246, Batch 2)', () => {
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

  describe('accusé ARDOISE_FINAL_ACK', () => {
    function mountAck({ phase: phase0 = 'STARTED' } = {}) {
      const send = vi.fn(() => true)
      let phase = phase0, ack = null
      let cur = null, key = null
      const build = () => makeGame(send, {
        playerConnectStatus: BOUND(), ardoiseFinalAck: ack,
        gameState: { phase, question: { ID: 'q42', TYPE: 'ARDOISE', ARDOISE_KEYBOARD_TYPE: 'AZERTY' } },
      })
      useGame.mockImplementation(() => {
        const k = `${phase}|${JSON.stringify(ack)}`
        if (k !== key) { cur = build(); key = k }
        return cur
      })
      const view = render(<VPlayerPage />)
      const re = async () => { await act(async () => { view.rerender(<VPlayerPage />) }) }
      return {
        send, view,
        setPhase: async (p) => { phase = p; await re() },
        ack: async (a) => { ack = a; await re() },
      }
    }
    const txt = (c) => c.textContent.replace(/\s+/g, ' ')
    const typeAll = async (m, s) => { for (const ch of s) await act(async () => { fireEvent.click(clickKey(m.view.container, ch === ' ' ? '␣' : ch)) }) }

    it('Critical — accusé valide (même QUESTION_ID et même HASH) : « ✓ Réponse reçue » et arrêt des renvois', async () => {
      const m = mountAck()
      await act(async () => { vi.advanceTimersByTime(100) })
      await typeAll(m, 'PARIS EN FRANCE')
      await m.setPhase('STOPPED')
      expect(txt(m.view.container)).toMatch(/Envoi de votre réponse…/)

      await m.ack({ questionId: 'q42', hash: '066a01e6' })

      expect(txt(m.view.container)).toMatch(/✓ Réponse reçue/)
      const n = sent(m.send, 'ARDOISE_FINAL').length
      await act(async () => { vi.advanceTimersByTime(5000) })
      expect(sent(m.send, 'ARDOISE_FINAL').length).toBe(n)
    })

    it('accusé d’une autre question ou d’un autre hash : ignoré, toujours « Envoi… », le renvoi continue', async () => {
      const m = mountAck()
      await act(async () => { vi.advanceTimersByTime(100) })
      await typeAll(m, 'PARIS')
      await m.setPhase('STOPPED')
      await m.ack({ questionId: 'q-autre', hash: '246f9d40' })
      await m.ack({ questionId: 'q42', hash: 'deadbeef' })

      expect(txt(m.view.container)).not.toMatch(/✓ Réponse reçue/)
      const n = sent(m.send, 'ARDOISE_FINAL').length
      await act(async () => { vi.advanceTimersByTime(2100) })
      expect(sent(m.send, 'ARDOISE_FINAL').length).toBeGreaterThan(n)
    })

    it('texte vide accusé : « ✓ Reçu — aucune réponse écrite » et « Aucune réponse » sur le clavier', async () => {
      const m = mountAck()
      await act(async () => { vi.advanceTimersByTime(100) })
      await m.setPhase('STOPPED')
      await m.ack({ questionId: 'q42', hash: '811c9dc5' })

      const t = txt(m.view.container)
      expect(t).toMatch(/✓ Reçu — aucune réponse écrite/)
      expect(t).toMatch(/Aucune réponse/)
    })

    it('PAUSE + accusé : « ⏸ Jeu en pause · ✓ réponse reçue », sans invitation à continuer', async () => {
      const m = mountAck()
      await act(async () => { vi.advanceTimersByTime(100) })
      await typeAll(m, 'PARIS')
      await m.setPhase('PAUSED')
      await m.ack({ questionId: 'q42', hash: '246f9d40' })

      const t = txt(m.view.container)
      expect(t).toMatch(/Jeu en pause · ✓ réponse reçue/)
      expect(t).not.toMatch(/continuer|vous pourrez/i)
    })

    it('CONTINUE (retour à STARTED) : l’accusé précédent n’est plus affiché, la saisie reprend', async () => {
      const m = mountAck()
      await act(async () => { vi.advanceTimersByTime(100) })
      await typeAll(m, 'PA')
      await m.setPhase('PAUSED')
      await m.ack({ questionId: 'q42', hash: '1e1c1e1c' })
      await m.setPhase('STARTED')

      expect(txt(m.view.container)).not.toMatch(/Réponse reçue|réponse reçue/)
      await act(async () => { fireEvent.click(clickKey(m.view.container, 'R')) })
      expect(m.view.container.querySelector('.ardoise-keyboard-text').textContent).toBe('PAR')
    })
  })
})
