import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, waitFor, cleanup } from '@testing-library/react'
import EnrollPage from './EnrollPage'
import { useGame } from '../hooks/GameContext'
import { REDIRECT_MESSAGES, DEFAULT_REDIRECT_MESSAGE } from '../utils/playerConnectMessages'

// ---------------------------------------------------------------------------
// Tests : EnrollPage — bandeau de motif de renvoi + identité par ID (#120)
//
// Complète EnrollPage.test.jsx (#109, inchangé) avec :
// - le bandeau affiché quand VPlayerPage relaie un motif de renvoi via
//   sessionStorage (PLAYER_EVICTED ou filet de sécurité local) — F3 ;
// - la recherche du bumper existant par ID plutôt que par nom — F2, ferme la
//   même course que côté VPlayerPage : l'absence du bumper dans un roster
//   déjà chargé n'est plus jamais traitée comme une preuve de session morte ;
// - la complétion de la non-régression #109 (ENROLLMENT_CLOSED, INVALID_NAME,
//   non couverts par EnrollPage.test.jsx).
// ---------------------------------------------------------------------------

const mockNavigate = vi.hoisted(() => vi.fn())

vi.mock('react-router-dom', () => ({
  useNavigate: () => mockNavigate,
}))

vi.mock('../hooks/GameContext', () => ({
  useGame: vi.fn(),
}))

vi.mock('./EnrollPage.css', () => ({}))

function createStorageMock() {
  let store = {}
  return {
    getItem: (k) => (Object.prototype.hasOwnProperty.call(store, k) ? store[k] : null),
    setItem: (k, v) => { store[k] = String(v) },
    removeItem: (k) => { delete store[k] },
    clear: () => { store = {} },
  }
}

const makeGameMock = (overrides = {}) => ({
  connectVirtualPlayer: vi.fn(),
  clearPlayerConnectStatus: vi.fn(),
  playerConnectStatus: null,
  status: 'connected',
  bumpers: {},
  gameState: { enrollmentActive: true, virtualPlayerCount: 0, virtualPlayerLimit: 20 },
  ...overrides,
})

