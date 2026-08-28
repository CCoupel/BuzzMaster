import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, waitFor, act } from '@testing-library/react'
import VPlayerPage from './VPlayerPage'
import PlayerDisplay from './PlayerDisplay'
import { useGame } from '../hooks/GameContext'

// ---------------------------------------------------------------------------
// Tests : VPlayerPage — reconnexion par ID (fix R1, #109)
//
// La reconnexion auto (2s après montage si le bumper n'est pas retrouvé par
// nom) doit désormais renvoyer l'ID stocké en localStorage (`vplayer_id`,
// capturé depuis un PLAYER_CONNECTED précédent) en plus du nom — sans ID
// stocké (jamais connecté depuis cet appareil), l'ancien comportement
// (nom seul) reste le fallback. Un rejet (`playerConnectStatus.status ===
// 'rejected'`) doit bloquer l'écran sur un message d'erreur + bouton pour
// repartir sur EnrollPage, et purger `vplayer_id` (ne jamais retenter avec
// un ID désormais invalide).
// ---------------------------------------------------------------------------

const mockNavigate = vi.hoisted(() => vi.fn())

vi.mock('react-router-dom', () => ({
  useNavigate: () => mockNavigate,
}))

vi.mock('../hooks/GameContext', () => ({
  useGame: vi.fn(),
}))

vi.mock('./VPlayerPage.css', () => ({}))

vi.mock('nosleep.js', () => ({
  default: class NoSleep {
    enable() { return Promise.resolve() }
    disable() {}
  },
}))

vi.mock('./PlayerDisplay', () => ({ default: vi.fn(() => null) }))
vi.mock('../components/ArdoiseKeyboard', () => ({ default: () => null }))

// localStorage mock — voir EnrollPage.test.jsx pour le contexte (global
// `localStorage` inerte dans cet environnement de test).
function createLocalStorageMock() {
  let store = {}
  return {
    getItem: (k) => (Object.prototype.hasOwnProperty.call(store, k) ? store[k] : null),
    setItem: (k, v) => { store[k] = String(v) },
    removeItem: (k) => { delete store[k] },
    clear: () => { store = {} },
  }
}

const makeGameMock = (overrides = {}) => ({
  sendMessage: vi.fn(),
  gameState: { phase: 'STOPPED', question: null },
  bumpers: {},
  teams: {},
  status: 'connected',
  playerConnectStatus: null,
  clearPlayerConnectStatus: vi.fn(),
  ...overrides,
})

