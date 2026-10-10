import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, act, fireEvent } from '@testing-library/react'
import VPlayerPage from './VPlayerPage'
import { useGame } from '../hooks/GameContext'
import { createStorage, seedSession, makeGame, BOUND, sent, traceKinds, PAUL, OLD } from './vplayer246Helpers'

// #246 (v11.2) — onglet périmé et session reprise ailleurs.
// Contrat : contracts/websocket-actions.md §PLAYER_EVICTED (« Règle générale v11.2 »,
// DUPLICATE_SESSION). Reproduction harness246_stale_tab INVERSÉE (statut `regression`) :
// avant, l'onglet B (session d'une ancienne partie) renvoyait PLAYER_CONNECT {NAME: "Mamie", ID: P}
// et renommait le joueur actuel, puis effaçait la session partagée.

const mockNavigate = vi.hoisted(() => vi.fn())
vi.mock('react-router-dom', () => ({ useNavigate: () => mockNavigate }))
vi.mock('../hooks/GameContext', () => ({ useGame: vi.fn() }))
vi.mock('./VPlayerPage.css', () => ({}))
vi.mock('nosleep.js', () => ({ default: class { enable() { return Promise.resolve() } disable() {} } }))
vi.mock('./PlayerDisplay', () => ({ default: vi.fn(() => null) }))

describe('VPlayerPage — onglet périmé (#246)', () => {
  let ls
  beforeEach(() => {
    vi.useFakeTimers()
    mockNavigate.mockClear()
    ls = createStorage()
    vi.stubGlobal('localStorage', ls)
    vi.stubGlobal('sessionStorage', createStorage())
    useGame.mockReset()
  })
  afterEach(() => { vi.useRealTimers(); vi.unstubAllGlobals() })

  // Mise en place du scénario du harnais : l'onglet B monte avec l'ANCIENNE session ; ensuite
  // l'onglet A de la même tablette écrit sa session (Paul) dans le localStorage partagé.
  async function mountStaleTab({ status0 = 'disconnected' } = {}) {
    seedSession(ls, { name: 'Mamie', id: OLD, session: '1' })
    const send = vi.fn(() => true)
    const trace = vi.fn()
    let status = status0
    let cur = makeGame(send, { status, id: PAUL, sendClientTrace: trace })
    let curStatus = status
    useGame.mockImplementation(() => {
      if (curStatus !== status) { cur = makeGame(send, { status, id: PAUL, sendClientTrace: trace }); curStatus = status }
      return cur
    })
    const view = render(<VPlayerPage />)
    seedSession(ls, { name: 'Paul', id: PAUL, session: '2' }) // l'onglet A s'est enrôlé entre-temps
    return {
      send, trace, view,
      setStatus: async (s) => { status = s; await act(async () => { view.rerender(<VPlayerPage />) }) },
    }
  }

  it('Critical — à la reconnexion, n’envoie aucun PLAYER_CONNECT ni aucune action et ne touche pas au localStorage partagé', async () => {
    const t = await mountStaleTab()

    await t.setStatus('connected')
    await act(async () => { vi.advanceTimersByTime(2100) })
    for (let i = 0; i < 12; i++) await act(async () => { vi.advanceTimersByTime(3000) }) // ≈ 36 s (le harnais observait l'effacement à ~46 s)

    expect(sent(t.send, 'PLAYER_CONNECT')).toEqual([])
    expect(sent(t.send, 'ARDOISE_INPUT')).toEqual([])
    expect(ls.dump()).toEqual({ vplayer_name: 'Paul', vplayer_session: '2', vplayer_id: PAUL })
    expect(mockNavigate).not.toHaveBeenCalled()
  })

  it('affiche l’écran « Session ouverte dans un autre onglet » et n’émet qu’une trace STALE_TAB', async () => {
    const t = await mountStaleTab()

    await t.setStatus('connected')
    await act(async () => { vi.advanceTimersByTime(2100) })

    expect(t.view.container.textContent).toMatch(/autre onglet/i)
    const kinds = traceKinds(t.send, t.trace)
    expect(kinds.filter((k) => k === 'STALE_TAB').length).toBe(1)
    const other = t.send.mock.calls.filter((c) => !['HELLO', 'CLIENT_TRACE'].includes(c[0]))
    expect(other).toEqual([])
  })

  it('une réponse PLAYER_EVICTED reçue par un onglet périmé n’efface pas la session de l’onglet actif', async () => {
    const t = await mountStaleTab()
    await t.setStatus('connected')
    const evicted = makeGame(t.send, { status: 'connected', playerEvictedStatus: { reason: 'PLAYER_REMOVED' }, sendClientTrace: t.trace })
    useGame.mockImplementation(() => evicted)

    await act(async () => { t.view.rerender(<VPlayerPage />) })
    await act(async () => { vi.advanceTimersByTime(5000) })

    expect(ls.dump()).toEqual({ vplayer_name: 'Paul', vplayer_session: '2', vplayer_id: PAUL })
  })

  it('sur l’événement `storage` d’un autre onglet : un onglet actif devient périmé et cesse d’émettre', async () => {
    seedSession(ls, { name: 'Paul', id: PAUL })
    const send = vi.fn(() => true)
    let game = makeGame(send, { playerConnectStatus: BOUND() })
    useGame.mockImplementation(() => game)
    const view = render(<VPlayerPage />)
    await act(async () => { vi.advanceTimersByTime(100) })
    const before = sent(send, 'PLAYER_CONNECT').length

    seedSession(ls, { name: 'Mamie', id: OLD, session: '9' }) // un autre onglet de la tablette s'enrôle
    await act(async () => { window.dispatchEvent(new StorageEvent('storage', { key: 'vplayer_id', newValue: OLD })) })
    await act(async () => { vi.advanceTimersByTime(5000) })

    expect(view.container.textContent).toMatch(/autre onglet/i)
    expect(sent(send, 'PLAYER_CONNECT').length).toBe(before)
    expect(sent(send, 'ARDOISE_INPUT')).toEqual([])
    expect(ls.dump().vplayer_id).toBe(OLD)
  })

  it('non-régression — l’onglet qui possède la session reçoit PLAYER_REMOVED : efface SA session et retourne à l’inscription', async () => {
    seedSession(ls, { name: 'Paul', id: PAUL })
    const send = vi.fn(() => true)
    useGame.mockImplementation(() => makeGame(send, { playerEvictedStatus: { reason: 'PLAYER_REMOVED' } }))
    render(<VPlayerPage />)

    await act(async () => { vi.advanceTimersByTime(5000) })

    expect(ls.getItem('vplayer_id')).toBeNull()
    expect(ls.getItem('vplayer_name')).toBeNull()
    expect(mockNavigate).toHaveBeenCalled()
  })
})

