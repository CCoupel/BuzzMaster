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

  it('le filet « session expirée » (10 s) ne remplace ni l\'écran onglet périmé ni l\'écran session ouverte ailleurs', async () => {
    // Bumper absent du roster : le filet s'armerait pour un onglet normal.
    const mock = makeGameMock({ bumpers: {}, playerEvictedStatus: { reason: 'DUPLICATE_SESSION' } })
    useGame.mockReturnValue(mock)
    const dup = render(<VPlayerPage />)
    await act(async () => { vi.advanceTimersByTime(60000) })
    expect(dup.container.querySelector('[data-testid="vplayer-duplicate-session"]')).not.toBeNull()
    expect(mockNavigate).not.toHaveBeenCalledWith('/')
    dup.unmount()

    const mock2 = makeGameMock({ bumpers: {}, status: 'connecting' })
    useGame.mockReturnValue(mock2)
    const stale = render(<VPlayerPage />)
    await act(async () => {})
    localStorage.setItem('vplayer_id', 'vjoueur_bob_9')
    useGame.mockReturnValue({ ...mock2, status: 'connected' })
    stale.rerender(<VPlayerPage />)
    await act(async () => { vi.advanceTimersByTime(60000) })
    expect(stale.container.querySelector('[data-testid="vplayer-stale-tab"]')).not.toBeNull()
    expect(mockNavigate).not.toHaveBeenCalledWith('/')
  })

  it('2 × PLAYER_CONNECTED sur la même connexion : SEQ ne repart pas à 1 (strictement croissant)', async () => {
    const gameState = { phase: 'STARTED', question: { ID: 'q1', TYPE: 'ARDOISE' }, enrollmentActive: false }
    const mock = makeGameMock({ gameState, playerConnectStatus: { status: 'connected', id: ID, name: 'Alice' } })
    useGame.mockReturnValue(mock)
    const { getByTestId, rerender } = render(<VPlayerPage />)
    await act(async () => {})
    fireEvent.change(getByTestId('ardoise-input'), { target: { value: 'P' } })
    fireEvent.change(getByTestId('ardoise-input'), { target: { value: 'PA' } })
    await act(async () => { vi.advanceTimersByTime(250) })

    // second PLAYER_CONNECTED (renvoi à 2 s) : nouvel objet de statut, même connexion
    useGame.mockReturnValue({ ...mock, playerConnectStatus: { status: 'connected', id: ID, name: 'Alice' } })
    rerender(<VPlayerPage />)
    await act(async () => {})
    fireEvent.change(getByTestId('ardoise-input'), { target: { value: 'PAR' } })
    await act(async () => { vi.advanceTimersByTime(250) })

    const seqs = mock.sendMessage.mock.calls.filter(([a]) => a === 'ARDOISE_INPUT').map(([, p]) => p.SEQ)
    expect(seqs.length).toBeGreaterThanOrEqual(3)
    for (let i = 1; i < seqs.length; i++) expect(seqs[i]).toBeGreaterThan(seqs[i - 1])
  })

  it('throttle : effacement puis ressaisie continue de 3 s sans pause → envois toutes les ≤ 200 ms et dernier état envoyé', async () => {
    const gameState = { phase: 'STARTED', question: { ID: 'q1', TYPE: 'ARDOISE' }, enrollmentActive: false }
    const mock = makeGameMock({ gameState, playerConnectStatus: { status: 'connected', id: ID, name: 'Alice' } })
    useGame.mockReturnValue(mock)
    const { getByTestId } = render(<VPlayerPage />)
    await act(async () => {})
    const input = () => getByTestId('ardoise-input')
    fireEvent.change(input(), { target: { value: 'PARIS' } })
    await act(async () => { vi.advanceTimersByTime(300) })
    fireEvent.change(input(), { target: { value: '' } }) // effacement
    let text = ''
    for (let i = 0; i < 60; i++) { // 3 s à une touche / 50 ms : jamais de pause de 200 ms
      text += String.fromCharCode(65 + (i % 26))
      fireEvent.change(input(), { target: { value: text } })
      await act(async () => { vi.advanceTimersByTime(50) })
    }
    const inputs = mock.sendMessage.mock.calls.filter(([a]) => a === 'ARDOISE_INPUT').map(([, p]) => p)
    const afterClear = inputs.slice(2) // [PARIS, ''?, ...]
    expect(afterClear.length).toBeGreaterThanOrEqual(10) // ~ 3 s / 200 ms
    await act(async () => { vi.advanceTimersByTime(250) })
    const all = mock.sendMessage.mock.calls.filter(([a]) => a === 'ARDOISE_INPUT').map(([, p]) => p)
    expect(all.at(-1).TEXT).toBe(text)
    const seqs = all.map((p) => p.SEQ)
    for (let i = 1; i < seqs.length; i++) expect(seqs[i]).toBeGreaterThan(seqs[i - 1])
  })

  it('pull-to-refresh : overscroll-behavior-y none sur la racine pendant la page joueur, restauré au démontage', async () => {
    useGame.mockReturnValue(makeGameMock())
    const { unmount } = render(<VPlayerPage />)
    await act(async () => {})
    expect(document.documentElement.style.overscrollBehaviorY).toBe('none')
    expect(document.body.style.overscrollBehaviorY).toBe('none')
    unmount()
    expect(document.documentElement.style.overscrollBehaviorY).toBe('')
    expect(document.body.style.overscrollBehaviorY).toBe('')
  })

  it('démontage : le timer de throttle est annulé, aucun envoi après la sortie de la page', async () => {
    const gameState = { phase: 'STARTED', question: { ID: 'q1', TYPE: 'ARDOISE' }, enrollmentActive: false }
    const mock = makeGameMock({ gameState, playerConnectStatus: { status: 'connected', id: ID, name: 'Alice' } })
    useGame.mockReturnValue(mock)
    const { getByTestId, unmount } = render(<VPlayerPage />)
    await act(async () => {})
    fireEvent.change(getByTestId('ardoise-input'), { target: { value: 'P' } }) // envoi immédiat
    fireEvent.change(getByTestId('ardoise-input'), { target: { value: 'PA' } }) // programmé (trailing)
    const before = mock.sendMessage.mock.calls.filter(([a]) => a === 'ARDOISE_INPUT').length
    unmount()
    await act(async () => { vi.advanceTimersByTime(1000) })
    expect(mock.sendMessage.mock.calls.filter(([a]) => a === 'ARDOISE_INPUT').length).toBe(before)
  })

  it('brouillon : conservé si la question disparaît puis revient avec le même ID (A → null → A), effacé pour un autre ID', async () => {
    const q = (id) => ({ phase: 'STARTED', question: id ? { ID: id, TYPE: 'ARDOISE' } : null, enrollmentActive: false })
    const mock = makeGameMock({ gameState: q('q1'), playerConnectStatus: { status: 'connected', id: ID, name: 'Alice' } })
    useGame.mockReturnValue(mock)
    const { getByTestId, rerender } = render(<VPlayerPage />)
    await act(async () => {})
    fireEvent.change(getByTestId('ardoise-input'), { target: { value: 'PAR' } })
    const draft = () => sessionStorage.getItem('vplayer_ardoise_draft')
    expect(draft()).toContain('PAR')

    useGame.mockReturnValue({ ...mock, gameState: q(null) })
    rerender(<VPlayerPage />)
    await act(async () => {})
    expect(draft()).toContain('PAR')

    useGame.mockReturnValue({ ...mock, gameState: q('q1') })
    rerender(<VPlayerPage />)
    await act(async () => {})
    expect(draft()).toContain('PAR')
    expect(getByTestId('ardoise-input').value).toBe('PAR') // restauré

    useGame.mockReturnValue({ ...mock, gameState: q('q2') })
    rerender(<VPlayerPage />)
    await act(async () => {})
    expect(draft()).toBeNull()
  })
})
