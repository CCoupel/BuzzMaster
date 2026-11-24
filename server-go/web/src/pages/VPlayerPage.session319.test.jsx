import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, act } from '@testing-library/react'
import VPlayerPage from './VPlayerPage'
import { useGame } from '../hooks/GameContext'
import { createStorage, seedSession, makeGame, sent, PAUL } from './vplayer246Helpers'

// #319 (v13.0.0, LOT-2.2) — tablette : jeton de session (contracts/vplayer-session.md §4.4, §5, §6). Écrit depuis
// le contrat : PLAYER_CONNECT envoie {NAME, ID, TOKEN} à chaque ouverture (jamais un jeton qu'on n'a pas) ; une session
// sans jeton (tablette ouverte avant la mise à jour) est ACCEPTÉE au montage (migration 1c) ; INVALID_SESSION renvoie
// vers l'inscription avec le texte du contrat, en purgeant la session — sans toucher à celle d'un autre onglet (#246).
// Scaffold : VPlayerPage.binding.246.test.jsx (aides vplayer246Helpers).

const mockNavigate = vi.hoisted(() => vi.fn())
vi.mock('react-router-dom', () => ({ useNavigate: () => mockNavigate }))
vi.mock('../hooks/GameContext', () => ({ useGame: vi.fn() }))
vi.mock('./VPlayerPage.css', () => ({}))
vi.mock('nosleep.js', () => ({ default: class { enable() { return Promise.resolve() } disable() {} } }))
vi.mock('./PlayerDisplay', () => ({ default: vi.fn(() => null) }))

const TOKEN = 'dG9rZW4tZGUtdGVzdC0zMi1vY3RldHMtYmFzZTY0dXJs'
const INVALID_SESSION_TEXT = 'Cette place est déjà liée à un autre appareil. Demandez à l’animateur de libérer la place, puis reconnectez-vous avec votre nom.'

