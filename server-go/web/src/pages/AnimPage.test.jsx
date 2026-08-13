import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import AnimPage from './AnimPage'
import { useGame } from '../hooks/GameContext'
import { useCategories } from '../hooks/useCategories'

// ---------------------------------------------------------------------------
// AnimPage — page animateur (#155/#156, F4/F5/F6)
//
// Gabarit 3 zones : A (contexte) / B (conduite SPEEDY, AnimConductPanel,
// #156/F5) / C (équipes, AnimTeamCard enrichie — ordre de buzz, rang, temps
// de réaction, bouton de crédit, #156/F6). Voir plan
// _work/reports/plan-20260813-094321.md §4.
// AnimConductPanel a sa propre couverture exhaustive par phase
// (AnimConductPanel.test.jsx) — ici on vérifie surtout le CÂBLAGE (les
// bonnes props/callbacks lui arrivent) et les particularités propres à
// AnimPage (dérivation de canStart/canReveal/isPlaying, calcul du crédit).
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
      gameTime: 0,
      question: null,
      ...overrides.gameState,
    },
    teams: {},
    bumpers: {},
    nextQuestion: null,
    startGame: vi.fn(),
    stopGame: vi.fn(),
    pauseGame: vi.fn(),
    continueGame: vi.fn(),
    revealAnswer: vi.fn(),
    selectQuestion: vi.fn(),
    setTeamPoints: vi.fn(),
    setBumperPoints: vi.fn(),
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
    expect(screen.getByText('Suivante')).toBeInTheDocument()
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
// Zone B — conduite SPEEDY (#156/F5) : câblage vers AnimConductPanel
// ---------------------------------------------------------------------------

