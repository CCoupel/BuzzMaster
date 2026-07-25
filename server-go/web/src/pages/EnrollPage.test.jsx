import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import EnrollPage from './EnrollPage'
import { useGame } from '../hooks/GameContext'

// ---------------------------------------------------------------------------
// localStorage mock — cet environnement de test expose un `localStorage`
// global inerte (objet vide, sans getItem/setItem/clear : Node fournit un
// global `localStorage` expérimental qui masque celui de jsdom). On le
// remplace par un mock mémoire simple, comme le mock WebSocket des tests de
// hooks existants (useWebSocket.ardoise.test.js).
// ---------------------------------------------------------------------------

function createLocalStorageMock() {
  let store = {}
  return {
    getItem: (k) => (Object.prototype.hasOwnProperty.call(store, k) ? store[k] : null),
    setItem: (k, v) => { store[k] = String(v) },
    removeItem: (k) => { delete store[k] },
    clear: () => { store = {} },
  }
}

// ---------------------------------------------------------------------------
// Tests : EnrollPage — attente PLAYER_CONNECTED/PLAYER_REJECTED (fix R1, #109)
//
// Avant le fix, EnrollPage naviguait vers /player après un délai fixe (500ms)
// sans jamais vérifier la réponse du serveur — un rejet NAME_TAKEN restait
// invisible. Le fix consomme `playerConnectStatus` (exposé par useWebSocket/
// useGame) : 'connected' -> navigation + persistance localStorage ;
// 'rejected' -> message d'erreur (REJECTION_MESSAGES) + reset du formulaire,
// sans navigation, sans persister localStorage.
// ---------------------------------------------------------------------------

const mockNavigate = vi.hoisted(() => vi.fn())

vi.mock('react-router-dom', () => ({
  useNavigate: () => mockNavigate,
}))

vi.mock('../hooks/GameContext', () => ({
  useGame: vi.fn(),
}))

vi.mock('./EnrollPage.css', () => ({}))

const makeGameMock = (overrides = {}) => ({
  connectVirtualPlayer: vi.fn(),
  clearPlayerConnectStatus: vi.fn(),
  playerConnectStatus: null,
  status: 'connected',
  bumpers: {},
  gameState: { enrollmentActive: true, virtualPlayerCount: 0, virtualPlayerLimit: 20 },
  ...overrides,
})

