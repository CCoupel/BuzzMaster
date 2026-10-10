import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, act, fireEvent } from '@testing-library/react'
import VPlayerPage from './VPlayerPage'
import { useGame } from '../hooks/GameContext'
import { createStorage, seedSession, makeGame, BOUND, sent, clickKey, PAUL } from './vplayer246Helpers'

// #246 (v11.2) — liaison connexion ↔ bumper côté tablette, SEQ/REASON d'ARDOISE_INPUT.
// Contrats : contracts/websocket-actions.md §PLAYER_CONNECT (« Liaison connexion ↔ bumper »),
// §ARDOISE_INPUT (SEQ, REASON). Reproduction harness246_name_binding INVERSÉE (statut `regression`) :
// avant, une tablette sans ID mémorisé se liait au premier bumper de même NOM et écrivait dans
// l'équipe d'un homonyme.

const mockNavigate = vi.hoisted(() => vi.fn())
vi.mock('react-router-dom', () => ({ useNavigate: () => mockNavigate }))
vi.mock('../hooks/GameContext', () => ({ useGame: vi.fn() }))
vi.mock('./VPlayerPage.css', () => ({}))
vi.mock('nosleep.js', () => ({ default: class { enable() { return Promise.resolve() } disable() {} } }))
vi.mock('./PlayerDisplay', () => ({ default: vi.fn(() => null) }))

describe('VPlayerPage — liaison à chaque ouverture de WebSocket (#246)', () => {
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

  function mount(initialStatus = 'connected', extra = {}) {
    const send = vi.fn(() => true)
    let status = initialStatus
    let cur = makeGame(send, { status, ...extra })
    let curStatus = status
    useGame.mockImplementation(() => {
      // une nouvelle connexion repart sans PLAYER_CONNECTED (le statut est consommé puis effacé)
      if (curStatus !== status) { cur = makeGame(send, { status, ...extra, playerConnectStatus: null }); curStatus = status }
      return cur
    })
    const view = render(<VPlayerPage />)
    return {
      send, view,
      setStatus: async (s) => { status = s; await act(async () => { view.rerender(<VPlayerPage />) }) },
      setGame: async (over) => { cur = makeGame(send, { status, ...extra, ...over }); curStatus = status; await act(async () => { view.rerender(<VPlayerPage />) }) },
    }
  }

  it('Critical — envoie PLAYER_CONNECT {NAME, ID} dès l’ouverture, même si le bumper est déjà CONNECTED dans l’état reçu', async () => {
    const m = mount('connected') // bumpers[PAUL].CONNECTED === true

    await act(async () => { vi.advanceTimersByTime(2100) })

    expect(sent(m.send, 'PLAYER_CONNECT')[0]).toEqual({ NAME: 'Paul', ID: PAUL })
  })

  it('renvoie PLAYER_CONNECT à CHAQUE nouvelle ouverture de WebSocket (coupure puis reconnexion)', async () => {
    const m = mount('connected')
    await act(async () => { vi.advanceTimersByTime(2100) })
    const first = sent(m.send, 'PLAYER_CONNECT').length
    expect(first).toBeGreaterThanOrEqual(1)

    await m.setStatus('disconnected')
    await m.setStatus('connected')
    await act(async () => { vi.advanceTimersByTime(2100) })

    expect(sent(m.send, 'PLAYER_CONNECT').length).toBeGreaterThan(first)
  })

  it('utilise l’ID de la session en mémoire, jamais le localStorage courant', async () => {
    const m = mount('disconnected')
    ls.setItem('vplayer_name', 'Paul') // inchangé : même onglet, pas de stale
    await m.setStatus('connected')
    await act(async () => { vi.advanceTimersByTime(2100) })

    for (const p of sent(m.send, 'PLAYER_CONNECT')) expect(p.ID).toBe(PAUL)
  })

  it('met les frappes en attente jusqu’au PLAYER_CONNECTED de la connexion courante, puis envoie le texte complet', async () => {
    const m = mount('connected')
    await act(async () => { vi.advanceTimersByTime(100) })
    for (const ch of 'PARIS') await act(async () => { fireEvent.click(clickKey(m.view.container, ch)) })
    await act(async () => { vi.advanceTimersByTime(1000) })
    expect(sent(m.send, 'ARDOISE_INPUT')).toEqual([]) // connexion non liée : rien ne part

    await m.setGame({ playerConnectStatus: BOUND() }) // PLAYER_CONNECTED reçu
    await act(async () => { vi.advanceTimersByTime(500) })

    const texts = sent(m.send, 'ARDOISE_INPUT')
    expect(texts.length).toBeGreaterThanOrEqual(1)
    expect(texts[texts.length - 1]).toMatchObject({ TEXT: 'PARIS', ID: PAUL })
    expect(texts.map((p) => p.REASON)).toContain('resend')
  })

  it('une nouvelle connexion non encore liée remet les actions en attente (la liaison est propre à la connexion)', async () => {
    const m = mount('connected', { playerConnectStatus: BOUND() })
    await act(async () => { vi.advanceTimersByTime(100) })
    await m.setStatus('disconnected')
    await m.setStatus('connected') // nouvelle connexion, pas encore de PLAYER_CONNECTED
    m.send.mockClear()

    await act(async () => { fireEvent.click(clickKey(m.view.container, 'A')) })
    await act(async () => { vi.advanceTimersByTime(300) })

    expect(sent(m.send, 'ARDOISE_INPUT')).toEqual([])
  })

  it('Critical — pas de liaison par nom : sans ID mémorisé, aucune action n’est envoyée avec l’ID d’un homonyme', async () => {
    ls.removeItem('vplayer_id') // tablette sans ID (ex. après un PLAYER_EVICTED)
    ls.setItem('vplayer_name', 'Mamie')
    const ALICE = 'vjoueur_Alice_20261006_140000', MAMIE = 'vjoueur_Mamie_20261006_140500'
    const bumpers = {
      [ALICE]: { NAME: 'Mamie', IS_VIRTUAL: true, IS_VPLAYER: true, TEAM: 'Les Bleus', CONNECTED: true },
      [MAMIE]: { NAME: 'Mamie', IS_VIRTUAL: true, IS_VPLAYER: true, TEAM: 'Les Rouges', CONNECTED: true },
    }
    const send = vi.fn(() => true)
    const game = makeGame(send, { bumpers })
    useGame.mockImplementation(() => game)
    render(<VPlayerPage />)

    await act(async () => { vi.advanceTimersByTime(10000) })

    expect(sent(send, 'ARDOISE_INPUT')).toEqual([])
    expect(sent(send, 'PLAYER_CONNECT').filter((p) => p.ID)).toEqual([])
    expect(mockNavigate).toHaveBeenCalled() // retour à l'inscription / demande de place (#122)
  })
})