describe('VPlayerPage — reconnexion auto renvoie l\'ID stocké', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    mockNavigate.mockClear()
    vi.stubGlobal('localStorage', createLocalStorageMock())
    localStorage.setItem('vplayer_name', 'Alice')
    localStorage.setItem('vplayer_session', '1234567890')
    useGame.mockReset()
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
  })

  it('renvoie NAME + ID quand un vplayer_id est stocké (bumper introuvable par nom)', async () => {
    localStorage.setItem('vplayer_id', 'vjoueur_alice_123')
    const mock = makeGameMock()
    useGame.mockReturnValue(mock)

    render(<VPlayerPage />)

    await act(async () => {
      vi.advanceTimersByTime(2000)
    })

    expect(mock.sendMessage).toHaveBeenCalledWith('PLAYER_CONNECT', { NAME: 'Alice', ID: 'vjoueur_alice_123' })
  })

  it("renvoie NAME seul (pas de clé ID) quand vplayer_id est absent", async () => {
    // No vplayer_id in localStorage — never connected from this device before.
    const mock = makeGameMock()
    useGame.mockReturnValue(mock)

    render(<VPlayerPage />)

    await act(async () => {
      vi.advanceTimersByTime(2000)
    })

    expect(mock.sendMessage).toHaveBeenCalledWith('PLAYER_CONNECT', { NAME: 'Alice' })
    const [, payload] = mock.sendMessage.mock.calls[0]
    expect(payload).not.toHaveProperty('ID')
  })

  it('ne tente pas de reconnexion si le bumper est déjà retrouvé par nom ET connecté (CONNECTED=true)', async () => {
    const mock = makeGameMock({
      bumpers: { vjoueur_alice_123: { NAME: 'Alice', IS_VIRTUAL: true, IS_VPLAYER: true, TEAM: '', CONNECTED: true } },
    })
    useGame.mockReturnValue(mock)

    render(<VPlayerPage />)

    await act(async () => {
      vi.advanceTimersByTime(2000)
    })

    expect(mock.sendMessage).not.toHaveBeenCalledWith('PLAYER_CONNECT', expect.anything())
  })

  // Régression (#109, _work/reports/dev-frontend-20260726-091500-conn-state-verify.md) :
  // avant le fix, un bumper retrouvé par nom court-circuitait la reconnexion
  // même déconnecté (CONNECTED=false ou absent) — après une coupure réseau
  // normale, le WebSocket brut se rétablissait, l'UPDATE renvoyait ce même
  // bumper encore orange/rouge, le matching par nom le trouvait, et
  // PLAYER_CONNECT n'était alors JAMAIS renvoyé : le badge admin restait
  // bloqué pour de bon. Le fix ne considère "déjà connecté" (court-circuit
  // légitime) que si bumperData.CONNECTED === true.
  it('tente bien la reconnexion (avec ID stocké) si le bumper est retrouvé par nom mais CONNECTED=false', async () => {
    localStorage.setItem('vplayer_id', 'vjoueur_alice_123')
    const mock = makeGameMock({
      bumpers: { vjoueur_alice_123: { NAME: 'Alice', IS_VIRTUAL: true, IS_VPLAYER: true, TEAM: 'red', CONNECTED: false } },
    })
    useGame.mockReturnValue(mock)

    render(<VPlayerPage />)

    await act(async () => {
      vi.advanceTimersByTime(2000)
    })

    expect(mock.sendMessage).toHaveBeenCalledWith('PLAYER_CONNECT', { NAME: 'Alice', ID: 'vjoueur_alice_123' })
  })

  it('tente bien la reconnexion si le bumper est retrouvé par nom mais CONNECTED est absent (firmware/état pré-fix)', async () => {
    const mock = makeGameMock({
      bumpers: { vjoueur_alice_123: { NAME: 'Alice', IS_VIRTUAL: true, IS_VPLAYER: true, TEAM: 'red' } },
    })
    useGame.mockReturnValue(mock)

    render(<VPlayerPage />)

    await act(async () => {
      vi.advanceTimersByTime(2000)
    })

    expect(mock.sendMessage).toHaveBeenCalledWith('PLAYER_CONNECT', { NAME: 'Alice' })
  })
})