describe('VPlayerPage — DUPLICATE_SESSION (#246)', () => {
  let ls
  beforeEach(() => {
    vi.useFakeTimers()
    mockNavigate.mockClear()
    ls = createStorage()
    vi.stubGlobal('localStorage', ls)
    vi.stubGlobal('sessionStorage', createStorage())
    useGame.mockReset()
    seedSession(ls)
  })
  afterEach(() => { vi.useRealTimers(); vi.unstubAllGlobals() })

  function mountEvicted() {
    const send = vi.fn(() => true)
    const trace = vi.fn()
    let game = makeGame(send, { playerConnectStatus: BOUND(), sendClientTrace: trace })
    useGame.mockImplementation(() => game)
    const view = render(<VPlayerPage />)
    return { send, trace, view, evict: async () => {
      game = makeGame(send, { playerEvictedStatus: { reason: 'DUPLICATE_SESSION' }, sendClientTrace: trace })
      await act(async () => { view.rerender(<VPlayerPage />) })
    } }
  }

  it('Critical — n’efface rien, ne retourne pas à l’inscription, cesse d’émettre et affiche « Reprendre ici »', async () => {
    const m = mountEvicted()
    await act(async () => { vi.advanceTimersByTime(100) })

    await m.evict()
    await act(async () => { vi.advanceTimersByTime(6000) })

    expect(ls.dump()).toEqual({ vplayer_name: 'Paul', vplayer_session: '1', vplayer_id: PAUL })
    expect(mockNavigate).not.toHaveBeenCalled()
    expect(m.view.container.textContent).toMatch(/ouverte ailleurs/i)
    expect(m.view.container.textContent).toMatch(/Reprendre ici/i)
    const connects = sent(m.send, 'PLAYER_CONNECT').length
    await act(async () => { vi.advanceTimersByTime(20000) })
    expect(sent(m.send, 'PLAYER_CONNECT').length).toBe(connects) // pas de reprise automatique
    expect(traceKinds(m.send, m.trace)).toContain('DUPLICATE_SESSION')
  })

  it('« Reprendre ici » renvoie PLAYER_CONNECT {NAME, ID} avec la session de cet onglet', async () => {
    const m = mountEvicted()
    await m.evict()
    m.send.mockClear()

    const btn = [...m.view.container.querySelectorAll('button')].find((b) => /Reprendre ici/i.test(b.textContent))
    expect(btn).toBeTruthy()
    await act(async () => { fireEvent.click(btn) })
    await act(async () => { vi.advanceTimersByTime(100) })

    expect(sent(m.send, 'PLAYER_CONNECT')).toContainEqual({ NAME: 'Paul', ID: PAUL })
  })
})
