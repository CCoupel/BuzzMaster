/**
 * Tests for GamePage — blocage du START RAFALE (fail-closed, contrat
 * rafale.md §7.2), milestone v8.0.0 #107.
 *
 * Deux commits successifs :
 *   - SHA 1a742782 : liste blanche — seuls rafalePoolLevel 'ok'/'warning'
 *     autorisent le START ; tout le reste (blocking, chargement, erreur,
 *     catégorie non sélectionnée -> null) bloque par défaut ("fail closed").
 *   - SHA 75b0472c : régression introduite PAR ce fail-closed — la prop
 *     `difficulty` passée à <RafalePoolAlert> n'avait pas le même repli
 *     `|| 1` que le chip d'affichage juste au-dessus (RAFALE_DIFFICULTY est
 *     omitempty côté serveur, contrat §3.3). Une question RAFALE
 *     parfaitement valide (catégorie + difficulté par défaut 1, pool
 *     disponible) restait bloquée au START car RafalePoolAlert calculait
 *     `hasFilter=false` (difficulty=undefined) -> rafalePoolLevel=null ->
 *     bloqué par le fail-closed, alors que /anim démarrait sans problème.
 *
 * Fichier SÉPARÉ de GamePage.rafale.test.jsx (qui stubbe RafalePoolAlert —
 * pertinent pour tester le panneau lui-même, mais ce stub ne fait JAMAIS
 * appel à onLevelChange, donc rafalePoolLevel reste `null` dans TOUS ses
 * tests : il ne peut structurellement pas détecter ce bug, ni celui-ci ni
 * une régression future de la même famille). Ici, RafalePoolAlert est le
 * VRAI composant (déjà testé isolément par RafalePoolAlert.rafale.test.jsx
 * pour ses 3 états — pas dupliqué ici), avec un fetch mocké, pour exercer
 * la VRAIE chaîne : props reçues -> hasFilter -> niveau -> bouton START.
 */
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'

vi.mock('../hooks/GameContext', () => ({
  useGame: vi.fn(),
  GameProvider: ({ children }) => children,
}))

vi.mock('../hooks/useCategories', () => ({
  useCategories: vi.fn(() => ({ categories: [], loading: false, error: null, refetch: vi.fn() })),
}))

vi.mock('../components/Timer', () => ({
  default: () => <div data-testid="timer" />,
}))

vi.mock('../components/QuestionPreview', () => ({
  default: () => <div data-testid="question-preview" />,
}))

vi.mock('../components/TeamCard', () => ({
  default: ({ name }) => <div data-testid={`team-card-${name}`} />,
  OtaAllModal: () => null,
}))

vi.mock('../components/QuestionCard', () => ({
  default: ({ question, onClick }) => (
    <div data-testid={`question-card-${question.ID}`} onClick={() => onClick && onClick(question)} />
  ),
  CATEGORIES: {
    HISTORY: { label: 'Histoire', icon: '📜', color: '#eab308' },
  },
}))

vi.mock('../components/Card', () => ({
  default: ({ children, className, padding, variant, ...rest }) => (
    <div className={className} {...rest}>{children}</div>
  ),
}))

vi.mock('../components/Button', () => ({
  default: ({ children, onClick, disabled, variant, size, ...rest }) => (
    <button onClick={onClick} disabled={disabled} {...rest}>{children}</button>
  ),
}))

// PAS de mock de RafalePoolAlert ici — c'est délibérément le VRAI composant
// (voir en-tête de fichier).

vi.mock('./GamePage.css', () => ({}))

import { useGame } from '../hooks/GameContext'
import GamePage from './GamePage'

function jsonResponse(body, ok = true, status = 200) {
  return Promise.resolve({ ok, status, json: () => Promise.resolve(body) })
}