describe('EnrollPage — soumission et attente de la réponse serveur', () => {
  beforeEach(() => {
    mockNavigate.mockClear()
    vi.stubGlobal('localStorage', createLocalStorageMock())
    useGame.mockReset()
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it("envoie PLAYER_CONNECT et affiche 'Connexion...' pendant l'attente", async () => {
    const mock = makeGameMock()
    useGame.mockReturnValue(mock)
    render(<EnrollPage />)

    await waitFor(() => expect(screen.getByLabelText('Choisis ton pseudo')).toBeInTheDocument())

    fireEvent.change(screen.getByLabelText('Choisis ton pseudo'), { target: { value: 'Alice' } })
    fireEvent.click(screen.getByRole('button', { name: /Rejoindre la partie/i }))

    expect(mock.connectVirtualPlayer).toHaveBeenCalledWith('Alice')
    expect(screen.getByRole('button', { name: /Connexion/i })).toBeDisabled()
  })

  it('navigue vers /player et persiste localStorage sur PLAYER_CONNECTED (status=connected)', async () => {
    const mock = makeGameMock()
    useGame.mockReturnValue(mock)
    const { rerender } = render(<EnrollPage />)

    await waitFor(() => expect(screen.getByLabelText('Choisis ton pseudo')).toBeInTheDocument())
    fireEvent.change(screen.getByLabelText('Choisis ton pseudo'), { target: { value: 'Alice' } })
    fireEvent.click(screen.getByRole('button', { name: /Rejoindre la partie/i }))

    // Simulate the WS hook receiving PLAYER_CONNECTED.
    useGame.mockReturnValue(makeGameMock({
      ...mock,
      playerConnectStatus: { status: 'connected', id: 'vjoueur_alice_123', name: 'Alice' },
    }))
    rerender(<EnrollPage />)

    await waitFor(() => expect(mockNavigate).toHaveBeenCalledWith('/player'))
    expect(localStorage.getItem('vplayer_name')).toBe('Alice')
    expect(localStorage.getItem('vplayer_session')).not.toBeNull()
  })

  it("affiche le message NAME_TAKEN, ne navigue pas, et permet de ressaisir un pseudo sur PLAYER_REJECTED", async () => {
    const mock = makeGameMock()
    useGame.mockReturnValue(mock)
    const { rerender } = render(<EnrollPage />)

    await waitFor(() => expect(screen.getByLabelText('Choisis ton pseudo')).toBeInTheDocument())
    fireEvent.change(screen.getByLabelText('Choisis ton pseudo'), { target: { value: 'Emma' } })
    fireEvent.click(screen.getByRole('button', { name: /Rejoindre la partie/i }))

    useGame.mockReturnValue(makeGameMock({
      ...mock,
      playerConnectStatus: { status: 'rejected', reason: 'NAME_TAKEN' },
    }))
    rerender(<EnrollPage />)

    await waitFor(() =>
      expect(screen.getByText('Ce pseudo est déjà utilisé, choisis-en un autre')).toBeInTheDocument()
    )
    expect(mockNavigate).not.toHaveBeenCalled()
    // localStorage must NOT have been persisted for a rejected attempt.
    expect(localStorage.getItem('vplayer_name')).toBeNull()
    // Form re-enabled: the submit button must show its idle label again.
    expect(screen.getByRole('button', { name: /Rejoindre la partie/i })).toBeInTheDocument()
    // Input cleared, ready for a new pseudo.
    expect(screen.getByLabelText('Choisis ton pseudo').value).toBe('')
  })

  it('consomme playerConnectStatus après traitement (clearPlayerConnectStatus appelé)', async () => {
    const mock = makeGameMock()
    useGame.mockReturnValue(mock)
    const { rerender } = render(<EnrollPage />)

    await waitFor(() => expect(screen.getByLabelText('Choisis ton pseudo')).toBeInTheDocument())

    const rejectedMock = makeGameMock({
      ...mock,
      playerConnectStatus: { status: 'rejected', reason: 'NAME_TAKEN' },
    })
    useGame.mockReturnValue(rejectedMock)
    rerender(<EnrollPage />)

    await waitFor(() => expect(rejectedMock.clearPlayerConnectStatus).toHaveBeenCalled())
  })

  it('affiche un message générique pour une raison de rejet inconnue', async () => {
    const mock = makeGameMock()
    useGame.mockReturnValue(mock)
    const { rerender } = render(<EnrollPage />)
    await waitFor(() => expect(screen.getByLabelText('Choisis ton pseudo')).toBeInTheDocument())

    useGame.mockReturnValue(makeGameMock({
      ...mock,
      playerConnectStatus: { status: 'rejected', reason: 'SOME_FUTURE_REASON' },
    }))
    rerender(<EnrollPage />)

    await waitFor(() =>
      expect(screen.getByText('Connexion refusée, réessaie avec un autre pseudo')).toBeInTheDocument()
    )
  })

  it('non-régression : les raisons de rejet existantes restent affichées correctement', async () => {
    const mock = makeGameMock()
    useGame.mockReturnValue(mock)
    const { rerender } = render(<EnrollPage />)
    await waitFor(() => expect(screen.getByLabelText('Choisis ton pseudo')).toBeInTheDocument())

    useGame.mockReturnValue(makeGameMock({
      ...mock,
      playerConnectStatus: { status: 'rejected', reason: 'LIMIT_REACHED' },
    }))
    rerender(<EnrollPage />)

    await waitFor(() =>
      expect(screen.getByText('Nombre maximum de joueurs atteint')).toBeInTheDocument()
    )
  })
})
