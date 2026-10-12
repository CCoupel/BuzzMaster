import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, act, fireEvent } from '@testing-library/react'
import VPlayerPage from './VPlayerPage'
import { useGame } from '../hooks/GameContext'
import { createStorage, seedSession, makeGame, BOUND, clickKey, sent } from './vplayer246Helpers'

// #246 (v11.2) — rechargement de la page (pull-to-refresh) en cours de question : le texte en cours est
// conservé dans le sessionStorage de l'ONGLET (clé = bumper + question) et restauré au remontage pour la
// MÊME question ; sinon, un ARDOISE_FINAL "" écraserait la réponse que le serveur détient encore.
// Boîte noire : le format de la clé n'est pas figé, on remonte la page avec le même sessionStorage.

const mockNavigate = vi.hoisted(() => vi.fn())
vi.mock('react-router-dom', () => ({ useNavigate: () => mockNavigate }))
vi.mock('../hooks/GameContext', () => ({ useGame: vi.fn() }))
vi.mock('./VPlayerPage.css', () => ({}))
vi.mock('nosleep.js', () => ({ default: class { enable() { return Promise.resolve() } disable() {} } }))
vi.mock('./PlayerDisplay', () => ({ default: vi.fn(() => null) }))

const fnvHex = (str) => {
  let h = 0x811c9dc5
  for (const b of new TextEncoder().encode(str)) { h ^= b; h = Math.imul(h, 0x01000193) >>> 0 }
  return h.toString(16).padStart(8, '0')
}
const shown = (view) => view.container.querySelector('.ardoise-keyboard-text')?.textContent ?? ''

describe('VPlayerPage — restauration du texte ARDOISE après rechargement (#246)', () => {
  let ls, ss
  beforeEach(() => {
    vi.useFakeTimers()
    ls = createStorage()
    ss = createStorage()
    vi.stubGlobal('localStorage', ls)
    vi.stubGlobal('sessionStorage', ss)
    useGame.mockReset()
    seedSession(ls)
  })
  afterEach(() => { vi.useRealTimers(); vi.unstubAllGlobals() })

  // Monte la page ; renvoie les contrôles. `q` et `phase` sont modifiables.
  function mount({ q = 'q7', phase = 'STARTED' } = {}) {
    const send = vi.fn(() => true)
    const st = { q, phase }
    let key = '', cur = null
    useGame.mockImplementation(() => {
      const k = `${st.q}|${st.phase}`
      if (k !== key) {
        cur = makeGame(send, { playerConnectStatus: BOUND(), gameState: { phase: st.phase, question: { ID: st.q, TYPE: 'ARDOISE', ARDOISE_KEYBOARD_TYPE: 'AZERTY' } } })
        key = k
      }
      return cur
    })
    const view = render(<VPlayerPage />)
    return {
      send, view, st,
      type: async (s) => { for (const ch of s) { await act(async () => { fireEvent.click(clickKey(view.container, ch)) }); await act(async () => { vi.advanceTimersByTime(300) }) } },
      go: async (patch) => { Object.assign(st, patch); await act(async () => { view.rerender(<VPlayerPage />) }) },
    }
  }

  it('Critical — rechargement en STARTED puis STOP sans frappe : le texte restauré est envoyé en ARDOISE_FINAL (jamais un final vide)', async () => {
    const first = mount()
    await act(async () => { vi.advanceTimersByTime(300) })
    await first.type('PARIS')
    first.view.unmount() // rechargement : la page repart de zéro, le sessionStorage de l'onglet survit

    const second = mount()
    await act(async () => { vi.advanceTimersByTime(300) })
    expect(shown(second.view)).toBe('PARIS')

    await second.go({ phase: 'STOPPED' })
    await act(async () => { vi.advanceTimersByTime(1500) })

    const finals = sent(second.send, 'ARDOISE_FINAL')
    expect(finals.length).toBeGreaterThanOrEqual(1)
    expect(finals.every((f) => f.TEXT === 'PARIS' && f.HASH === fnvHex('PARIS'))).toBe(true)
    expect(finals.filter((f) => f.TEXT === '')).toEqual([])
  })

  it('question différente : rien n’est restauré, et aucun final vide n’est envoyé au STOP', async () => {
    const first = mount({ q: 'q7' })
    await act(async () => { vi.advanceTimersByTime(300) })
    await first.type('PARIS')
    first.view.unmount()

    const second = mount({ q: 'q8' })
    await act(async () => { vi.advanceTimersByTime(300) })
    expect(shown(second.view)).not.toMatch(/PARIS/)

    await second.go({ phase: 'STOPPED' })
    await act(async () => { vi.advanceTimersByTime(1500) })
    expect(sent(second.send, 'ARDOISE_FINAL')).toEqual([]) // sans texte restauré ni saisie : jamais de final vide
  })

  it('le texte d’une question précédente n’est pas ressuscité quand on revient sur elle après avoir changé de question', async () => {
    const first = mount({ q: 'q7' })
    await act(async () => { vi.advanceTimersByTime(300) })
    await first.type('PARIS')
    await first.go({ q: 'q8' }) // changement de question : effacé
    await act(async () => { vi.advanceTimersByTime(300) })
    first.view.unmount()

    const second = mount({ q: 'q7' })
    await act(async () => { vi.advanceTimersByTime(300) })
    expect(shown(second.view)).not.toMatch(/PARIS/)
  })

  it('rechargement sans aucune frappe préalable : rien de restauré, pas de final vide au STOP', async () => {
    const m = mount()
    await act(async () => { vi.advanceTimersByTime(300) })
    await m.go({ phase: 'STOPPED' })
    await act(async () => { vi.advanceTimersByTime(1500) })
    expect(sent(m.send, 'ARDOISE_FINAL').filter((f) => f.TEXT === '')).toEqual([])
  })

  it('sessionStorage indisponible (exceptions) : aucune erreur, la saisie fonctionne et part normalement', async () => {
    const broken = {
      getItem: () => { throw new Error('SecurityError') },
      setItem: () => { throw new Error('QuotaExceededError') },
      removeItem: () => { throw new Error('SecurityError') },
      clear: () => {},
    }
    vi.stubGlobal('sessionStorage', broken)

    let m
    expect(() => { m = mount() }).not.toThrow()
    await act(async () => { vi.advanceTimersByTime(300) })
    await m.type('AB')

    expect(shown(m.view)).toBe('AB')
    expect(sent(m.send, 'ARDOISE_INPUT').pop().TEXT).toBe('AB')
    await m.go({ phase: 'STOPPED' })
    await act(async () => { vi.advanceTimersByTime(1500) })
    expect(sent(m.send, 'ARDOISE_FINAL')[0].TEXT).toBe('AB')
  })
})
