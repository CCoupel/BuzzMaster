import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import AnimPage from './AnimPage'
import { useGame } from '../hooks/GameContext'
import { useCategories } from '../hooks/useCategories'
import { calcQcmTeamAward } from '../utils/pointsAward'

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
    // MAJEUR-1 — creditPoints (CREDIT_POINTS) est l'équivalent serveur de
    // pointsInput sur /admin, PAS question.POINTS brut. Défaut à 0 comme
    // l'état initial réel de useWebSocket.js (avant tout CREDIT_POINTS reçu).
    creditPoints: 0,
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

  it('LANCER envoie startGame avec TIME de la question et creditPoints (MAJEUR-1), pas question.POINTS', () => {
    const props = makeGameMock({
      gameState: { phase: 'READY', question: { ID: '1', TIME: '45', POINTS: '3' } },
      creditPoints: 7, // ajusté côté admin, diverge délibérément de question.POINTS (3)
    })
    useGame.mockReturnValue(props)
    render(<AnimPage />)
    screen.getByText('LANCER').click()
    expect(props.startGame).toHaveBeenCalledWith(45, 7)
  })

  it('LANCER replie sur 30s/1pt si TIME est absent et creditPoints vaut 0', () => {
    const props = makeGameMock({ gameState: { phase: 'READY', question: { ID: '1' } }, creditPoints: 0 })
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
      creditPoints: 5,
      teams: { 'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0 } },
      bumpers: { m1: { TEAM: 'Les Rouges' } },
    }))
    render(<AnimPage />)
    expect(screen.queryByText(/pts/)).not.toBeInTheDocument()
  })

  it('STOPPED et REVEALED : bouton de crédit visible avec le montant de creditPoints', () => {
    ;['STOPPED', 'REVEALED'].forEach(phase => {
      useGame.mockReturnValue(makeGameMock({
        gameState: { phase, question: { ID: '1', POINTS: '5' } },
        creditPoints: 5,
        teams: { 'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0 } },
        bumpers: { m1: { TEAM: 'Les Rouges' } },
      }))
      const { unmount } = render(<AnimPage />)
      expect(screen.getByText('+5 pts')).toBeInTheDocument()
      unmount()
    })
  })

  // MAJEUR-1 — scénario exact de la revue de code : question sélectionnée à
  // 10 points, l'admin ajuste pointsInput à 20 sans resélectionner (donc
  // question.POINTS reste 10, seul creditPoints — rediffusé par
  // CREDIT_POINTS — reflète l'ajustement). /anim doit créditer 20, pas 10.
  it('crédite creditPoints (ajusté par l\'admin), jamais question.POINTS brut', () => {
    const props = makeGameMock({
      gameState: { phase: 'STOPPED', question: { ID: '1', POINTS: '10' } },
      creditPoints: 20,
      teams: { 'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0 } },
      bumpers: { m1: { TEAM: 'Les Rouges', TIME: 2000 } },
    })
    useGame.mockReturnValue(props)
    render(<AnimPage />)
    expect(screen.getByText('+20 pts')).toBeInTheDocument()
    expect(screen.queryByText('+10 pts')).not.toBeInTheDocument()

    screen.getByText('+20 pts').click()
    expect(props.setBumperPoints).toHaveBeenCalledWith('m1', 20)
  })

  it('POINTS_TARGET absent (PLAYER) : crédite le bumper le plus rapide de l\'équipe', () => {
    const props = makeGameMock({
      gameState: { phase: 'STOPPED', question: { ID: '1', POINTS: '5' } },
      creditPoints: 5,
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
      creditPoints: 5,
      teams: { 'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0 } },
      bumpers: { m1: { TEAM: 'Les Rouges', TIME: 2000 } },
    })
    useGame.mockReturnValue(props)
    render(<AnimPage />)
    screen.getByText('+5 pts').click()
    expect(props.setTeamPoints).toHaveBeenCalledWith('Les Rouges', 5)
    expect(props.setBumperPoints).not.toHaveBeenCalled()
  })

  it('replie sur 1 point si creditPoints vaut 0 (aucun CREDIT_POINTS encore reçu)', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: { phase: 'STOPPED', question: { ID: '1', POINTS: '5' } },
      creditPoints: 0,
      teams: { 'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0 } },
      bumpers: { m1: { TEAM: 'Les Rouges', TIME: 2000 } },
    }))
    render(<AnimPage />)
    expect(screen.getByText('+1 pts')).toBeInTheDocument()
  })
})

