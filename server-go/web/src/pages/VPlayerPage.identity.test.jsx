import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, act, fireEvent } from '@testing-library/react'
import VPlayerPage from './VPlayerPage'
import { useGame } from '../hooks/GameContext'

// #246 (F-ID) — identité de l'onglet : liaison à chaque ouverture, onglet
// périmé, DUPLICATE_SESSION, effacement limité au localStorage de l'onglet.

const mockNavigate = vi.hoisted(() => vi.fn())
vi.mock('react-router-dom', () => ({ useNavigate: () => mockNavigate }))
vi.mock('../hooks/GameContext', () => ({ useGame: vi.fn() }))
vi.mock('./VPlayerPage.css', () => ({}))
vi.mock('nosleep.js', () => ({ default: class { enable() { return Promise.resolve() } disable() {} } }))
vi.mock('./PlayerDisplay', () => ({ default: vi.fn(() => null) }))
vi.mock('../components/ArdoiseKeyboard', () => ({
  default: ({ value, onChange }) => (
    <input data-testid="ardoise-input" value={value} onChange={(e) => onChange(e.target.value)} />
  ),
}))

function createStorage() {
  let store = {}
  return {
    getItem: (k) => (k in store ? store[k] : null),
    setItem: (k, v) => { store[k] = String(v) },
    removeItem: (k) => { delete store[k] },
    clear: () => { store = {} },
  }
}

const ID = 'vjoueur_alice_1'
const bumpers = { [ID]: { NAME: 'Alice', IS_VIRTUAL: true, IS_VPLAYER: true, CONNECTED: true } }

const makeGameMock = (overrides = {}) => ({
  sendMessage: vi.fn(() => true),
  sendClientTrace: vi.fn(),
  gameState: { phase: 'STOPPED', question: null, enrollmentActive: true },
  bumpers,
  teams: {},
  status: 'connected',
  playerConnectStatus: null,
  clearPlayerConnectStatus: vi.fn(),
  playerEvictedStatus: null,
  clearPlayerEvictedStatus: vi.fn(),
  ...overrides,
})

