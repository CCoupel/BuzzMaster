import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import EnrollPage from './EnrollPage'
import { useGame } from '../hooks/GameContext'

// #319 — enrôlement avec un ID périmé dans le stockage : la session périmée est purgée avant l'envoi de { NAME },
// pour que PLAYER_CONNECTED écrive la nouvelle paire {ID, jeton} ; un ID vivant (autre onglet) n'est jamais purgé.

function ls() {
  let store = {}
  return {
    getItem: (k) => (Object.prototype.hasOwnProperty.call(store, k) ? store[k] : null),
    setItem: (k, v) => { store[k] = String(v) },
    removeItem: (k) => { delete store[k] },
    clear: () => { store = {} },
  }
}
const mockNavigate = vi.hoisted(() => vi.fn())
vi.mock('react-router-dom', () => ({ useNavigate: () => mockNavigate }))
vi.mock('../hooks/GameContext', () => ({ useGame: vi.fn() }))
vi.mock('./EnrollPage.css', () => ({}))

const mk = (overrides = {}) => ({
  connectVirtualPlayer: vi.fn(), clearPlayerConnectStatus: vi.fn(), playerConnectStatus: null, status: 'connected',
  bumpers: {}, gameState: { enrollmentActive: true, virtualPlayerCount: 0, virtualPlayerLimit: 20 }, ...overrides,
})
async function submit(name) {
  await waitFor(() => expect(screen.getByLabelText('Choisis ton pseudo')).toBeInTheDocument())
  fireEvent.change(screen.getByLabelText('Choisis ton pseudo'), { target: { value: name } })
  fireEvent.click(screen.getByRole('button', { name: /Rejoindre la partie/i }))
}

describe('EnrollPage — purge de la session périmée avant l\'enrôlement (#319)', () => {
  beforeEach(() => { mockNavigate.mockClear(); vi.stubGlobal('localStorage', ls()); useGame.mockReset() })
  afterEach(() => vi.unstubAllGlobals())

  it('ID périmé (bumper absent) : ID et jeton anciens sont purgés avant PLAYER_CONNECT { NAME }', async () => {
    localStorage.setItem('vplayer_id', 'vieux'); localStorage.setItem('vplayer_token', 'vieux-jeton')
    const g = mk(); useGame.mockReturnValue(g)
    let atSend
    g.connectVirtualPlayer.mockImplementation(() => { atSend = [localStorage.getItem('vplayer_id'), localStorage.getItem('vplayer_token')] })
    render(<EnrollPage />)
    await submit('Alice')
    expect(g.connectVirtualPlayer).toHaveBeenCalledWith('Alice')
    expect(atSend).toEqual([null, null])
  })
  it('ID dont le bumper existe (autre onglet vivant) : rien n\'est purgé', async () => {
    localStorage.setItem('vplayer_id', 'vivant'); localStorage.setItem('vplayer_token', 'jeton-vivant')
    const g = mk({ bumpers: { vivant: { IS_VIRTUAL: true } } }); useGame.mockReturnValue(g)
    render(<EnrollPage />)
    // bumper existant + pas de nom enregistré : le formulaire reste accessible
    await submit('Bob')
    expect(localStorage.getItem('vplayer_id')).toBe('vivant')
    expect(localStorage.getItem('vplayer_token')).toBe('jeton-vivant')
    expect(g.connectVirtualPlayer).toHaveBeenCalledWith('Bob')
  })
  it('stockage vide : aucune purge, l\'enrôlement part normalement', async () => {
    const g = mk(); useGame.mockReturnValue(g)
    render(<EnrollPage />)
    await submit('Chloé')
    expect(g.connectVirtualPlayer).toHaveBeenCalledWith('Chloé')
  })
})