describe('VPlayerPage — jeton de session (#319)', () => {
  let ls
  let ss
  beforeEach(() => {
    vi.useFakeTimers()
    mockNavigate.mockClear()
    ls = createStorage()
    ss = createStorage()
    vi.stubGlobal('localStorage', ls)
    vi.stubGlobal('sessionStorage', ss)
    useGame.mockReset()
  })
  afterEach(() => { vi.useRealTimers(); vi.unstubAllGlobals() })

  // Objet de jeu STABLE entre deux rendus (un nouvel objet à chaque rendu relance les effets de la page) ; il ne change
  // que par setGame / setStatus, comme VPlayerPage.binding.246.test.jsx.
  const mount = (over = {}) => {
    const send = vi.fn(() => true)
    let status = over.status ?? 'connected'
    let cur = makeGame(send, { ...over, status })
    useGame.mockImplementation(() => cur)
    const view = render(<VPlayerPage />)
    const rerender = async () => { await act(async () => { view.rerender(<VPlayerPage />) }) }
    return {
      send, view,
      setGame: async (o) => { cur = makeGame(send, { ...over, status, ...o }); await rerender() },
      setStatus: async (st) => { status = st; cur = makeGame(send, { ...over, status, playerConnectStatus: null }); await rerender() },
    }
  }

  it('Critical — PLAYER_CONNECT porte {NAME, ID, TOKEN} dès l\'ouverture quand un jeton est stocké', async () => {
    seedSession(ls)
    ls.setItem('vplayer_token', TOKEN)
    const m = mount()
    await act(async () => { vi.advanceTimersByTime(2100) })
    const first = sent(m.send, 'PLAYER_CONNECT')[0]
    expect(first).toEqual({ NAME: 'Paul', ID: PAUL, TOKEN: TOKEN })
  })

  it('le jeton est envoyé à CHAQUE ouverture de WebSocket (coupure puis reconnexion)', async () => {
    seedSession(ls)
    ls.setItem('vplayer_token', TOKEN)
    const m = mount()
    await act(async () => { vi.advanceTimersByTime(2100) })
    await m.setStatus('disconnected')
    await m.setStatus('connected')
    await act(async () => { vi.advanceTimersByTime(2100) })
    const all = sent(m.send, 'PLAYER_CONNECT')
    expect(all.length).toBeGreaterThanOrEqual(2)
    for (const p of all) expect(p.TOKEN).toBe(TOKEN)
  })

  it('migration — une session SANS jeton est acceptée au montage : pas de renvoi vers l\'inscription, PLAYER_CONNECT sans TOKEN', async () => {
    seedSession(ls) // id mémorisé, aucun vplayer_token (tablette ouverte avant la mise à jour)
    const m = mount()
    await act(async () => { vi.advanceTimersByTime(5000) })
    expect(mockNavigate).not.toHaveBeenCalled()
    expect(ls.getItem('vplayer_id')).toBe(PAUL) // session conservée
    const first = sent(m.send, 'PLAYER_CONNECT')[0]
    expect(first).toEqual({ NAME: 'Paul', ID: PAUL })
    expect('TOKEN' in first).toBe(false) // jamais de TOKEN vide ou « undefined »
  })

  it('le jeton reçu après une migration (cas 1c) est envoyé aux reconnexions suivantes', async () => {
    seedSession(ls)
    const m = mount()
    await act(async () => { vi.advanceTimersByTime(2100) })
    expect('TOKEN' in sent(m.send, 'PLAYER_CONNECT')[0]).toBe(false)
    ls.setItem('vplayer_token', TOKEN) // PLAYER_CONNECTED a livré le jeton (écrit par le hook)
    await m.setGame({ playerConnectStatus: { status: 'connected', id: PAUL, name: 'Paul' } })
    await m.setStatus('disconnected')
    await m.setStatus('connected')
    await act(async () => { vi.advanceTimersByTime(2100) })
    expect(sent(m.send, 'PLAYER_CONNECT').some(p => p.TOKEN === TOKEN)).toBe(true)
  })

  it('règle d\'onglet (#246) : le jeton d\'un AUTRE onglet n\'est jamais envoyé avec l\'ID de cet onglet', async () => {
    seedSession(ls)
    ls.setItem('vplayer_token', TOKEN)
    const m = mount()
    await act(async () => { vi.advanceTimersByTime(2100) })
    const before = sent(m.send, 'PLAYER_CONNECT').length
    ls.setItem('vplayer_id', 'vjoueur_Autre_1') // un autre onglet écrase le localStorage partagé
    ls.setItem('vplayer_token', 'jeton-de-l-autre-onglet')
    await m.setStatus('disconnected')
    await m.setStatus('connected')
    await act(async () => { vi.advanceTimersByTime(2100) })
    for (const p of sent(m.send, 'PLAYER_CONNECT').slice(before)) {
      expect(p.TOKEN, JSON.stringify(p)).not.toBe('jeton-de-l-autre-onglet')
    }
  })

  it('Critical — INVALID_SESSION : texte du contrat §4.4, session purgée (jeton compris), retour à l\'inscription', async () => {
    seedSession(ls)
    ls.setItem('vplayer_token', TOKEN)
    const m = mount()
    await act(async () => { vi.advanceTimersByTime(100) })
    await m.setGame({ playerConnectStatus: { status: 'rejected', reason: 'INVALID_SESSION' } })

    expect(m.view.container.textContent).toContain(INVALID_SESSION_TEXT)
    await act(async () => { vi.advanceTimersByTime(10000) })
    expect(mockNavigate).toHaveBeenCalledWith('/')
    for (const k of ['vplayer_name', 'vplayer_session', 'vplayer_id', 'vplayer_token']) expect(ls.getItem(k), k).toBeNull()
    expect(ss.getItem('vplayer_redirect_reason')).toBe('INVALID_SESSION') // l'inscription affiche le motif
  })

  it('INVALID_SESSION dans un onglet périmé : la session (et le jeton) d\'un autre onglet n\'est pas purgée (#246)', async () => {
    seedSession(ls)
    ls.setItem('vplayer_token', TOKEN)
    const m = mount()
    await act(async () => { vi.advanceTimersByTime(100) })
    ls.setItem('vplayer_id', 'vjoueur_Autre_2') // un autre onglet a enregistré SA session
    ls.setItem('vplayer_token', 'jeton-autre-onglet')
    await m.setGame({ playerConnectStatus: { status: 'rejected', reason: 'INVALID_SESSION' } })
    await act(async () => { vi.advanceTimersByTime(10000) })
    expect(ls.getItem('vplayer_id')).toBe('vjoueur_Autre_2')
    expect(ls.getItem('vplayer_token')).toBe('jeton-autre-onglet')
  })

  it('INVALID_SESSION n\'est pas traité comme un rejet d\'inscription (NAME_TAKEN & co) : c\'est un motif de redirection', async () => {
    const { REDIRECT_MESSAGES, REJECTION_MESSAGES } = await import('../utils/playerConnectMessages')
    expect(REDIRECT_MESSAGES.INVALID_SESSION).toBe(INVALID_SESSION_TEXT)
    expect(REJECTION_MESSAGES.INVALID_SESSION).toBeUndefined()
  })
})
