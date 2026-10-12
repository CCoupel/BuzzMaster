import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, act, fireEvent } from '@testing-library/react'
import VPlayerPage from './VPlayerPage'
import { useGame } from '../hooks/GameContext'
import { createStorage, seedSession, makeGame, BOUND, clickKey } from './vplayer246Helpers'

// #246 (v11.2) — scénario REPRODUIT sur v11.1.0 (planner-20261010-203000, harnais
// harness246_clear_retype_stop) : effacement (⌫/✕) → "" envoyé aussitôt ; puis RESSAISIE continue sans
// pause de 200 ms jusqu'au STOP → le debounce pur ne part jamais pendant la frappe : serveur, admin et TV
// restent VIDES pendant tout le chrono alors que la tablette affiche le texte.
// Spécification : envoi EN DIRECT pendant la frappe continue (au plus toutes les ~200 ms, leading +
// trailing), SEQ strictement croissant, contrat ARDOISE_INPUT inchangé.
// Statut regression / critical.

const mockNavigate = vi.hoisted(() => vi.fn())
vi.mock('react-router-dom', () => ({ useNavigate: () => mockNavigate }))
vi.mock('../hooks/GameContext', () => ({ useGame: vi.fn() }))
vi.mock('./VPlayerPage.css', () => ({}))
vi.mock('nosleep.js', () => ({ default: class { enable() { return Promise.resolve() } disable() {} } }))
vi.mock('./PlayerDisplay', () => ({ default: vi.fn(() => null) }))

describe('VPlayerPage — effacement puis ressaisie continue jusqu’au STOP (#246)', () => {
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
    const calls = [] // { at, action, payload }
    const send = vi.fn((action, payload) => { calls.push({ at: Date.now(), action, payload }); return true })
    let phase = 'STARTED', key = '', cur = null
    useGame.mockImplementation(() => {
      if (key !== phase) {
        cur = makeGame(send, { playerConnectStatus: BOUND(), gameState: { phase, question: { ID: 'q7', TYPE: 'ARDOISE', ARDOISE_KEYBOARD_TYPE: 'AZERTY' } } })
        key = phase
      }
      return cur
    })
    const view = render(<VPlayerPage />)
    const press = async (label, wait) => {
      await act(async () => { fireEvent.click(clickKey(view.container, label)) })
      await act(async () => { vi.advanceTimersByTime(wait) })
    }
    return { calls, view, press, setPhase: async (p) => { phase = p; await act(async () => { view.rerender(<VPlayerPage />) }) } }
  }
  const inputs = (calls) => calls.filter((c) => c.action === 'ARDOISE_INPUT')

  // Frappe posée « PARIS », effacement jusqu'au vide, pause (le "" part), puis ressaisie CONTINUE.
  async function clearThenRetype(m, word, gapMs) {
    for (const c of 'PARIS') await m.press(c, 300)
    for (let i = 0; i < 5; i++) await m.press('⌫', 100)
    await act(async () => { vi.advanceTimersByTime(300) })
    const retypeStart = Date.now()
    for (const c of word) await m.press(c, gapMs)
    return retypeStart
  }

  it('Critical — pendant une ressaisie continue de 3 s sans pause ≥ 200 ms, des ARDOISE_INPUT partent au plus toutes les 200 ms (le serveur n’est jamais vide)', async () => {
    const m = mount()
    await act(async () => { vi.advanceTimersByTime(300) })
    const word = 'LYONNAISEMENTVOTREEEEEEEE' // 25 caractères à 120 ms ≈ 3 s
    const t0 = await clearThenRetype(m, word, 120)

    const during = inputs(m.calls).filter((c) => c.at >= t0)
    expect(during.length, `envois pendant la ressaisie : ${JSON.stringify(during.map((c) => [c.at - t0, c.payload.TEXT]))}`).toBeGreaterThanOrEqual(10)
    // aucun trou > ~250 ms entre deux envois consécutifs (200 ms + tolérance du pas de 120 ms)
    let maxGap = 0
    for (let i = 1; i < during.length; i++) maxGap = Math.max(maxGap, during[i].at - during[i - 1].at)
    expect(maxGap, 'plus long intervalle entre deux ARDOISE_INPUT pendant la frappe').toBeLessThanOrEqual(250)
    // le serveur voit du texte non vide dès les premières secondes de la ressaisie
    const firstNonEmpty = during.find((c) => c.payload.TEXT !== '')
    expect(firstNonEmpty, 'au moins un envoi non vide pendant la ressaisie').toBeTruthy()
    expect(firstNonEmpty.at - t0).toBeLessThanOrEqual(400)
    // aucun envoi vide après le début de la ressaisie, textes qui ne reculent jamais
    const texts = during.map((c) => c.payload.TEXT)
    expect(texts.filter((t) => t === '')).toEqual([])
    for (let i = 1; i < texts.length; i++) expect(texts[i].length).toBeGreaterThanOrEqual(texts[i - 1].length)
    // SEQ strictement croissant
    const seqs = inputs(m.calls).map((c) => c.payload.SEQ)
    for (let i = 1; i < seqs.length; i++) expect(seqs[i]).toBeGreaterThan(seqs[i - 1])
  })

  it('Critical — STOP 250 ms après la dernière touche : le dernier ARDOISE_INPUT avant STOP contient le texte courant', async () => {
    const m = mount()
    await act(async () => { vi.advanceTimersByTime(300) })
    await clearThenRetype(m, 'LYON', 120)
    await act(async () => { vi.advanceTimersByTime(250) })

    await m.setPhase('STOPPED')

    expect(inputs(m.calls).pop().payload.TEXT).toBe('LYON')
  })

  it('STOP immédiatement après la dernière touche : l’envoi en direct a au plus une fenêtre de retard, et ARDOISE_FINAL porte le texte complet', async () => {
    const m = mount()
    await act(async () => { vi.advanceTimersByTime(300) })
    await clearThenRetype(m, 'LYONNAIS', 120)

    await m.setPhase('STOPPED')
    await act(async () => { vi.advanceTimersByTime(200) })

    const last = inputs(m.calls).pop().payload.TEXT
    expect(last.length, `dernier envoi en direct : ${last}`).toBeGreaterThanOrEqual('LYONNAIS'.length - 2)
    expect(m.calls.filter((c) => c.action === 'ARDOISE_FINAL')[0].payload.TEXT).toBe('LYONNAIS')
  })

  it('le « "" » de l’effacement part encore tout de suite (non-régression #246 ARDOISE_CLEARED)', async () => {
    const m = mount()
    await act(async () => { vi.advanceTimersByTime(300) })
    await m.press('A', 300)
    await act(async () => { fireEvent.click(m.view.container.querySelector('.ardoise-keyboard-clear')) })
    await act(async () => { vi.advanceTimersByTime(50) })

    expect(inputs(m.calls).pop().payload).toMatchObject({ TEXT: '', REASON: 'clear' })
  })
})
