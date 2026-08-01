import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import EnrollPage from './EnrollPage'
import { useGame } from '../hooks/GameContext'
import { REJECTION_MESSAGES } from '../utils/playerConnectMessages'

// ---------------------------------------------------------------------------
// Tests : EnrollPage — message différencié NAME_TAKEN_OFFLINE (#122, F2)
//
// Plan : _work/reports/plan-20260730-123000.md.
//
// #109 R1 forces identity by bumper ID; a player who lost theirs (storage
// wiped, new device, an earlier rejection) can never prove ownership again
// and always hit NAME_TAKEN — the WORST advice for reclaiming one's own
// seat. #122 (B1) distinguishes the reason server-side by the holder's
// connection state; this file locks the two resulting client messages, and
// that NAME_TAKEN itself is completely untouched (#109 non-regression).
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

async function submitPseudo(name) {
  await waitFor(() => expect(screen.getByLabelText('Choisis ton pseudo')).toBeInTheDocument())
  fireEvent.change(screen.getByLabelText('Choisis ton pseudo'), { target: { value: name } })
  fireEvent.click(screen.getByRole('button', { name: /Rejoindre la partie/i }))
}

describe('EnrollPage — NAME_TAKEN_OFFLINE invite à demander la place (#122, F2)', () => {
  beforeEach(() => {
    mockNavigate.mockClear()
    vi.stubGlobal('localStorage', createLocalStorageMock())
    useGame.mockReset()
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('affiche le message d\'invitation à demander la place, pas le conseil "choisis-en un autre"', async () => {
    const mock = makeGameMock()
    useGame.mockReturnValue(mock)
    const { rerender } = render(<EnrollPage />)
    await submitPseudo('Emma')

    useGame.mockReturnValue(makeGameMock({
      ...mock,
      playerConnectStatus: { status: 'rejected', reason: 'NAME_TAKEN_OFFLINE' },
    }))
    rerender(<EnrollPage />)

    await waitFor(() =>
      expect(screen.getByText(REJECTION_MESSAGES.NAME_TAKEN_OFFLINE)).toBeInTheDocument()
    )
    expect(screen.queryByText(REJECTION_MESSAGES.NAME_TAKEN)).toBeNull()
    expect(mockNavigate).not.toHaveBeenCalled()
    // Non-persisted — same guarantee as any other rejection (#109).
    expect(localStorage.getItem('vplayer_name')).toBeNull()
  })

  it('non-régression stricte #109 : NAME_TAKEN affiche toujours son texte exact, inchangé', async () => {
    const mock = makeGameMock()
    useGame.mockReturnValue(mock)
    const { rerender } = render(<EnrollPage />)
    await submitPseudo('Emma')

    useGame.mockReturnValue(makeGameMock({
      ...mock,
      playerConnectStatus: { status: 'rejected', reason: 'NAME_TAKEN' },
    }))
    rerender(<EnrollPage />)

    await waitFor(() =>
      expect(screen.getByText('Ce pseudo est déjà utilisé, choisis-en un autre')).toBeInTheDocument()
    )
    expect(REJECTION_MESSAGES.NAME_TAKEN).toBe('Ce pseudo est déjà utilisé, choisis-en un autre')
  })

  it('permet de ressaisir un pseudo après NAME_TAKEN_OFFLINE, comme pour tout autre rejet', async () => {
    const mock = makeGameMock()
    useGame.mockReturnValue(mock)
    const { rerender } = render(<EnrollPage />)
    await submitPseudo('Emma')

    useGame.mockReturnValue(makeGameMock({
      ...mock,
      playerConnectStatus: { status: 'rejected', reason: 'NAME_TAKEN_OFFLINE' },
    }))
    rerender(<EnrollPage />)

    await waitFor(() => expect(screen.getByRole('button', { name: /Rejoindre la partie/i })).toBeInTheDocument())
    expect(screen.getByLabelText('Choisis ton pseudo').value).toBe('')
  })
})