// ⚠️ `...overrides` doit être spread EN PREMIER : sinon `overrides.gameState`
// (objet PARTIEL, ex. { question: {...} } seul) écraserait entièrement la
// clé `gameState` fusionnée juste en dessous — un `overrides.gameState`
// sans `phase` ferait alors disparaître `phase: 'READY'`, cassant
// silencieusement `canStart(phase)` (bug réel rencontré en écrivant ce
// fichier : le bouton START restait désactivé pour la mauvaise raison,
// canStart=false au lieu de rafaleBlocked=true — piège à ne pas
// reproduire dans un futur test de cette page).
const makeGameMock = (overrides = {}) => ({
  ...overrides,
  gameState: {
    phase: 'READY', // canStart(phase) exige READY précisément
    question: { ID: '1', TYPE: 'RAFALE', STATUS: 'READY', CATEGORY: 'HISTORY', RAFALE_MODE: 'SOLO' },
    remote: 'GAME',
    timer: 0,
    totalTime: 120,
    MEMORY_PARTICIPATING_TEAMS: [],
    ...overrides.gameState,
  },
  teams: overrides.teams ?? { red: { name: 'red', ready: true } },
  bumpers: overrides.bumpers ?? {},
  questions: overrides.questions ?? {
    '1': { ID: '1', TYPE: 'RAFALE', STATUS: 'READY', ORDER: 1, CATEGORY: 'HISTORY' },
  },
  startGame: vi.fn(),
  stopGame: vi.fn(),
  pauseGame: vi.fn(),
  continueGame: vi.fn(),
  revealAnswer: vi.fn(),
  selectQuestion: vi.fn(),
  setRemoteDisplay: vi.fn(),
  setBumperPoints: vi.fn(),
  setTeamPoints: vi.fn(),
  forceReady: vi.fn(),
  simulateButton: vi.fn(),
  simulatePong: vi.fn(),
  sendMessage: vi.fn(),
  rafaleSetTeams: vi.fn(),
})

beforeEach(() => {
  vi.clearAllMocks()
})

describe('GamePage — RAFALE_DIFFICULTY absente (omitempty) : START ne doit PAS être bloqué à tort (bugfix SHA 75b0472c)', () => {
  it('CATEGORY définie, RAFALE_DIFFICULTY ABSENTE, pool disponible : le bouton START devient cliquable (rafalePoolLevel="ok", pas null)', async () => {
    global.fetch = vi.fn(() => jsonResponse({ AVAILABLE: 50, USED: 0, TOTAL: 50 }))
    useGame.mockReturnValue(makeGameMock({
      gameState: { question: { ID: '1', TYPE: 'RAFALE', STATUS: 'READY', CATEGORY: 'HISTORY', RAFALE_DIFFICULTY: undefined } },
    }))
    render(<GamePage />)

    // LE bug exact : sans le repli || 1, difficulty=undefined est transmis
    // à RafalePoolAlert, qui échoue sa garde hasFilter et n'appelle jamais
    // fetch — le repli présent doit produire un vrai appel réseau avec
    // difficulty=1.
    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledWith('/api/rafale/pool?category=HISTORY&difficulty=1')
    })

    const startBtn = await screen.findByText('START')
    await waitFor(() => {
      expect(startBtn.disabled).toBe(false)
    })
  })

  it('même scénario avec RAFALE_DIFFICULTY=0 (falsy mais explicitement écrit) : traité comme absent, même repli sur 1', async () => {
    global.fetch = vi.fn(() => jsonResponse({ AVAILABLE: 50, USED: 0, TOTAL: 50 }))
    useGame.mockReturnValue(makeGameMock({
      gameState: { question: { ID: '1', TYPE: 'RAFALE', STATUS: 'READY', CATEGORY: 'HISTORY', RAFALE_DIFFICULTY: 0 } },
    }))
    render(<GamePage />)

    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledWith('/api/rafale/pool?category=HISTORY&difficulty=1')
    })
  })
})

describe('GamePage — non-régression du fail-closed (SHA 1a742782) : catégorie réellement absente reste bloquant', () => {
  it('CATEGORY absente (jamais sélectionnée) : le START reste bloqué, quel que soit RAFALE_DIFFICULTY', async () => {
    global.fetch = vi.fn() // ne doit même pas être appelé (hasFilter déjà faux sur la catégorie)
    useGame.mockReturnValue(makeGameMock({
      gameState: { question: { ID: '1', TYPE: 'RAFALE', STATUS: 'READY', CATEGORY: '', RAFALE_DIFFICULTY: 2 } },
    }))
    render(<GamePage />)

    const startBtn = await screen.findByText('START')
    expect(startBtn.disabled).toBe(true)
    expect(global.fetch).not.toHaveBeenCalled()
  })

  it('CATEGORY et RAFALE_DIFFICULTY valides mais pool réellement VIDE (AVAILABLE=0) : START reste bloqué', async () => {
    global.fetch = vi.fn(() => jsonResponse({ AVAILABLE: 0, USED: 5, TOTAL: 5 }))
    useGame.mockReturnValue(makeGameMock({
      gameState: { question: { ID: '1', TYPE: 'RAFALE', STATUS: 'READY', CATEGORY: 'HISTORY', RAFALE_DIFFICULTY: undefined } },
    }))
    render(<GamePage />)

    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledWith('/api/rafale/pool?category=HISTORY&difficulty=1')
    })

    const startBtn = await screen.findByText('START')
    await waitFor(() => {
      expect(startBtn.title).toMatch(/RAFALE/) // tooltip de blocage présent
    })
    expect(startBtn.disabled).toBe(true)
  })
})