describe('VPlayerPage — rejet de reconnexion (ID périmé / NAME_TAKEN)', () => {
  beforeEach(() => {
    mockNavigate.mockClear()
    vi.stubGlobal('localStorage', createLocalStorageMock())
    localStorage.setItem('vplayer_name', 'Alice')
    localStorage.setItem('vplayer_session', '1234567890')
    localStorage.setItem('vplayer_id', 'vjoueur_alice_stale')
    useGame.mockReset()
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it("affiche un message d'erreur bloquant et purge vplayer_id sur rejet", async () => {
    const mock = makeGameMock()
    useGame.mockReturnValue(mock)
    const { rerender } = render(<VPlayerPage />)

    useGame.mockReturnValue(makeGameMock({
      ...mock,
      playerConnectStatus: { status: 'rejected', reason: 'NAME_TAKEN' },
    }))
    rerender(<VPlayerPage />)

    await waitFor(() =>
      expect(screen.getByText('Ce pseudo est déjà utilisé, choisis-en un autre')).toBeInTheDocument()
    )
    expect(localStorage.getItem('vplayer_id')).toBeNull()
    expect(screen.getByRole('button', { name: /Rejoindre à nouveau/i })).toBeInTheDocument()
  })

  it("'Rejoindre à nouveau' nettoie toute la session locale et renvoie vers l'accueil", async () => {
    const mock = makeGameMock()
    useGame.mockReturnValue(mock)
    const { rerender } = render(<VPlayerPage />)

    useGame.mockReturnValue(makeGameMock({
      ...mock,
      playerConnectStatus: { status: 'rejected', reason: 'NAME_TAKEN' },
    }))
    rerender(<VPlayerPage />)

    await waitFor(() => screen.getByRole('button', { name: /Rejoindre à nouveau/i }))
    screen.getByRole('button', { name: /Rejoindre à nouveau/i }).click()

    expect(localStorage.getItem('vplayer_name')).toBeNull()
    expect(localStorage.getItem('vplayer_session')).toBeNull()
    expect(localStorage.getItem('vplayer_id')).toBeNull()
    expect(mockNavigate).toHaveBeenCalledWith('/')
  })

  it('consomme playerConnectStatus (clearPlayerConnectStatus appelé) après un rejet', async () => {
    const mock = makeGameMock()
    useGame.mockReturnValue(mock)
    const { rerender } = render(<VPlayerPage />)

    const rejectedMock = makeGameMock({
      ...mock,
      playerConnectStatus: { status: 'rejected', reason: 'NAME_TAKEN' },
    })
    useGame.mockReturnValue(rejectedMock)
    rerender(<VPlayerPage />)

    await waitFor(() => expect(rejectedMock.clearPlayerConnectStatus).toHaveBeenCalled())
  })
})

// ---------------------------------------------------------------------------
// Régression (#112) : la couleur du badge nom du VJoueur doit toujours
// correspondre à la couleur de son équipe, y compris après une réponse QCM.
//
// Le backend (engine.go, ProcessButtonPress) réassigne bumper.ANSWER_COLOR à
// chaque réponse QCM et ne le réinitialise jamais — avant le fix, VPlayerPage
// priorisait ANSWER_COLOR sur la couleur d'équipe pour le badge nom, ce qui le
// faisait basculer définitivement sur la dernière couleur de réponse QCM au
// lieu de team.COLOR, en incohérence avec l'admin/la TV.
// ---------------------------------------------------------------------------
describe('VPlayerPage — couleur du badge VJoueur = couleur d\'équipe (#112)', () => {
  beforeEach(() => {
    mockNavigate.mockClear()
    vi.stubGlobal('localStorage', createLocalStorageMock())
    localStorage.setItem('vplayer_name', 'Alice')
    localStorage.setItem('vplayer_session', '1234567890')
    useGame.mockReset()
    PlayerDisplay.mockClear()
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it("utilise team.COLOR pour playerNameColor même si bumper.ANSWER_COLOR est déjà positionné (réponse QCM précédente)", () => {
    const mock = makeGameMock({
      bumpers: {
        vjoueur_alice_123: {
          NAME: 'Alice', IS_VIRTUAL: true, IS_VPLAYER: true, TEAM: 'red',
          CONNECTED: true, ANSWER_COLOR: 'BLUE', TIME: 0,
        },
      },
      teams: { red: { NAME: 'Les Rouges', COLOR: [239, 68, 68] } },
    })
    useGame.mockReturnValue(mock)

    render(<VPlayerPage />)

    const lastCallProps = PlayerDisplay.mock.calls.at(-1)[0]
    expect(lastCallProps.playerNameColor).toBe('rgb(239,68,68)')
    expect(lastCallProps.teamColor).toBe('rgb(239,68,68)')
  })

  it('retombe sur ANSWER_COLOR seulement si le VJoueur n\'a pas d\'équipe assignée', () => {
    const mock = makeGameMock({
      bumpers: {
        vjoueur_alice_123: {
          NAME: 'Alice', IS_VIRTUAL: true, IS_VPLAYER: true, TEAM: '',
          CONNECTED: true, ANSWER_COLOR: 'BLUE', TIME: 0,
        },
      },
      teams: {},
    })
    useGame.mockReturnValue(mock)

    render(<VPlayerPage />)

    const lastCallProps = PlayerDisplay.mock.calls.at(-1)[0]
    expect(lastCallProps.playerNameColor).toBe('#3b82f6')
  })

  it("retourne null (pas de couleur) si le VJoueur n'a ni équipe assignée ni jamais répondu à une QCM", () => {
    const mock = makeGameMock({
      bumpers: {
        vjoueur_alice_123: {
          NAME: 'Alice', IS_VIRTUAL: true, IS_VPLAYER: true, TEAM: '',
          CONNECTED: true, TIME: 0,
        },
      },
      teams: {},
    })
    useGame.mockReturnValue(mock)

    render(<VPlayerPage />)

    const lastCallProps = PlayerDisplay.mock.calls.at(-1)[0]
    expect(lastCallProps.playerNameColor).toBeNull()
    expect(lastCallProps.teamColor).toBeNull()
  })

  it("suit un changement d'équipe en cours de partie (réassignation admin) sans rester figé sur l'ancienne couleur ni sur ANSWER_COLOR", () => {
    const baseBumper = {
      NAME: 'Alice', IS_VIRTUAL: true, IS_VPLAYER: true,
      CONNECTED: true, ANSWER_COLOR: 'BLUE', TIME: 0,
    }
    const teamsBoth = {
      red: { NAME: 'Les Rouges', COLOR: [239, 68, 68] },
      blue: { NAME: 'Les Bleus', COLOR: [59, 130, 246] },
    }

    useGame.mockReturnValue(makeGameMock({
      bumpers: { vjoueur_alice_123: { ...baseBumper, TEAM: 'red' } },
      teams: teamsBoth,
    }))
    const { rerender } = render(<VPlayerPage />)

    expect(PlayerDisplay.mock.calls.at(-1)[0].playerNameColor).toBe('rgb(239,68,68)')

    // L'admin réassigne Alice à l'équipe bleue en cours de partie — ANSWER_COLOR
    // (toujours BLUE côté backend, jamais réinitialisé) ne doit jamais être
    // confondu avec ce changement d'équipe légitime.
    useGame.mockReturnValue(makeGameMock({
      bumpers: { vjoueur_alice_123: { ...baseBumper, TEAM: 'blue' } },
      teams: teamsBoth,
    }))
    rerender(<VPlayerPage />)

    const lastCallProps = PlayerDisplay.mock.calls.at(-1)[0]
    expect(lastCallProps.playerNameColor).toBe('rgb(59,130,246)')
    expect(lastCallProps.teamColor).toBe('rgb(59,130,246)')
  })

  it('reste sur team.COLOR à travers une reconnexion (perte réseau puis retour) malgré ANSWER_COLOR déjà positionné', () => {
    const teams = { red: { NAME: 'Les Rouges', COLOR: [239, 68, 68] } }

    useGame.mockReturnValue(makeGameMock({
      bumpers: {
        vjoueur_alice_123: {
          NAME: 'Alice', IS_VIRTUAL: true, IS_VPLAYER: true, TEAM: 'red',
          CONNECTED: true, ANSWER_COLOR: 'GREEN', TIME: 0,
        },
      },
      teams,
    }))
    const { rerender } = render(<VPlayerPage />)
    expect(PlayerDisplay.mock.calls.at(-1)[0].playerNameColor).toBe('rgb(239,68,68)')

    // Perte réseau : le bumper reste dans l'état (CONNECTED bascule à false),
    // ANSWER_COLOR inchangé — le badge doit rester sur la couleur d'équipe.
    useGame.mockReturnValue(makeGameMock({
      bumpers: {
        vjoueur_alice_123: {
          NAME: 'Alice', IS_VIRTUAL: true, IS_VPLAYER: true, TEAM: 'red',
          CONNECTED: false, ANSWER_COLOR: 'GREEN', TIME: 0,
        },
      },
      teams,
    }))
    rerender(<VPlayerPage />)
    expect(PlayerDisplay.mock.calls.at(-1)[0].playerNameColor).toBe('rgb(239,68,68)')

    // Retour réseau : reconnexion, toujours la couleur d'équipe.
    useGame.mockReturnValue(makeGameMock({
      bumpers: {
        vjoueur_alice_123: {
          NAME: 'Alice', IS_VIRTUAL: true, IS_VPLAYER: true, TEAM: 'red',
          CONNECTED: true, ANSWER_COLOR: 'GREEN', TIME: 0,
        },
      },
      teams,
    }))
    rerender(<VPlayerPage />)
    expect(PlayerDisplay.mock.calls.at(-1)[0].playerNameColor).toBe('rgb(239,68,68)')
  })

  it("reste sur team.COLOR à travers plusieurs réponses QCM successives (ANSWER_COLOR change à chaque question)", () => {
    const teams = { red: { NAME: 'Les Rouges', COLOR: [239, 68, 68] } }
    const bumperWith = (answerColor) => ({
      vjoueur_alice_123: {
        NAME: 'Alice', IS_VIRTUAL: true, IS_VPLAYER: true, TEAM: 'red',
        CONNECTED: true, ANSWER_COLOR: answerColor, TIME: 0,
      },
    })

    useGame.mockReturnValue(makeGameMock({ bumpers: bumperWith('RED'), teams }))
    const { rerender } = render(<VPlayerPage />)
    expect(PlayerDisplay.mock.calls.at(-1)[0].playerNameColor).toBe('rgb(239,68,68)')

    useGame.mockReturnValue(makeGameMock({ bumpers: bumperWith('YELLOW'), teams }))
    rerender(<VPlayerPage />)
    expect(PlayerDisplay.mock.calls.at(-1)[0].playerNameColor).toBe('rgb(239,68,68)')

    useGame.mockReturnValue(makeGameMock({ bumpers: bumperWith('GREEN'), teams }))
    rerender(<VPlayerPage />)
    expect(PlayerDisplay.mock.calls.at(-1)[0].playerNameColor).toBe('rgb(239,68,68)')
  })
})

// ---------------------------------------------------------------------------
// Régression bug QUALIF cycle 7 (#187) : PlayerDisplay ne recevait AUCUN
// identifiant de bumper (seulement playerName/teamName) — flipMemoryCard ne
// pouvait donc jamais transmettre payload.ID au serveur, dont la résolution
// de secours (clientID) ne correspond PAS à la clé du bumper pour un VJoueur
// (cmd/server/main.go, resolveFlipMemoryCardBumper). Tout flip MEMORY depuis
// /player était donc ignoré, quelle que soit l'équipe active. Fix :
// VPlayerPage transmet désormais playerId={bumper?.id} — même clé de roster
// que celle utilisée pour ID dans ARDOISE_INPUT/VPLAYER_QCM_ANSWER/BUTTON.
// ---------------------------------------------------------------------------
describe('VPlayerPage — playerId transmis à PlayerDisplay (#187, régression bug QUALIF cycle 7)', () => {
  beforeEach(() => {
    mockNavigate.mockClear()
    vi.stubGlobal('localStorage', createLocalStorageMock())
    localStorage.setItem('vplayer_name', 'Alice')
    localStorage.setItem('vplayer_session', '1234567890')
    useGame.mockReset()
    PlayerDisplay.mockClear()
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('🔴 playerId = la clé du bumper dans le roster (bumper.id), jamais absent/undefined', () => {
    useGame.mockReturnValue(makeGameMock({
      bumpers: {
        vjoueur_alice_123: { NAME: 'Alice', IS_VIRTUAL: true, IS_VPLAYER: true, TEAM: 'red', CONNECTED: true, TIME: 0 },
      },
      teams: { red: { NAME: 'Les Rouges', COLOR: [239, 68, 68] } },
    }))

    render(<VPlayerPage />)

    const lastCallProps = PlayerDisplay.mock.calls.at(-1)[0]
    expect(lastCallProps.playerId).toBe('vjoueur_alice_123')
  })

  it('playerId suit une reconnexion (même bumper, roster mis à jour)', () => {
    const bumperState = (connected) => ({
      vjoueur_alice_123: { NAME: 'Alice', IS_VIRTUAL: true, IS_VPLAYER: true, TEAM: 'red', CONNECTED: connected, TIME: 0 },
    })
    useGame.mockReturnValue(makeGameMock({
      bumpers: bumperState(true),
      teams: { red: { NAME: 'Les Rouges', COLOR: [239, 68, 68] } },
    }))
    const { rerender } = render(<VPlayerPage />)
    expect(PlayerDisplay.mock.calls.at(-1)[0].playerId).toBe('vjoueur_alice_123')

    useGame.mockReturnValue(makeGameMock({
      bumpers: bumperState(false),
      teams: { red: { NAME: 'Les Rouges', COLOR: [239, 68, 68] } },
    }))
    rerender(<VPlayerPage />)
    expect(PlayerDisplay.mock.calls.at(-1)[0].playerId).toBe('vjoueur_alice_123')
  })
})
