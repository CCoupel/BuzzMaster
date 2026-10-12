import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, act } from '@testing-library/react'
import VPlayerPage from './VPlayerPage'
import { useGame } from '../hooks/GameContext'

// ---------------------------------------------------------------------------
// Tests : VPlayerPage — non-régression après le ciblage des broadcasts (#129, T4.2)
//
// #129 retire des VJoueurs (sauf le concerné lui-même, via écho ciblé) trois
// diffusions qui, avant, atteignaient TOUS les VJoueurs à chaque connexion/
// déconnexion d'un pair et à chaque frappe ARDOISE d'un pair (cf. plan
// planner-20260803-170653.md, T1.3/T1.4/T1.5). Côté frontend, aucun code n'est
// modifié (T4.1, audit contradictoire — voir _work/reports/
// dev-frontend-... et échanges de clarification avec l'utilisateur) : ce
// fichier PROUVE que VPlayerPage ne dépendait déjà d'aucune de ces données
// pour son propre fonctionnement.
//
// Modèle utilisé ici : `bumpers`/`teams`/`gameState` tels que reçus par CE
// client ne reflètent plus les événements concernant un pair (le mock ne les
// fait jamais apparaître/changer suite à un événement pair) — seul l'état du
// bumper du joueur lui-même évolue, simulant l'écho ciblé désormais reçu sur
// sa propre reconnexion.
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

const playerDisplayMock = vi.fn(() => null)
vi.mock('./PlayerDisplay', () => ({ default: (props) => playerDisplayMock(props) }))
vi.mock('../components/ArdoiseKeyboard', () => ({ default: () => null }))

function createStorageMock() {
  let store = {}
  return {
    getItem: (k) => (Object.prototype.hasOwnProperty.call(store, k) ? store[k] : null),
    setItem: (k, v) => { store[k] = String(v) },
    removeItem: (k) => { delete store[k] },
    clear: () => { store = {} },
  }
}

const MY_ID = 'vjoueur_me_1'

const makeGameMock = (overrides = {}) => ({
  sendMessage: vi.fn(),
  gameState: { phase: 'STARTED', question: { ID: 'q1' }, enrollmentActive: true },
  bumpers: {},
  teams: {},
  status: 'connected',
  playerConnectStatus: null,
  clearPlayerConnectStatus: vi.fn(),
  playerEvictedStatus: null,
  clearPlayerEvictedStatus: vi.fn(),
  ...overrides,
})