describe('VPlayerPage — identité de l\'onglet (#246, F-ID)', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    mockNavigate.mockClear()
    vi.stubGlobal('localStorage', createStorage())
    vi.stubGlobal('sessionStorage', createStorage())
    localStorage.setItem('vplayer_name', 'Alice')
    localStorage.setItem('vplayer_session', '1')
    localStorage.setItem('vplayer_id', ID)
    useGame.mockReset()
  })
  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
  })

  it('envoie PLAYER_CONNECT {NAME, ID} à l\'ouverture même si le bumper est déjà CONNECTED', async () => {
    const mock = makeGameMock()
    useGame.mockReturnValue(mock)
    render(<VPlayerPage />)
    await act(async () => {})
    expect(mock.sendMessage).toHaveBeenCalledWith('PLAYER_CONNECT', { NAME: 'Alice', ID })
  })

  it('renvoie PLAYER_CONNECT à chaque nouvelle ouverture (status connected → disconnected → connected)', async () => {
    const mock = makeGameMock({ playerConnectStatus: { status: 'connected', id: ID, name: 'Alice' } })
    useGame.mockReturnValue(mock)
    const { rerender } = render(<VPlayerPage />)
    await act(async () => {})
    mock.sendMessage.mockClear()

    useGame.mockReturnValue({ ...mock, status: 'disconnected', playerConnectStatus: null })
    rerender(<VPlayerPage />)
    await act(async () => {})
    useGame.mockReturnValue({ ...mock, status: 'connected', playerConnectStatus: null })
    rerender(<VPlayerPage />)
    await act(async () => {})
    expect(mock.sendMessage).toHaveBeenCalledWith('PLAYER_CONNECT', { NAME: 'Alice', ID })
  })

  it('utilise l\'ID de la session en mémoire, jamais le localStorage courant (onglet périmé : aucun PLAYER_CONNECT)', async () => {
    const mock = makeGameMock()
    useGame.mockReturnValue(mock)
    const { container } = render(<VPlayerPage />)
    await act(async () => {})
    mock.sendMessage.mockClear()

    // Un autre onglet enrôle un autre joueur dans le localStorage partagé.
    localStorage.setItem('vplayer_id', 'vjoueur_bob_9')
    localStorage.setItem('vplayer_name', 'Bob')
    await act(async () => { window.dispatchEvent(new StorageEvent('storage', { key: 'vplayer_id' })) })

    expect(container.querySelector('[data-testid="vplayer-stale-tab"]')).not.toBeNull()
    await act(async () => { vi.advanceTimersByTime(5000) })
    expect(mock.sendMessage).not.toHaveBeenCalledWith('PLAYER_CONNECT', expect.anything())
    expect(mock.sendClientTrace).toHaveBeenCalledWith('STALE_TAB', expect.anything())
  })

  it('onglet périmé dès le montage : écran dédié, aucun PLAYER_CONNECT, localStorage de l\'autre session intact', async () => {
    // Session en mémoire = ID (montage), puis un autre onglet écrit avant la 1re émission.
    const mock = makeGameMock({ status: 'connecting' })
    useGame.mockReturnValue(mock)
    const { container, rerender } = render(<VPlayerPage />)
    await act(async () => {})
    localStorage.setItem('vplayer_id', 'vjoueur_bob_9')
    useGame.mockReturnValue({ ...mock, status: 'connected' })
    rerender(<VPlayerPage />)
    await act(async () => {})
    expect(container.querySelector('[data-testid="vplayer-stale-tab"]')).not.toBeNull()
    expect(mock.sendMessage).not.toHaveBeenCalledWith('PLAYER_CONNECT', expect.anything())
    expect(localStorage.getItem('vplayer_id')).toBe('vjoueur_bob_9')
  })

  it('PLAYER_EVICTED PLAYER_REMOVED d\'un onglet dont l\'ID n\'est plus celui du localStorage n\'efface pas ce dernier', async () => {
    const mock = makeGameMock({ playerEvictedStatus: { reason: 'PLAYER_REMOVED' } })
    useGame.mockReturnValue(mock)
    render(<VPlayerPage />)
    await act(async () => {})
    localStorage.setItem('vplayer_id', 'vjoueur_bob_9')
    await act(async () => { vi.advanceTimersByTime(3500) })
    expect(localStorage.getItem('vplayer_id')).toBe('vjoueur_bob_9')
  })

  it('DUPLICATE_SESSION : n\'efface rien, cesse d\'émettre, propose « Reprendre ici »', async () => {
    const mock = makeGameMock({ playerEvictedStatus: { reason: 'DUPLICATE_SESSION' } })
    useGame.mockReturnValue(mock)
    const { container, getByText } = render(<VPlayerPage />)
    await act(async () => {})

    expect(container.querySelector('[data-testid="vplayer-duplicate-session"]')).not.toBeNull()
    expect(localStorage.getItem('vplayer_id')).toBe(ID)
    expect(localStorage.getItem('vplayer_name')).toBe('Alice')
    expect(mockNavigate).not.toHaveBeenCalledWith('/')
    expect(mock.sendClientTrace).toHaveBeenCalledWith('DUPLICATE_SESSION', expect.anything())

    mock.sendMessage.mockClear()
    await act(async () => { vi.advanceTimersByTime(5000) })
    expect(mock.sendMessage).not.toHaveBeenCalled()

    await act(async () => { fireEvent.click(getByText('Reprendre ici')) })
    expect(mock.sendMessage).toHaveBeenCalledWith('PLAYER_CONNECT', { NAME: 'Alice', ID })
  })

  it('ARDOISE : texte mis en attente tant que la connexion n\'est pas liée, puis renvoi complet (REASON=resend)', async () => {
    const gameState = { phase: 'STARTED', question: { ID: 'q1', TYPE: 'ARDOISE' }, enrollmentActive: false }
    const mock = makeGameMock({ gameState })
    useGame.mockReturnValue(mock)
    const { getByTestId, rerender } = render(<VPlayerPage />)
    await act(async () => {})

    fireEvent.change(getByTestId('ardoise-input'), { target: { value: 'PARIS' } })
    await act(async () => { vi.advanceTimersByTime(300) })
    expect(mock.sendMessage.mock.calls.filter(([a]) => a === 'ARDOISE_INPUT')).toHaveLength(0)

    useGame.mockReturnValue({ ...mock, playerConnectStatus: { status: 'connected', id: ID, name: 'Alice' } })
    rerender(<VPlayerPage />)
    await act(async () => {})
    expect(mock.sendMessage).toHaveBeenCalledWith('ARDOISE_INPUT', expect.objectContaining({ TEXT: 'PARIS', ID, REASON: 'resend' }))
  })

  it('adopte le nom serveur renvoyé par PLAYER_CONNECTED (le cas 1 ne renomme plus)', async () => {
    const mock = makeGameMock({ playerConnectStatus: { status: 'connected', id: ID, name: 'Alice-serveur' } })
    useGame.mockReturnValue(mock)
    render(<VPlayerPage />)
    await act(async () => {})
    expect(localStorage.getItem('vplayer_name')).toBe('Alice-serveur')
  })

  it('SEQ croissant par question et par connexion, REASON clear/backspace_empty/key', async () => {
    const gameState = { phase: 'STARTED', question: { ID: 'q1', TYPE: 'ARDOISE' }, enrollmentActive: false }
    const mock = makeGameMock({ gameState, playerConnectStatus: { status: 'connected', id: ID, name: 'Alice' } })
    useGame.mockReturnValue(mock)
    const { getByTestId, rerender } = render(<VPlayerPage />)
    await act(async () => {})
    const input = getByTestId('ardoise-input')
    fireEvent.change(input, { target: { value: 'P' } })
    fireEvent.change(input, { target: { value: 'PA' } })
    await act(async () => { vi.advanceTimersByTime(250) })
    const sends = () => mock.sendMessage.mock.calls.filter(([a]) => a === 'ARDOISE_INPUT').map(([, p]) => p)
    expect(sends().map((p) => p.SEQ)).toEqual([1, 2])
    expect(sends().every((p) => p.REASON === 'key')).toBe(true)

    // Nouvelle question → SEQ repart à 1
    useGame.mockReturnValue({ ...mock, gameState: { ...gameState, question: { ID: 'q2', TYPE: 'ARDOISE' } } })
    rerender(<VPlayerPage />)
    await act(async () => {})
    fireEvent.change(getByTestId('ardoise-input'), { target: { value: 'Z' } })
    expect(sends().at(-1)).toMatchObject({ TEXT: 'Z', SEQ: 1 })
  })
})
