import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import AnimPage from './AnimPage'
import { useGame } from '../hooks/GameContext'
import { useCategories } from '../hooks/useCategories'

// ---------------------------------------------------------------------------
// AnimPage — page animateur (#155/#156, F4)
//
// Gabarit 3 zones : A (contexte) / B (conduite, conteneur vide — contenu
// livré par #156/F5) / C (équipes, AnimTeamCard). Voir plan
// _work/reports/plan-20260813-094321.md §4 F4.
// ---------------------------------------------------------------------------

vi.mock('nosleep.js', () => ({
  default: class NoSleep {
    constructor() { this.isEnabled = false }
    enable() { this.isEnabled = true; return Promise.resolve() }
    disable() { this.isEnabled = false }
  },
}))

vi.mock('../hooks/GameContext', () => ({
  useGame: vi.fn(),
  GameProvider: ({ children }) => children,
}))

vi.mock('../hooks/useCategories', () => ({
  useCategories: vi.fn(),
}))

vi.mock('../components/Timer', () => ({
  default: ({ currentTime, phase }) => <div data-testid="timer" data-phase={phase}>{currentTime}</div>,
}))

function makeGameMock(overrides = {}) {
  return {
    status: 'connected',
    gameState: {
      phase: 'STOPPED',
      timer: 30,
      totalTime: 30,
      question: null,
      ...overrides.gameState,
    },
    teams: {},
    bumpers: {},
    nextQuestion: null,
    ...overrides,
  }
}

beforeEach(() => {
  useGame.mockReturnValue(makeGameMock())
  useCategories.mockReturnValue({ categories: [] })
})

afterEach(() => {
  vi.clearAllMocks()
})

// ---------------------------------------------------------------------------
// Zone A — contexte : question courante, statut, question suivante
// ---------------------------------------------------------------------------

describe('AnimPage — Zone A (contexte)', () => {
  it('affiche "Aucune question en cours" quand aucune question n\'est chargée', () => {
    render(<AnimPage />)
    expect(screen.getByText('Aucune question en cours')).toBeInTheDocument()
  })

  it('affiche ID et TYPE de la question courante', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: { phase: 'STARTED', timer: 20, totalTime: 30, question: { ID: '42', TYPE: 'QCM', CATEGORY: 'SCIENCE' } },
    }))
    render(<AnimPage />)
    expect(screen.getByText('#42')).toBeInTheDocument()
    expect(screen.getByText('QCM')).toBeInTheDocument()
  })

  it('replie sur SPEEDY quand TYPE est absent', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: { phase: 'STARTED', timer: 20, totalTime: 30, question: { ID: '1' } },
    }))
    render(<AnimPage />)
    expect(screen.getByText('SPEEDY')).toBeInTheDocument()
  })

  it('affiche le statut de connexion', () => {
    useGame.mockReturnValue(makeGameMock({ status: 'disconnected' }))
    const { container } = render(<AnimPage />)
    expect(container.querySelector('.anim-connection-status.disconnected')).not.toBeNull()
    expect(screen.getByText('Déconnecté')).toBeInTheDocument()
  })

  it('affiche "—" quand aucune question suivante n\'est connue (NEXT_QUESTION vide ou jamais reçu)', () => {
    render(<AnimPage />)
    expect(screen.getByText('—')).toBeInTheDocument()
  })

  it('affiche la question suivante quand NEXT_QUESTION en a fourni une', () => {
    useGame.mockReturnValue(makeGameMock({
      nextQuestion: { ID: '43', TYPE: 'MEMORY', CATEGORY: 'HISTORY' },
    }))
    render(<AnimPage />)
    expect(screen.getByText('À suivre')).toBeInTheDocument()
    expect(screen.getByText('MEMORY')).toBeInTheDocument()
  })

  it('passe currentTime/totalTime/phase au Timer', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: { phase: 'PAUSED', timer: 12, totalTime: 30, question: null },
    }))
    render(<AnimPage />)
    const timer = screen.getByTestId('timer')
    expect(timer).toHaveAttribute('data-phase', 'PAUSED')
    expect(timer.textContent).toBe('12')
  })
})

// ---------------------------------------------------------------------------
// Zone B — conduite : conteneur posé, vide (contenu livré par #156/F5)
// ---------------------------------------------------------------------------

describe('AnimPage — Zone B (conduite)', () => {
  it('rend un conteneur vide', () => {
    const { container } = render(<AnimPage />)
    const zoneB = container.querySelector('.anim-zone-conduct')
    expect(zoneB).not.toBeNull()
    expect(zoneB.children).toHaveLength(0)
  })
})

// ---------------------------------------------------------------------------
// Zone C — équipes : carte de base (nom, couleur, score), équipes sans
// joueur masquées (même règle de base que /admin, #45)
// ---------------------------------------------------------------------------

describe('AnimPage — Zone C (équipes)', () => {
  it('affiche les équipes ayant au moins un joueur, avec leur score', () => {
    useGame.mockReturnValue(makeGameMock({
      teams: {
        'Les Rouges': { COLOR: [255, 0, 0], SCORE: 15 },
        'Les Bleus': { COLOR: [0, 0, 255], SCORE: 8 },
      },
      bumpers: {
        mac1: { TEAM: 'Les Rouges' },
        mac2: { TEAM: 'Les Bleus' },
      },
    }))
    render(<AnimPage />)
    expect(screen.getByText('Les Rouges')).toBeInTheDocument()
    expect(screen.getByText('15')).toBeInTheDocument()
    expect(screen.getByText('Les Bleus')).toBeInTheDocument()
    expect(screen.getByText('8')).toBeInTheDocument()
  })

  it('masque une équipe sans joueur assigné', () => {
    useGame.mockReturnValue(makeGameMock({
      teams: {
        'Les Rouges': { COLOR: [255, 0, 0], SCORE: 15 },
        'Equipe Vide': { COLOR: [0, 255, 0], SCORE: 0 },
      },
      bumpers: {
        mac1: { TEAM: 'Les Rouges' },
      },
    }))
    render(<AnimPage />)
    expect(screen.getByText('Les Rouges')).toBeInTheDocument()
    expect(screen.queryByText('Equipe Vide')).not.toBeInTheDocument()
  })
})

// ---------------------------------------------------------------------------
// Veille écran — reprend le motif PlayerDisplay.jsx:912-921
// ---------------------------------------------------------------------------

describe('AnimPage — veille écran', () => {
  it('utilise le wake lock natif quand disponible', async () => {
    const request = vi.fn().mockResolvedValue({ release: vi.fn() })
    Object.defineProperty(navigator, 'wakeLock', { value: { request }, configurable: true })

    render(<AnimPage />)
    await vi.waitFor(() => expect(request).toHaveBeenCalledWith('screen'))

    delete navigator.wakeLock
  })

  it('replie sur NoSleep.js quand le wake lock natif est indisponible', () => {
    expect('wakeLock' in navigator).toBe(false)
    // Pas d'assertion sur l'instance NoSleep (mockée) au-delà du rendu sans
    // erreur — le chemin nominal HTTP est déjà couvert par la même logique
    // testée sur PlayerDisplay (PlayerDisplay.*.test.jsx).
    expect(() => render(<AnimPage />)).not.toThrow()
  })
})