describe('VPlayerPage — SEQ et REASON d’ARDOISE_INPUT (#246)', () => {
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

  function mountBound(questionId = 'q1') {
    const send = vi.fn(() => true)
    let q = questionId, status = 'connected'
    let cur = null, key = ''
    useGame.mockImplementation(() => {
      const k = `${q}|${status}`
      if (k !== key) {
        cur = makeGame(send, { status, playerConnectStatus: BOUND(), gameState: { question: { ID: q, TYPE: 'ARDOISE', ARDOISE_KEYBOARD_TYPE: 'AZERTY' } } })
        key = k
      }
      return cur
    })
    const view = render(<VPlayerPage />)
    return {
      send, view,
      rerender: async () => { await act(async () => { view.rerender(<VPlayerPage />) }) },
      setQ: (v) => { q = v }, setStatus: (v) => { status = v },
    }
  }
  const press = async (m, label) => { await act(async () => { fireEvent.click(clickKey(m.view.container, label)) }) }

  it('SEQ est croissant (1, 2, 3…) et REASON vaut « key » pour une frappe', async () => {
    const m = mountBound()
    await act(async () => { vi.advanceTimersByTime(100) })
    for (const ch of 'ABC') { await press(m, ch); await act(async () => { vi.advanceTimersByTime(300) }) }

    const inputs = sent(m.send, 'ARDOISE_INPUT')
    expect(inputs.length).toBeGreaterThanOrEqual(2)
    const seqs = inputs.map((p) => p.SEQ)
    expect(seqs[0]).toBe(1)
    for (let i = 1; i < seqs.length; i++) expect(seqs[i]).toBeGreaterThan(seqs[i - 1])
    expect(inputs.find((p) => p.TEXT === 'A').REASON).toBe('key')
  })

  it('✕ « Effacer tout » envoie REASON « clear » avec TEXT vide', async () => {
    const m = mountBound()
    await act(async () => { vi.advanceTimersByTime(100) })
    await press(m, 'A'); await press(m, 'B')
    await act(async () => { vi.advanceTimersByTime(300) })

    await act(async () => { fireEvent.click(m.view.container.querySelector('.ardoise-keyboard-clear')) })
    await act(async () => { vi.advanceTimersByTime(300) })

    const last = sent(m.send, 'ARDOISE_INPUT').pop()
    expect(last).toMatchObject({ TEXT: '', REASON: 'clear' })
  })

  it('⌫ jusqu’au vide envoie REASON « backspace_empty »', async () => {
    const m = mountBound()
    await act(async () => { vi.advanceTimersByTime(100) })
    await press(m, 'A')
    await act(async () => { vi.advanceTimersByTime(300) })

    await press(m, '⌫')
    await act(async () => { vi.advanceTimersByTime(300) })

    const last = sent(m.send, 'ARDOISE_INPUT').pop()
    expect(last).toMatchObject({ TEXT: '', REASON: 'backspace_empty' })
  })

  it('SEQ repart à 1 pour une nouvelle question et pour une nouvelle connexion', async () => {
    const m = mountBound('q1')
    await act(async () => { vi.advanceTimersByTime(100) })
    await press(m, 'A'); await act(async () => { vi.advanceTimersByTime(300) })
    await press(m, 'B'); await act(async () => { vi.advanceTimersByTime(300) })
    expect(Math.max(...sent(m.send, 'ARDOISE_INPUT').map((p) => p.SEQ))).toBeGreaterThanOrEqual(2)

    // nouvelle question : le premier envoi de cette question porte SEQ = 1
    m.setQ('q2'); await m.rerender()
    await act(async () => { vi.advanceTimersByTime(100) })
    const nQ2 = sent(m.send, 'ARDOISE_INPUT').length
    await press(m, 'C'); await act(async () => { vi.advanceTimersByTime(300) })
    expect(sent(m.send, 'ARDOISE_INPUT')[nQ2].SEQ).toBe(1)

    // nouvelle connexion (liée) : le premier envoi de cette connexion porte SEQ = 1
    const nBefore = sent(m.send, 'ARDOISE_INPUT').length
    m.setStatus('disconnected'); await m.rerender()
    m.setStatus('connected'); await m.rerender()
    await press(m, 'D'); await act(async () => { vi.advanceTimersByTime(300) })
    const afterReconnect = sent(m.send, 'ARDOISE_INPUT').slice(nBefore)
    expect(afterReconnect.length).toBeGreaterThanOrEqual(1)
    expect(afterReconnect[0].SEQ).toBe(1)
  })
})
