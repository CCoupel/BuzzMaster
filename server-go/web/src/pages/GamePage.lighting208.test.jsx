/**
 * Tests — GamePage : intégration du panneau de conduite en direct #208
 * (sélecteur ON/AUTO/OFF + Flash), déplacé depuis AmbiancePage.jsx le
 * 2026-09-07 (correction utilisateur — outil utilisé par la régie PENDANT
 * une partie, donc sur l'écran ouvert en séance).
 *
 * Test d'INTÉGRATION léger seulement : le composant lui-même
 * (`components/LightingModePanel.jsx`) est testé en profondeur dans
 * `LightingModePanel.test.jsx` (sélecteur tri-état, Flash indépendant,
 * bandeau d'avertissement, absence d'état optimiste, erreurs réseau).
 * Ici on vérifie seulement le CÂBLAGE : le composant réel est monté sur
 * GamePage (pas mocké, contrairement à Timer/QuestionPreview/TeamCard —
 * suit le pattern de GamePage.newGameButton215.test.jsx), et s'efface bien
 * quand l'éclairage n'est pas configuré.
 */
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import GamePage from './GamePage'

vi.mock('../hooks/GameContext', () => ({
  useGame: vi.fn(),
  GameProvider: ({ children }) => children,
}))

vi.mock('../components/Timer', () => ({
  default: ({ phase }) => <div data-testid="timer" data-phase={phase} />,
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
  CATEGORIES: {},
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

vi.mock('./GamePage.css', () => ({}))
// LightingModePanel n'est PAS mocké — c'est exactement ce que ce fichier vérifie.

import { useGame } from '../hooks/GameContext'

const makeGameMock = (overrides = {}) => ({
  gameState: {
    phase: 'STOPPED',
    question: { ID: '2', STATUS: 'STOPPED' },
    remote: 'GAME',
    timer: 0,
    totalTime: 30,
    MEMORY_PARTICIPATING_TEAMS: [],
    ...overrides.gameState,
  },
  teams: overrides.teams ?? {},
  bumpers: overrides.bumpers ?? {},
  questions: overrides.questions ?? {},
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
  newGame: vi.fn(),
  ...overrides,
})

function mockLightingStatus(body) {
  global.fetch = vi.fn(async (url) => {
    if (url === '/api/lighting/status') {
      return { ok: true, status: 200, json: async () => body }
    }
    throw new Error(`Route non mockée : ${url}`)
  })
}

describe('GamePage — panneau de conduite #208 (câblage)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    useGame.mockReturnValue(makeGameMock())
  })

  it('affiche le sélecteur ON/AUTO/OFF quand l\'éclairage est configuré', async () => {
    mockLightingStatus({ state: 'ok', mode: 'AUTO', flash: false })
    render(<GamePage />)

    const auto = await screen.findByRole('radio', { name: 'AUTO' })
    expect(auto).toHaveAttribute('aria-checked', 'true')
    expect(screen.getByRole('radio', { name: 'ON' })).toBeInTheDocument()
    expect(screen.getByRole('radio', { name: 'OFF' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Flash/ })).toBeInTheDocument()
  })

  it('ne montre aucun contrôle d\'éclairage quand rien n\'est configuré (state disabled)', async () => {
    mockLightingStatus({ state: 'disabled', mode: 'AUTO', flash: false })
    render(<GamePage />)

    // Laisse le hook useLightingStatus terminer son fetch avant de conclure
    // à l'absence — sans quoi le test passerait aussi si le composant
    // n'avait simplement pas fini de charger (faux négatif masqué en faux
    // positif).
    await waitFor(() => expect(global.fetch).toHaveBeenCalledWith('/api/lighting/status'))
    expect(screen.queryByRole('radiogroup', { name: 'Éclairage général' })).toBeNull()
    expect(screen.queryByRole('button', { name: /Flash/ })).toBeNull()
  })
})