describe('EnrollPage — bandeau de motif de renvoi (#120, F3)', () => {
  beforeEach(() => {
    mockNavigate.mockClear()
    vi.stubGlobal('localStorage', createStorageMock())
    vi.stubGlobal('sessionStorage', createStorageMock())
    useGame.mockReset()
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('aucun bandeau dans le cas nominal (pas de motif relayé)', async () => {
    useGame.mockReturnValue(makeGameMock())
    render(<EnrollPage />)

    await waitFor(() => expect(screen.getByLabelText('Choisis ton pseudo')).toBeInTheDocument())
    expect(screen.queryByRole('status')).toBeNull()
  })

  it('affiche le bandeau "place libérée" pour PLAYER_REMOVED', async () => {
    sessionStorage.setItem('vplayer_redirect_reason', 'PLAYER_REMOVED')
    useGame.mockReturnValue(makeGameMock())
    render(<EnrollPage />)

    await waitFor(() => expect(screen.getByText(REDIRECT_MESSAGES.PLAYER_REMOVED)).toBeInTheDocument())
    expect(screen.getByRole('status').className).toContain('warn')
  })

  it('affiche le bandeau "place libérée, même pseudo" pour SEAT_RELEASED (#134)', async () => {
    sessionStorage.setItem('vplayer_redirect_reason', 'SEAT_RELEASED')
    useGame.mockReturnValue(makeGameMock())
    render(<EnrollPage />)

    await waitFor(() => expect(screen.getByText(REDIRECT_MESSAGES.SEAT_RELEASED)).toBeInTheDocument())
    expect(screen.getByRole('status').className).toContain('warn')
  })

  it('affiche le bandeau "nouvelle partie" pour GAME_RESET', async () => {
    sessionStorage.setItem('vplayer_redirect_reason', 'GAME_RESET')
    useGame.mockReturnValue(makeGameMock())
    render(<EnrollPage />)

    await waitFor(() => expect(screen.getByText(REDIRECT_MESSAGES.GAME_RESET)).toBeInTheDocument())
    expect(screen.getByRole('status').className).toContain('info')
  })

  it('affiche le bandeau pour SESSION_EXPIRED (filet de sécurité local)', async () => {
    sessionStorage.setItem('vplayer_redirect_reason', 'SESSION_EXPIRED')
    useGame.mockReturnValue(makeGameMock())
    render(<EnrollPage />)

    await waitFor(() => expect(screen.getByText(REDIRECT_MESSAGES.SESSION_EXPIRED)).toBeInTheDocument())
  })

  it('affiche le message générique pour un motif inconnu (jamais de bandeau muet)', async () => {
    sessionStorage.setItem('vplayer_redirect_reason', 'SOME_FUTURE_REASON')
    useGame.mockReturnValue(makeGameMock())
    render(<EnrollPage />)

    await waitFor(() => expect(screen.getByText(DEFAULT_REDIRECT_MESSAGE)).toBeInTheDocument())
  })

  it('affiche le bandeau générique même pour une chaîne vide (motif transmis mais absent côté serveur)', async () => {
    sessionStorage.setItem('vplayer_redirect_reason', '')
    useGame.mockReturnValue(makeGameMock())
    render(<EnrollPage />)

    await waitFor(() => expect(screen.getByText(DEFAULT_REDIRECT_MESSAGE)).toBeInTheDocument())
  })

  it('affiche le bandeau au-dessus de l\'écran d\'attente quand les inscriptions sont fermées', async () => {
    sessionStorage.setItem('vplayer_redirect_reason', 'GAME_RESET')
    useGame.mockReturnValue(makeGameMock({
      gameState: { enrollmentActive: false, virtualPlayerCount: 0, virtualPlayerLimit: 20 },
    }))
    render(<EnrollPage />)

    await waitFor(() => expect(screen.getByText(REDIRECT_MESSAGES.GAME_RESET)).toBeInTheDocument())
    expect(screen.getByText(/inscriptions ne sont pas ouvertes|ouverture des inscriptions/i)).toBeInTheDocument()
  })

  it('bandeau consommé : le motif est retiré de sessionStorage, un remontage ultérieur (rechargement) ne le réaffiche pas', async () => {
    sessionStorage.setItem('vplayer_redirect_reason', 'PLAYER_REMOVED')
    useGame.mockReturnValue(makeGameMock())
    render(<EnrollPage />)

    await waitFor(() => expect(screen.getByText(REDIRECT_MESSAGES.PLAYER_REMOVED)).toBeInTheDocument())
    expect(sessionStorage.getItem('vplayer_redirect_reason')).toBeNull()

    cleanup() // unmount, simulating navigation away
    render(<EnrollPage />) // fresh mount, simulating a page reload

    await waitFor(() => expect(screen.getByLabelText('Choisis ton pseudo')).toBeInTheDocument())
    expect(screen.queryByText(REDIRECT_MESSAGES.PLAYER_REMOVED)).toBeNull()
  })
})

describe('EnrollPage — recherche du bumper existant par ID (#120, F2)', () => {
  beforeEach(() => {
    mockNavigate.mockClear()
    vi.stubGlobal('localStorage', createStorageMock())
    vi.stubGlobal('sessionStorage', createStorageMock())
    useGame.mockReset()
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('trouve le bumper par ID et navigue directement vers /player, même si le nom a changé côté serveur', async () => {
    localStorage.setItem('vplayer_name', 'Alice')
    localStorage.setItem('vplayer_id', 'vjoueur_alice_1')

    useGame.mockReturnValue(makeGameMock({
      bumpers: {
        vjoueur_alice_1: { NAME: 'AliceRenamed', IS_VIRTUAL: true },
      },
    }))
    render(<EnrollPage />)

    await waitFor(() => expect(mockNavigate).toHaveBeenCalledWith('/player'))
  })

  it('retombe sur la recherche par nom quand aucun vplayer_id n\'est stocké (session antérieure à #120)', async () => {
    localStorage.setItem('vplayer_name', 'Alice')
    // No vplayer_id.

    useGame.mockReturnValue(makeGameMock({
      bumpers: {
        some_legacy_key: { NAME: 'Alice', IS_VIRTUAL: true },
      },
    }))
    render(<EnrollPage />)

    await waitFor(() => expect(mockNavigate).toHaveBeenCalledWith('/player'))
  })

  it('ne renvoie plus le formulaire quand le roster est chargé mais ne contient pas encore notre ID (ferme la course, comme VPlayerPage)', async () => {
    localStorage.setItem('vplayer_name', 'Alice')
    localStorage.setItem('vplayer_session', '1234567890')
    localStorage.setItem('vplayer_id', 'vjoueur_alice_1')

    // Non-empty roster (someone else already loaded) that does NOT yet
    // contain our own bumper — the old code's "else if (bumpers loaded)"
    // branch used to clear the session right here. #120 removes that branch
    // entirely: absence from an already-loaded roster is never proof of a
    // dead session (only PLAYER_EVICTED, handled on VPlayerPage, is authoritative).
    useGame.mockReturnValue(makeGameMock({
      bumpers: {
        vjoueur_bob: { NAME: 'Bob', IS_VIRTUAL: true },
      },
    }))
    render(<EnrollPage />)

    // The session-check effect runs synchronously on mount — no need to wait
    // out the real 2s checking-session timeout to observe whether it cleared
    // anything: it either does so immediately, or (per the fix) never does.
    await Promise.resolve()

    expect(mockNavigate).not.toHaveBeenCalledWith('/')
    expect(localStorage.getItem('vplayer_name')).toBe('Alice')
    expect(localStorage.getItem('vplayer_id')).toBe('vjoueur_alice_1')
  })
})

describe('EnrollPage — non-régression #109 (complète EnrollPage.test.jsx)', () => {
  beforeEach(() => {
    mockNavigate.mockClear()
    vi.stubGlobal('localStorage', createStorageMock())
    vi.stubGlobal('sessionStorage', createStorageMock())
    useGame.mockReset()
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('affiche toujours ENROLLMENT_CLOSED sur PLAYER_REJECTED', async () => {
    const mock = makeGameMock()
    useGame.mockReturnValue(mock)
    const { rerender } = render(<EnrollPage />)
    await waitFor(() => expect(screen.getByLabelText('Choisis ton pseudo')).toBeInTheDocument())

    useGame.mockReturnValue(makeGameMock({
      ...mock,
      playerConnectStatus: { status: 'rejected', reason: 'ENROLLMENT_CLOSED' },
    }))
    rerender(<EnrollPage />)

    await waitFor(() => expect(screen.getByText('Les inscriptions sont fermées')).toBeInTheDocument())
  })

  it('affiche toujours INVALID_NAME sur PLAYER_REJECTED', async () => {
    const mock = makeGameMock()
    useGame.mockReturnValue(mock)
    const { rerender } = render(<EnrollPage />)
    await waitFor(() => expect(screen.getByLabelText('Choisis ton pseudo')).toBeInTheDocument())

    useGame.mockReturnValue(makeGameMock({
      ...mock,
      playerConnectStatus: { status: 'rejected', reason: 'INVALID_NAME' },
    }))
    rerender(<EnrollPage />)

    await waitFor(() => expect(screen.getByText('Pseudo invalide, choisis-en un autre')).toBeInTheDocument())
  })
})