// ---------------------------------------------------------------------------
// Zone C — crédit QCM par équipe (#157/T3)
// ---------------------------------------------------------------------------

describe('AnimPage — crédit QCM par équipe (#157/T3)', () => {
  function qcmGameMock(phase, overrides = {}) {
    // Déstructure `gameState` séparément — un spread `...overrides` en
    // dernier écraserait entièrement le `gameState` fusionné ci-dessous
    // (phase/question perdus) si `overrides.gameState` ne contient qu'un
    // sous-ensemble de champs (ex: juste `qcmInvalidated`).
    const { gameState: gameStateOverrides, ...restOverrides } = overrides
    return makeGameMock({
      gameState: {
        phase,
        question: { ID: '1', TYPE: 'QCM', QCM_HINTS_ENABLED: true, QCM_CORRECT: 'RED', POINTS: '10' },
        ...gameStateOverrides,
      },
      creditPoints: 10,
      teams: {
        'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0 },
        'Les Bleus': { COLOR: [0, 0, 255], SCORE: 0 },
      },
      bumpers: {
        r1: { TEAM: 'Les Rouges', TIME: 1000, ANSWER_COLOR: 'RED', HINTS_AT_BUZZ: 1 },
        b1: { TEAM: 'Les Bleus', TIME: 1200, ANSWER_COLOR: 'RED', HINTS_AT_BUZZ: 0 },
      },
      ...restOverrides,
    })
  }

  it('chaque équipe affiche et crédite SON montant (pénalité de son buzzer)', () => {
    const props = qcmGameMock('STOPPED')
    useGame.mockReturnValue(props)
    render(<AnimPage />)

    // Les Rouges : 1 indice au buzz -> 10*0.67 = 7 pts
    expect(screen.getByText('+7 pts')).toBeInTheDocument()
    // Les Bleus : 0 indice au buzz -> 10 pts pleins
    expect(screen.getByText('+10 pts')).toBeInTheDocument()

    screen.getByText('+7 pts').click()
    expect(props.setBumperPoints).toHaveBeenCalledWith('r1', 7)
    screen.getByText('+10 pts').click()
    expect(props.setBumperPoints).toHaveBeenCalledWith('b1', 10)
  })

  it('équipe sans buzzer correct : replie sur la pénalité des indices courants (pas 0)', () => {
    const props = qcmGameMock('STOPPED', {
      gameState: { qcmInvalidated: ['GREEN'] }, // 1 indice courant invalidé -> pénalité 0.67
      bumpers: {
        r1: { TEAM: 'Les Rouges', TIME: 1000, ANSWER_COLOR: 'BLUE', HINTS_AT_BUZZ: 2 }, // mauvaise couleur
      },
      teams: { 'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0 } },
    })
    useGame.mockReturnValue(props)
    render(<AnimPage />)

    // Repli : pénalité des indices COURANTS (1 -> 0.67 -> 7), pas le
    // HINTS_AT_BUZZ du buzz (2 -> 0.33 -> 3)
    expect(screen.getByText('+7 pts')).toBeInTheDocument()
    expect(screen.queryByText('+3 pts')).not.toBeInTheDocument()
  })

  it('le montant est identique à celui que calcQcmTeamAward (la même règle que /admin) calculerait', () => {
    // 2 indices au buzz -> pénalité 0.33 -> 10*0.33 = 3.3 -> round 3
    const props = qcmGameMock('REVEALED', {
      bumpers: { r1: { TEAM: 'Les Rouges', TIME: 1000, ANSWER_COLOR: 'RED', HINTS_AT_BUZZ: 2 } },
      teams: { 'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0 } },
    })
    useGame.mockReturnValue(props)
    render(<AnimPage />)
    expect(screen.getByText('+3 pts')).toBeInTheDocument()
  })

  it('non-régression SPEEDY : montant unique identique pour toutes les équipes', () => {
    const props = makeGameMock({
      gameState: { phase: 'STOPPED', question: { ID: '1', TYPE: 'SPEEDY', POINTS: '10' } },
      creditPoints: 10,
      teams: {
        'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0 },
        'Les Bleus': { COLOR: [0, 0, 255], SCORE: 0 },
      },
      bumpers: {
        r1: { TEAM: 'Les Rouges', TIME: 1000 },
        b1: { TEAM: 'Les Bleus', TIME: 1200 },
      },
    })
    useGame.mockReturnValue(props)
    const { container } = render(<AnimPage />)
    const creditButtons = Array.from(container.querySelectorAll('.anim-team-credit-btn')).map(b => b.textContent)
    expect(creditButtons).toEqual(['+10 pts', '+10 pts'])
  })

  it('le crédit reste indisponible avant STOPPED/REVEALED en QCM aussi', () => {
    useGame.mockReturnValue(qcmGameMock('STARTED'))
    render(<AnimPage />)
    expect(screen.queryByText(/\+\d+ pts/)).not.toBeInTheDocument()
  })
})

// ---------------------------------------------------------------------------
// Zone C — réponse QCM (couleur, joueur, justesse) (#157/T4)
// ---------------------------------------------------------------------------

describe('AnimPage — réponse QCM en zone C (#157/T4)', () => {
  function qcmAnswerGameMock(phase) {
    return makeGameMock({
      gameState: {
        phase,
        question: { ID: '1', TYPE: 'QCM', QCM_HINTS_ENABLED: false, QCM_CORRECT: 'RED' },
      },
      teams: { 'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0 } },
      bumpers: { r1: { TEAM: 'Les Rouges', TIME: 1000, ANSWER_COLOR: 'RED', NAME: 'Alice' } },
    })
  }

  it('affiche la couleur choisie et le nom du joueur dès le buzz (avant STOPPED/REVEALED)', () => {
    useGame.mockReturnValue(qcmAnswerGameMock('STARTED'))
    const { container } = render(<AnimPage />)
    expect(screen.getByText('Alice')).toBeInTheDocument()
    expect(container.querySelector('.anim-team-qcm-color')).not.toBeNull()
  })

  it('n\'affiche PAS la justesse (✓/✗) avant REVEALED (décision D1)', () => {
    useGame.mockReturnValue(qcmAnswerGameMock('STARTED'))
    const { container } = render(<AnimPage />)
    expect(container.querySelector('.anim-team-qcm-correct')).toBeNull()

    useGame.mockReturnValue(qcmAnswerGameMock('STOPPED'))
    const { container: container2 } = render(<AnimPage />)
    expect(container2.querySelector('.anim-team-qcm-correct')).toBeNull()
  })

  it('affiche ✓ en REVEALED quand la couleur choisie est correcte', () => {
    useGame.mockReturnValue(qcmAnswerGameMock('REVEALED'))
    const { container } = render(<AnimPage />)
    const marker = container.querySelector('.anim-team-qcm-correct')
    expect(marker).not.toBeNull()
    expect(marker.textContent).toBe('✓')
    expect(marker.className).toContain('correct')
  })

  it('affiche ✗ en REVEALED quand la couleur choisie est incorrecte', () => {
    const props = makeGameMock({
      gameState: { phase: 'REVEALED', question: { ID: '1', TYPE: 'QCM', QCM_CORRECT: 'RED' } },
      teams: { 'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0 } },
      bumpers: { r1: { TEAM: 'Les Rouges', TIME: 1000, ANSWER_COLOR: 'BLUE', NAME: 'Bob' } },
    })
    useGame.mockReturnValue(props)
    const { container } = render(<AnimPage />)
    const marker = container.querySelector('.anim-team-qcm-correct')
    expect(marker.textContent).toBe('✗')
    expect(marker.className).toContain('incorrect')
  })

  it('n\'affiche rien tant que l\'équipe n\'a pas buzzé', () => {
    const props = makeGameMock({
      gameState: { phase: 'STARTED', question: { ID: '1', TYPE: 'QCM', QCM_CORRECT: 'RED' } },
      teams: { 'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0 } },
      bumpers: { r1: { TEAM: 'Les Rouges', TIME: 0 } },
    })
    useGame.mockReturnValue(props)
    const { container } = render(<AnimPage />)
    expect(container.querySelector('.anim-team-qcm-answer')).toBeNull()
  })

  it('reste visuellement inchangée hors QCM (pas de badge de couleur en SPEEDY)', () => {
    const props = makeGameMock({
      gameState: { phase: 'STARTED', question: { ID: '1', TYPE: 'SPEEDY' } },
      teams: { 'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0 } },
      bumpers: { r1: { TEAM: 'Les Rouges', TIME: 1000 } },
    })
    useGame.mockReturnValue(props)
    const { container } = render(<AnimPage />)
    expect(container.querySelector('.anim-team-qcm-answer')).toBeNull()
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

// ---------------------------------------------------------------------------
// #157 T5 (test-writer) — compléments aux 16 tests T3/T4 de dev-frontend
// (rapport _work/reports/dev-frontend-20260813-153639.md, commit 8ebffd1).
// Angles non couverts par cette suite, vérifiés en la relisant d'abord pour
// ne rien dupliquer :
//   - un rendu MIXTE (3 équipes, les 3 branches de calcQcmTeamAward à la
//     fois — correct/repli-couleur-fausse/repli-sans-buzz) au lieu d'un
//     scénario par test
//   - le "no-op" de handleCredit quand aucun bumper n'a buzzé pour une
//     équipe cible=PLAYER (fastestBumper undefined) — non exercé
//   - la garde isQcmWithHints (QCM_HINTS_ENABLED=false) : seule la
//     régression SPEEDY était testée, pas la régression QCM-sans-indices,
//     un code path distinct (`gameState.question?.TYPE === 'QCM' &&
//     gameState.question?.QCM_HINTS_ENABLED`, AnimPage.jsx:146)
//   - QCM + POINTS_TARGET=TEAM combinés (testés séparément jusqu'ici :
//     QCM+PLAYER d'un côté, SPEEDY+TEAM de l'autre)
//   - un balayage de parité sur plusieurs valeurs de HINTS_AT_BUZZ (pas
//     seulement le cas 2 indices déjà couvert)
//   - la table QCM_COLORS réellement consommée (lettre/couleur/label exacts
//     pour une couleur autre que RED) et ses deux cas limites (couleur
//     absente de la table, buzzer correct sans NAME)
// ---------------------------------------------------------------------------

describe('AnimPage — crédit QCM par équipe, compléments (#157/T3)', () => {
  it('rendu mixte : 3 équipes, les 3 branches de calcQcmTeamAward en même temps', () => {
    const props = makeGameMock({
      gameState: {
        phase: 'STOPPED',
        question: { ID: '1', TYPE: 'QCM', QCM_HINTS_ENABLED: true, QCM_CORRECT: 'RED', POINTS: '10' },
        qcmInvalidated: ['GREEN', 'YELLOW'], // 2 indices courants -> pénalité 0.33 pour le repli
      },
      creditPoints: 10,
      teams: {
        'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0 }, // buzzer correct, 1 indice -> 7 pts
        'Les Bleus': { COLOR: [0, 0, 255], SCORE: 0 }, // buzzer incorrect -> repli 0.33 -> 3 pts
        'Les Verts': { COLOR: [0, 255, 0], SCORE: 0 }, // n'a pas buzzé (TIME=0) -> même repli -> 3 pts
      },
      bumpers: {
        r1: { TEAM: 'Les Rouges', TIME: 1000, ANSWER_COLOR: 'RED', HINTS_AT_BUZZ: 1 },
        b1: { TEAM: 'Les Bleus', TIME: 1200, ANSWER_COLOR: 'GREEN', HINTS_AT_BUZZ: 0 },
        v1: { TEAM: 'Les Verts', TIME: 0 },
      },
    })
    useGame.mockReturnValue(props)
    const { container } = render(<AnimPage />)

    // Rouges (buzzer correct, 1 indice) -> 7 pts ; Bleus et Verts partagent
    // le même repli (indices COURANTS, valeur globale) -> 3 pts chacun —
    // même montant bien que dans des situations différentes (mauvaise
    // couleur vs aucun buzz), ce qui est correct : calcQcmTeamAward n'a
    // qu'un seul repli, pas un par circonstance.
    const amounts = Array.from(container.querySelectorAll('.anim-team-credit-btn')).map(b => b.textContent)
    expect(amounts.filter(a => a === '+7 pts')).toHaveLength(1)
    expect(amounts.filter(a => a === '+3 pts')).toHaveLength(2)
    expect(amounts).toHaveLength(3)

    // Scope les clics par carte d'équipe (deux boutons partagent le même
    // texte "+3 pts") plutôt que par texte seul.
    const cardFor = (teamName) =>
      Array.from(container.querySelectorAll('.anim-team-card'))
        .find(card => card.querySelector('.anim-team-card-name')?.textContent === teamName)

    cardFor('Les Rouges').querySelector('.anim-team-credit-btn').click()
    expect(props.setBumperPoints).toHaveBeenCalledWith('r1', 7)

    // Les Bleus ont buzzé (mauvaise couleur) — le crédit va quand même à ce
    // bumper, au montant réduit.
    cardFor('Les Bleus').querySelector('.anim-team-credit-btn').click()
    expect(props.setBumperPoints).toHaveBeenCalledWith('b1', 3)

    // Les Verts n'ont pas buzzé du tout — le bouton affiche le même repli
    // mais aucun bumper n'est éligible : aucun crédit ne doit partir pour
    // cette équipe (couvert plus précisément par le test suivant).
    props.setBumperPoints.mockClear()
    cardFor('Les Verts').querySelector('.anim-team-credit-btn').click()
    expect(props.setBumperPoints).not.toHaveBeenCalled()
  })

  it('équipe sans aucun buzz : le bouton de crédit s\'affiche mais le clic ne crédite personne (cible PLAYER)', () => {
    const props = makeGameMock({
      gameState: {
        phase: 'STOPPED',
        question: { ID: '1', TYPE: 'QCM', QCM_HINTS_ENABLED: true, QCM_CORRECT: 'RED' },
        qcmInvalidated: [],
      },
      creditPoints: 10,
      teams: { 'Les Verts': { COLOR: [0, 255, 0], SCORE: 0 } },
      bumpers: { v1: { TEAM: 'Les Verts', TIME: 0 } },
    })
    useGame.mockReturnValue(props)
    render(<AnimPage />)

    const button = screen.getByText('+10 pts') // aucun indice invalidé -> repli neutre, montant de base
    expect(button).toBeInTheDocument()
    button.click()
    expect(props.setBumperPoints).not.toHaveBeenCalled()
    expect(props.setTeamPoints).not.toHaveBeenCalled()
  })

  it('QCM_HINTS_ENABLED=false : montant unique pour toutes les équipes malgré des indices différents au buzz (garde isQcmWithHints)', () => {
    const props = makeGameMock({
      gameState: {
        phase: 'STOPPED',
        question: { ID: '1', TYPE: 'QCM', QCM_HINTS_ENABLED: false, QCM_CORRECT: 'RED' },
      },
      creditPoints: 10,
      teams: {
        'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0 },
        'Les Bleus': { COLOR: [0, 0, 255], SCORE: 0 },
      },
      bumpers: {
        // Indices très différents (2 vs 0) — ne doivent avoir AUCUN effet
        // puisque QCM_HINTS_ENABLED est false : c'est un code path distinct
        // de la non-régression SPEEDY (TYPE différent), à garder testé
        // séparément.
        r1: { TEAM: 'Les Rouges', TIME: 1000, ANSWER_COLOR: 'RED', HINTS_AT_BUZZ: 2 },
        b1: { TEAM: 'Les Bleus', TIME: 1200, ANSWER_COLOR: 'GREEN', HINTS_AT_BUZZ: 0 },
      },
    })
    useGame.mockReturnValue(props)
    const { container } = render(<AnimPage />)
    const amounts = Array.from(container.querySelectorAll('.anim-team-credit-btn')).map(b => b.textContent)
    expect(amounts).toEqual(['+10 pts', '+10 pts'])
  })

  it('QCM + POINTS_TARGET=TEAM : crédite l\'équipe avec SON montant (pas un montant global)', () => {
    const props = makeGameMock({
      gameState: {
        phase: 'STOPPED',
        question: {
          ID: '1', TYPE: 'QCM', QCM_HINTS_ENABLED: true, QCM_CORRECT: 'RED', POINTS_TARGET: 'TEAM',
        },
      },
      creditPoints: 10,
      teams: { 'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0 } },
      bumpers: { r1: { TEAM: 'Les Rouges', TIME: 1000, ANSWER_COLOR: 'RED', HINTS_AT_BUZZ: 1 } },
    })
    useGame.mockReturnValue(props)
    render(<AnimPage />)

    screen.getByText('+7 pts').click()
    expect(props.setTeamPoints).toHaveBeenCalledWith('Les Rouges', 7)
    expect(props.setBumperPoints).not.toHaveBeenCalled()
  })

  it.each([0, 1, 2, 3])(
    'parité avec calcQcmTeamAward pour %i indice(s) au buzz',
    (hints) => {
      const question = { ID: '1', TYPE: 'QCM', QCM_HINTS_ENABLED: true, QCM_CORRECT: 'RED' }
      const bumper = { TEAM: 'Les Rouges', TIME: 1000, ANSWER_COLOR: 'RED', HINTS_AT_BUZZ: hints, mac: 'r1' }
      const expected = calcQcmTeamAward(question, 10, [bumper], 0).amount

      const props = makeGameMock({
        gameState: { phase: 'STOPPED', question },
        creditPoints: 10,
        teams: { 'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0 } },
        bumpers: { r1: bumper },
      })
      useGame.mockReturnValue(props)
      const { container } = render(<AnimPage />)
      expect(container.querySelector('.anim-team-credit-btn').textContent).toBe(`+${expected} pts`)
    }
  )
})

describe('AnimPage — réponse QCM en zone C, compléments (#157/T4)', () => {
  it('couleur autre que RED : lettre, teinte et libellé exacts de la table QCM_COLORS (GREEN)', () => {
    const props = makeGameMock({
      gameState: { phase: 'STARTED', question: { ID: '1', TYPE: 'QCM', QCM_CORRECT: 'RED' } },
      teams: { 'Les Verts': { COLOR: [0, 255, 0], SCORE: 0 } },
      bumpers: { v1: { TEAM: 'Les Verts', TIME: 1000, ANSWER_COLOR: 'GREEN', NAME: 'Chloé' } },
    })
    useGame.mockReturnValue(props)
    const { container } = render(<AnimPage />)
    const swatch = container.querySelector('.anim-team-qcm-color')
    expect(swatch.textContent).toBe('B')
    expect(swatch.style.backgroundColor).toBe('rgb(34, 197, 94)') // #22c55e
    expect(swatch.title).toBe('Vert')
  })

  it('ANSWER_COLOR absente de QCM_COLORS : pas de pastille, mais le nom du joueur reste affiché', () => {
    const props = makeGameMock({
      gameState: { phase: 'STARTED', question: { ID: '1', TYPE: 'QCM', QCM_CORRECT: 'RED' } },
      teams: { 'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0 } },
      bumpers: { r1: { TEAM: 'Les Rouges', TIME: 1000, ANSWER_COLOR: 'PURPLE', NAME: 'Zoé' } },
    })
    useGame.mockReturnValue(props)
    const { container } = render(<AnimPage />)
    expect(container.querySelector('.anim-team-qcm-color')).toBeNull()
    expect(screen.getByText('Zoé')).toBeInTheDocument()
  })

  it('buzzer correct sans NAME : pastille affichée, aucun nom rendu', () => {
    const props = makeGameMock({
      gameState: { phase: 'STARTED', question: { ID: '1', TYPE: 'QCM', QCM_CORRECT: 'RED' } },
      teams: { 'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0 } },
      bumpers: { r1: { TEAM: 'Les Rouges', TIME: 1000, ANSWER_COLOR: 'RED' } }, // pas de NAME
    })
    useGame.mockReturnValue(props)
    const { container } = render(<AnimPage />)
    expect(container.querySelector('.anim-team-qcm-color')).not.toBeNull()
    expect(container.querySelector('.anim-team-qcm-player')).toBeNull()
  })
})