// ---------------------------------------------------------------------------
// rafaleTeamsBlocked (SHA 2eb12351, #199) — garde équipe en mode multi,
// indépendante de rafaleBlocked (pool) ci-dessus. Pool systématiquement mis
// à 'ok' (fetch AVAILABLE>0) dans ce bloc pour isoler CETTE garde : sans
// cela, un test qui bloque à tort sur le pool masquerait un rafaleTeamsBlocked
// resté à false par erreur (faux positif).
// ---------------------------------------------------------------------------

describe('GamePage — rafaleTeamsBlocked : garde équipe en mode multi (SHA 2eb12351, #199)', () => {
  it('mode multi (CHACUN_SON_TOUR), aucune équipe sélectionnée : START désactivé avec tooltip équipe', async () => {
    global.fetch = vi.fn(() => jsonResponse({ AVAILABLE: 50, USED: 0, TOTAL: 50 }))
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        question: { ID: '1', TYPE: 'RAFALE', STATUS: 'READY', CATEGORY: 'HISTORY', RAFALE_DIFFICULTY: 1, RAFALE_MODE: 'CHACUN_SON_TOUR' },
        RAFALE_PARTICIPATING_TEAMS: [],
      },
    }))
    render(<GamePage />)

    // Laisse le gate pool se résoudre à 'ok' d'abord (sinon rafaleBlocked
    // masquerait la cause réellement testée ici).
    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledWith('/api/rafale/pool?category=HISTORY&difficulty=1')
    })

    const startBtn = await screen.findByText('START')
    await waitFor(() => {
      expect(startBtn.disabled).toBe(true)
      expect(startBtn.title).toMatch(/equipe participante/i)
    })
  })

  it('mode multi (MAILLON_FAIBLE), une équipe sélectionnée : START redevient cliquable', async () => {
    global.fetch = vi.fn(() => jsonResponse({ AVAILABLE: 50, USED: 0, TOTAL: 50 }))
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        question: { ID: '1', TYPE: 'RAFALE', STATUS: 'READY', CATEGORY: 'HISTORY', RAFALE_DIFFICULTY: 1, RAFALE_MODE: 'MAILLON_FAIBLE' },
        RAFALE_PARTICIPATING_TEAMS: ['red'],
      },
    }))
    render(<GamePage />)

    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledWith('/api/rafale/pool?category=HISTORY&difficulty=1')
    })

    const startBtn = await screen.findByText('START')
    await waitFor(() => {
      expect(startBtn.disabled).toBe(false)
    })
  })

  it('mode SOLO, aucune équipe sélectionnée : la garde ne s\'applique pas, START reste cliquable', async () => {
    global.fetch = vi.fn(() => jsonResponse({ AVAILABLE: 50, USED: 0, TOTAL: 50 }))
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        question: { ID: '1', TYPE: 'RAFALE', STATUS: 'READY', CATEGORY: 'HISTORY', RAFALE_DIFFICULTY: 1, RAFALE_MODE: 'SOLO' },
        RAFALE_PARTICIPATING_TEAMS: [],
      },
    }))
    render(<GamePage />)

    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledWith('/api/rafale/pool?category=HISTORY&difficulty=1')
    })

    const startBtn = await screen.findByText('START')
    await waitFor(() => {
      expect(startBtn.disabled).toBe(false)
    })
  })

  it('RAFALE_MODE absent (omitempty côté serveur) : traité comme SOLO, même repli que rafaleIsSolo, START cliquable', async () => {
    global.fetch = vi.fn(() => jsonResponse({ AVAILABLE: 50, USED: 0, TOTAL: 50 }))
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        question: { ID: '1', TYPE: 'RAFALE', STATUS: 'READY', CATEGORY: 'HISTORY', RAFALE_DIFFICULTY: 1, RAFALE_MODE: undefined },
        RAFALE_PARTICIPATING_TEAMS: [],
      },
    }))
    render(<GamePage />)

    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledWith('/api/rafale/pool?category=HISTORY&difficulty=1')
    })

    const startBtn = await screen.findByText('START')
    await waitFor(() => {
      expect(startBtn.disabled).toBe(false)
    })
  })
})