describe('AnimPage — Zone B (conduite, #156/F5)', () => {
  it('dérive isPlaying/canStart/canReveal comme /admin (GamePage.jsx:373-378)', () => {
    // READY → canStart, LANCER visible
    useGame.mockReturnValue(makeGameMock({ gameState: { phase: 'READY', question: { ID: '1' } } }))
    const { rerender } = render(<AnimPage />)
    expect(screen.getByText('LANCER')).toBeInTheDocument()

    // STARTED → isPlaying, PAUSE+STOP visibles
    useGame.mockReturnValue(makeGameMock({ gameState: { phase: 'STARTED', question: { ID: '1' } } }))
    rerender(<AnimPage />)
    expect(screen.getByText('PAUSE')).toBeInTheDocument()
    expect(screen.getByText('STOP')).toBeInTheDocument()

    // STOPPED avec question.STATUS === 'STOPPED' → canReveal, RÉPONSE visible
    useGame.mockReturnValue(makeGameMock({ gameState: { phase: 'STOPPED', question: { ID: '1', STATUS: 'STOPPED' } } }))
    rerender(<AnimPage />)
    expect(screen.getByText('RÉPONSE')).toBeInTheDocument()

    // STOPPED sans question.STATUS === 'STOPPED' (idle) → pas de canReveal
    useGame.mockReturnValue(makeGameMock({ gameState: { phase: 'STOPPED', question: { ID: '1', STATUS: 'AVAILABLE' } } }))
    rerender(<AnimPage />)
    expect(screen.queryByText('RÉPONSE')).not.toBeInTheDocument()
  })

  it('LANCER envoie startGame avec TIME/POINTS de la question courante', () => {
    const props = makeGameMock({
      gameState: { phase: 'READY', question: { ID: '1', TIME: '45', POINTS: '3' } },
    })
    useGame.mockReturnValue(props)
    render(<AnimPage />)
    screen.getByText('LANCER').click()
    expect(props.startGame).toHaveBeenCalledWith(45, 3)
  })

  it('LANCER replie sur 30s/1pt si TIME/POINTS sont absents de la question', () => {
    const props = makeGameMock({ gameState: { phase: 'READY', question: { ID: '1' } } })
    useGame.mockReturnValue(props)
    render(<AnimPage />)
    screen.getByText('LANCER').click()
    expect(props.startGame).toHaveBeenCalledWith(30, 1)
  })

  it('l\'enchaînement envoie selectQuestion avec l\'ID de NEXT_QUESTION', () => {
    const props = makeGameMock({
      gameState: { phase: 'REVEALED', question: { ID: '1' } },
      nextQuestion: { ID: '9', TYPE: 'SPEEDY' },
    })
    useGame.mockReturnValue(props)
    render(<AnimPage />)
    screen.getByText('À suivre').click()
    expect(props.selectQuestion).toHaveBeenCalledWith('9')
  })

  it('STOP/PAUSE/CONTINUER/RÉPONSE appellent les actions useGame() correspondantes', () => {
    const stopProps = makeGameMock({ gameState: { phase: 'STARTED', question: { ID: '1' } } })
    useGame.mockReturnValue(stopProps)
    const { rerender } = render(<AnimPage />)
    screen.getByText('STOP').click()
    expect(stopProps.stopGame).toHaveBeenCalledTimes(1)
    screen.getByText('PAUSE').click()
    expect(stopProps.pauseGame).toHaveBeenCalledTimes(1)

    const revealProps = makeGameMock({ gameState: { phase: 'STOPPED', question: { ID: '1', STATUS: 'STOPPED' } } })
    useGame.mockReturnValue(revealProps)
    rerender(<AnimPage />)
    screen.getByText('RÉPONSE').click()
    expect(revealProps.revealAnswer).toHaveBeenCalledTimes(1)
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
// Zone C — ordre de buzz, rang, temps de réaction (#156/F6)
// ---------------------------------------------------------------------------

describe('AnimPage — Zone C, ordre de buzz (#156/F6)', () => {
  function buzzGameMock(phase) {
    return makeGameMock({
      gameState: { phase, gameTime: 1_000_000, question: { ID: '1', TYPE: 'SPEEDY' } },
      teams: {
        'Lente': { COLOR: [0, 0, 255], SCORE: 0, TIME: 3_000_000 },
        'Rapide': { COLOR: [255, 0, 0], SCORE: 0, TIME: 1_500_000 },
        'PasBuzze': { COLOR: [0, 255, 0], SCORE: 0, TIME: 0 },
      },
      bumpers: {
        m1: { TEAM: 'Lente' },
        m2: { TEAM: 'Rapide' },
        m3: { TEAM: 'PasBuzze' },
      },
    })
  }

  it('réordonne les équipes par TIME croissant pendant STARTED', () => {
    useGame.mockReturnValue(buzzGameMock('STARTED'))
    const { container } = render(<AnimPage />)
    const names = Array.from(container.querySelectorAll('.anim-team-card-name')).map(el => el.textContent)
    expect(names).toEqual(['Rapide', 'Lente', 'PasBuzze'])
  })

  it('affiche le badge de rang (🏆) sur la première équipe en STARTED', () => {
    useGame.mockReturnValue(buzzGameMock('STARTED'))
    render(<AnimPage />)
    expect(screen.getByText('🏆')).toBeInTheDocument()
  })

  it('masque le badge de rang en STOPPED (mais garde le tri)', () => {
    useGame.mockReturnValue(buzzGameMock('STOPPED'))
    const { container } = render(<AnimPage />)
    expect(screen.queryByText('🏆')).not.toBeInTheDocument()
    const names = Array.from(container.querySelectorAll('.anim-team-card-name')).map(el => el.textContent)
    expect(names).toEqual(['Rapide', 'Lente', 'PasBuzze'])
  })

  it('affiche le temps de réaction formaté pour une équipe ayant buzzé', () => {
    useGame.mockReturnValue(buzzGameMock('STARTED'))
    render(<AnimPage />)
    // Rapide : TIME 1_500_000, gameTime 1_000_000 → 0.500s
    expect(screen.getByText('0.500s')).toBeInTheDocument()
  })

  it('hors des phases actives (READY), pas de tri ni de temps de réaction', () => {
    useGame.mockReturnValue(buzzGameMock('READY'))
    const { container } = render(<AnimPage />)
    const names = Array.from(container.querySelectorAll('.anim-team-card-name')).map(el => el.textContent)
    // Ordre d'objet d'origine (Lente, Rapide, PasBuzze), pas de tri
    expect(names).toEqual(['Lente', 'Rapide', 'PasBuzze'])
    expect(screen.queryByText('0.500s')).not.toBeInTheDocument()
  })
})

// ---------------------------------------------------------------------------
// Zone C — crédit (#156/F6)
// ---------------------------------------------------------------------------

describe('AnimPage — Zone C, crédit (#156/F6)', () => {
  it('aucun bouton de crédit avant l\'arrêt de la question (ex: STARTED)', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: { phase: 'STARTED', question: { ID: '1', POINTS: '5' } },
      teams: { 'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0 } },
      bumpers: { m1: { TEAM: 'Les Rouges' } },
    }))
    render(<AnimPage />)
    expect(screen.queryByText(/pts/)).not.toBeInTheDocument()
  })

  it('STOPPED et REVEALED : bouton de crédit visible avec le montant de la question', () => {
    ;['STOPPED', 'REVEALED'].forEach(phase => {
      useGame.mockReturnValue(makeGameMock({
        gameState: { phase, question: { ID: '1', POINTS: '5' } },
        teams: { 'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0 } },
        bumpers: { m1: { TEAM: 'Les Rouges' } },
      }))
      const { unmount } = render(<AnimPage />)
      expect(screen.getByText('+5 pts')).toBeInTheDocument()
      unmount()
    })
  })

  it('POINTS_TARGET absent (PLAYER) : crédite le bumper le plus rapide de l\'équipe', () => {
    const props = makeGameMock({
      gameState: { phase: 'STOPPED', question: { ID: '1', POINTS: '5' } },
      teams: { 'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0, TIME: 2000 } },
      bumpers: {
        slow: { TEAM: 'Les Rouges', TIME: 5000 },
        fast: { TEAM: 'Les Rouges', TIME: 2000 },
      },
    })
    useGame.mockReturnValue(props)
    render(<AnimPage />)
    screen.getByText('+5 pts').click()
    expect(props.setBumperPoints).toHaveBeenCalledWith('fast', 5)
    expect(props.setTeamPoints).not.toHaveBeenCalled()
  })

  it('POINTS_TARGET=TEAM : crédite l\'équipe directement', () => {
    const props = makeGameMock({
      gameState: { phase: 'STOPPED', question: { ID: '1', POINTS: '5', POINTS_TARGET: 'TEAM' } },
      teams: { 'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0 } },
      bumpers: { m1: { TEAM: 'Les Rouges', TIME: 2000 } },
    })
    useGame.mockReturnValue(props)
    render(<AnimPage />)
    screen.getByText('+5 pts').click()
    expect(props.setTeamPoints).toHaveBeenCalledWith('Les Rouges', 5)
    expect(props.setBumperPoints).not.toHaveBeenCalled()
  })

  it('replie sur 1 point si question.POINTS est absent', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: { phase: 'STOPPED', question: { ID: '1' } },
      teams: { 'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0 } },
      bumpers: { m1: { TEAM: 'Les Rouges', TIME: 2000 } },
    }))
    render(<AnimPage />)
    expect(screen.getByText('+1 pts')).toBeInTheDocument()
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