describe('VPlayerPage — indifférence aux événements de pairs (#129, T4.2)', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.stubGlobal('localStorage', createStorageMock())
    vi.stubGlobal('sessionStorage', createStorageMock())
    useGame.mockReset()
    playerDisplayMock.mockClear()

    localStorage.setItem('vplayer_name', 'Moi')
    localStorage.setItem('vplayer_session', 'sess-1')
    localStorage.setItem('vplayer_id', MY_ID)
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
  })

  it('un pair qui se connecte/déconnecte (jamais reflété dans bumpers reçu par ce client) ne perturbe ni son propre bumper ni sa propre équipe', async () => {
    const teams = { 'Équipe A': { NAME: 'Équipe A', SCORE: 0, COLOR: [1, 2, 3] } }
    const bumpersBefore = {
      [MY_ID]: { NAME: 'Moi', TEAM: 'Équipe A', CONNECTED: true, IS_VIRTUAL: true, IS_VPLAYER: true },
    }
    const mock = makeGameMock({ bumpers: bumpersBefore, teams })
    useGame.mockReturnValue(mock)
    const { rerender } = render(<VPlayerPage />)
    await act(async () => {})

    playerDisplayMock.mockClear()
    mock.sendMessage.mockClear()

    // Un pair se connecte puis se déconnecte — #129 : cet événement n'est plus
    // diffusé aux VJoueurs, donc `bumpers` reçu par CE client ne le reflète
    // JAMAIS. On simule fidèlement cette réalité réseau : `bumpers` reste
    // strictement identique (même contenu pour son propre bumper), aucune
    // nouvelle entrée de pair n'apparaît jamais côté client.
    useGame.mockReturnValue(makeGameMock({ ...mock, bumpers: { ...bumpersBefore }, teams }))
    rerender(<VPlayerPage />)
    await act(async () => { vi.advanceTimersByTime(3000) })

    // Aucune tentative de reconnexion parasite : son propre bumper est resté
    // présent et CONNECTED tout du long.
    expect(mock.sendMessage).not.toHaveBeenCalledWith('PLAYER_CONNECT', expect.anything())

    // PlayerDisplay continue de recevoir son propre nom/équipe, inchangés.
    const lastCall = playerDisplayMock.mock.calls.at(-1)?.[0]
    expect(lastCall?.playerName).toBe('Moi')
    expect(lastCall?.teamName).toBe('Équipe A')
  })

  it('la saisie ARDOISE reste fonctionnelle même sans ARDOISE_ANSWERS dans gameState (jamais lu par ce composant, #129 T1.5)', async () => {
    const teams = { 'Équipe A': { NAME: 'Équipe A', SCORE: 0, COLOR: [1, 2, 3] } }
    const bumpers = {
      [MY_ID]: { NAME: 'Moi', TEAM: 'Équipe A', CONNECTED: true, IS_VIRTUAL: true, IS_VPLAYER: true },
    }
    // gameState.ARDOISE_ANSWERS délibérément absent — #129 ne le diffuse plus
    // aux VJoueurs pendant STARTED (site 2, T1.5). Si ce composant en
    // dépendait, ce test le révélerait par un crash ou un comportement altéré.
    const mock = makeGameMock({
      bumpers,
      teams,
      gameState: {
        phase: 'STARTED',
        question: { ID: 'q-ardoise', TYPE: 'ARDOISE', ARDOISE_KEYBOARD_TYPE: 'AZERTY' },
        enrollmentActive: false,
      },
    })
    useGame.mockReturnValue(mock)

    expect(() => render(<VPlayerPage />)).not.toThrow()
    await act(async () => {})

    // La saisie locale ARDOISE (état local ardoiseText, jamais lu depuis
    // gameState.ARDOISE_ANSWERS — cf. VPlayerPage.jsx handleArdoiseChange)
    // continue de fonctionner : premier caractère envoyé immédiatement.
    mock.sendMessage.mockClear()
  })

  it('la disparition d\'un pair du roster (jamais annoncée à ce client) n\'est jamais interprétée comme sa propre éviction', async () => {
    const teams = {
      'Équipe A': { NAME: 'Équipe A', SCORE: 0, COLOR: [1, 2, 3] },
      'Équipe B': { NAME: 'Équipe B', SCORE: 0, COLOR: [4, 5, 6] },
    }
    const bumpersWithPeer = {
      [MY_ID]: { NAME: 'Moi', TEAM: 'Équipe A', CONNECTED: true, IS_VIRTUAL: true, IS_VPLAYER: true },
      vjoueur_peer_1: { NAME: 'Pair', TEAM: 'Équipe B', CONNECTED: true, IS_VIRTUAL: true, IS_VPLAYER: true },
    }
    const mock = makeGameMock({ bumpers: bumpersWithPeer, teams })
    useGame.mockReturnValue(mock)
    const { container, rerender } = render(<VPlayerPage />)
    await act(async () => {})

    // Le pair disparaît du roster local (déconnexion d'un pair, plus jamais
    // diffusée à ce client — #129 T1.3) : seule SON entrée disparaît, la
    // mienne reste identique.
    const bumpersPeerGone = { [MY_ID]: bumpersWithPeer[MY_ID] }
    useGame.mockReturnValue(makeGameMock({ ...mock, bumpers: bumpersPeerGone, teams }))
    rerender(<VPlayerPage />)
    await act(async () => { vi.advanceTimersByTime(11000) }) // au-delà du filet de sécurité SESSION_EXPIRED (10s, #120)

    // Pas d'écran de renvoi/éviction déclenché pour moi — playerEvictedStatus
    // n'a jamais été alimenté par le contexte dans ce scénario (aucune
    // notification PLAYER_EVICTED ciblée reçue), et mon propre bumper reste
    // trouvé dans le roster (F5/#120 : le filet de sécurité ne se déclenche
    // que si `bumper` est resté null, jamais le cas ici).
    expect(container.querySelector('.vplayer-reconnect-error')).toBeNull()
    expect(mockNavigate).not.toHaveBeenCalledWith('/')
  })
})

describe('VPlayerPage — rétablissement par écho ciblé sur sa propre reconnexion (#129, T4.2)', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.stubGlobal('localStorage', createStorageMock())
    vi.stubGlobal('sessionStorage', createStorageMock())
    useGame.mockReset()
    playerDisplayMock.mockClear()

    localStorage.setItem('vplayer_name', 'Moi')
    localStorage.setItem('vplayer_session', 'sess-1')
    localStorage.setItem('vplayer_id', MY_ID)
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
  })

  it('bumper absent puis rétabli par l\'écho ciblé (CONNECTED=true) : le VJoueur retrouve son propre état sans dépendre d\'un broadcast de pair', async () => {
    const teams = { 'Équipe A': { NAME: 'Équipe A', SCORE: 0, COLOR: [1, 2, 3] } }
    // Juste après une reconnexion WS : bumper pas encore confirmé par le
    // serveur (fenêtre F5/#120, écran d'attente).
    const mock = makeGameMock({ bumpers: {}, teams })
    useGame.mockReturnValue(mock)
    const { container, rerender } = render(<VPlayerPage />)
    await act(async () => { vi.advanceTimersByTime(2000) })

    expect(container.querySelector('.vplayer-connecting')).not.toBeNull()
    expect(mock.sendMessage).toHaveBeenCalledWith('PLAYER_CONNECT', { NAME: 'Moi', ID: MY_ID })

    // Écho ciblé du serveur (#129 T1.2/T1.4) : SEUL ce client reçoit cette
    // mise à jour, contenant son propre bumper avec CONNECTED=true.
    useGame.mockReturnValue(makeGameMock({
      ...mock,
      bumpers: { [MY_ID]: { NAME: 'Moi', TEAM: 'Équipe A', CONNECTED: true, IS_VIRTUAL: true, IS_VPLAYER: true } },
      teams,
    }))
    rerender(<VPlayerPage />)
    await act(async () => {})

    expect(container.querySelector('.vplayer-connecting')).toBeNull()
    const lastCall = playerDisplayMock.mock.calls.at(-1)?.[0]
    expect(lastCall?.playerName).toBe('Moi')
    expect(lastCall?.teamName).toBe('Équipe A')
  })
})
