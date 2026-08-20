import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import GamePage from './GamePage'

// ---------------------------------------------------------------------------
// GamePage — bouton ENTRACTE / FIN D'ENTRACTE (#119, T4 du plan
// _work/reports/plan-entracte-119-20260820-140825.md).
//
// Mêmes mocks que GamePage.test.jsx (F6 ajoute le bouton sans changer la
// structure des dépendances déjà mockées ailleurs).
// ---------------------------------------------------------------------------

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

import { useGame } from '../hooks/GameContext'

const makeGameMock = (overrides = {}) => ({
  gameState: {
    phase: 'STOPPED',
    question: { ID: '2', STATUS: 'STOPPED' },
    remote: 'GAME',
    timer: 0,
    totalTime: 30,
    MEMORY_PARTICIPATING_TEAMS: [],
    entracte: false,
    ...overrides.gameState,
  },
  teams: overrides.teams ?? {},
  bumpers: overrides.bumpers ?? {},
  questions: overrides.questions ?? {
    '1': { ID: '1', STATUS: 'PLAYED', ORDER: 1 },
    '2': { ID: '2', STATUS: 'STOPPED', ORDER: 2 },
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
  setEntracte: vi.fn(),
  ...overrides,
})

function getEntracteButton() {
  return screen.getByRole('button', { name: /ENTRACTE/i })
}

beforeEach(() => {
  vi.clearAllMocks()
})

// ---------------------------------------------------------------------------
// Libellé du bouton — dérivé de gameState.entracte, aucun état côté serveur (D3)
// ---------------------------------------------------------------------------

describe('GamePage — libellé du bouton ENTRACTE', () => {
  it('affiche "ENTRACTE" quand entracte est false', () => {
    useGame.mockReturnValue(makeGameMock({ gameState: { phase: 'READY', entracte: false } }))
    render(<GamePage />)
    expect(getEntracteButton()).toHaveTextContent('ENTRACTE')
    expect(screen.queryByText(/FIN D.ENTRACTE/i)).toBeNull()
  })

  it("affiche \"FIN D'ENTRACTE\" quand entracte est true", () => {
    useGame.mockReturnValue(makeGameMock({ gameState: { phase: 'STOPPED', entracte: true } }))
    render(<GamePage />)
    expect(screen.getByText(/FIN D.ENTRACTE/i)).toBeInTheDocument()
  })
})

// ---------------------------------------------------------------------------
// Table complète des phases — actif/désactivé selon D4 (entrée en entracte
// seulement, gameState.entracte reste false dans ce bloc)
// ---------------------------------------------------------------------------

describe('GamePage — bouton actif/désactivé selon la phase (D4, table complète)', () => {
  it.each([
    ['STOPPED', false],
    ['PREPARE', false],
    ['READY', false],
    ['NEW_GAME', false],
    ['REVEALED', false],
    ['COUNTDOWN', true],
    ['STARTED', true],
    ['PAUSED', true],
    ['ENROLL', true],
  ])('phase %s → bouton disabled=%s', (phase, expectedDisabled) => {
    useGame.mockReturnValue(makeGameMock({ gameState: { phase, entracte: false } }))
    render(<GamePage />)
    const btn = getEntracteButton()
    if (expectedDisabled) {
      expect(btn).toBeDisabled()
    } else {
      expect(btn).not.toBeDisabled()
    }
  })
})

// ---------------------------------------------------------------------------
// Sortie d'entracte toujours possible — même dans une phase où l'ENTRÉE
// serait refusée (acceptance criteria game-state.md : "jamais de blocage
// définitif"). Cas défensif : phase/entracte sont deux états indépendants
// côté client, même si le serveur ne les laisse pas coexister en pratique.
// ---------------------------------------------------------------------------

describe('GamePage — la sortie d\'entracte reste possible depuis toute phase', () => {
  it.each(['STOPPED', 'READY', 'COUNTDOWN', 'STARTED', 'PAUSED', 'ENROLL'])(
    'entracte actif + phase %s → bouton "FIN D\'ENTRACTE" reste cliquable',
    (phase) => {
      useGame.mockReturnValue(makeGameMock({ gameState: { phase, entracte: true } }))
      render(<GamePage />)
      expect(getEntracteButton()).not.toBeDisabled()
    }
  )
})

// ---------------------------------------------------------------------------
// Émission de ENTRACTE_SET (via setEntracte) — commande explicite, pas un
// toggle interne (D3) : la valeur émise porte l'état VOULU.
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// T3 — filtre .entracte-dim sur /admin : présent quand entracte est actif,
// le bouton ENTRACTE / FIN D'ENTRACTE restant l'élément NET (contracts/
// game-state.md, contrainte "aucun élément position:fixed ne se déplace" /
// maquette entracte-119.html). `.game-page` est une grille CSS à placement
// explicite (F6 du plan) : la classe est appliquée à CHAQUE enfant de
// grille séparément (pas un seul wrapper) — le compte exact n'est pas
// contractuel, seule l'existence d'au moins un nœud filtré l'est.
// ---------------------------------------------------------------------------

describe('GamePage — filtre .entracte-dim (T3)', () => {
  it('aucun filtre quand entracte est false', () => {
    useGame.mockReturnValue(makeGameMock({ gameState: { phase: 'READY', entracte: false } }))
    const { container } = render(<GamePage />)
    expect(container.querySelectorAll('.entracte-dim')).toHaveLength(0)
  })

  it('au moins un nœud filtré apparaît quand entracte est true', () => {
    useGame.mockReturnValue(makeGameMock({ gameState: { phase: 'STOPPED', entracte: true } }))
    const { container } = render(<GamePage />)
    expect(container.querySelectorAll('.entracte-dim').length).toBeGreaterThan(0)
  })

  it('le bouton ENTRACTE / FIN D\'ENTRACTE reste HORS de tout nœud filtré (net et cliquable)', () => {
    useGame.mockReturnValue(makeGameMock({ gameState: { phase: 'STOPPED', entracte: true } }))
    const { container } = render(<GamePage />)
    const dimNodes = container.querySelectorAll('.entracte-dim')
    const btn = getEntracteButton()
    dimNodes.forEach(dim => {
      expect(dim.contains(btn)).toBe(false)
    })
  })
})

describe('GamePage — clic sur le bouton émet ENTRACTE_SET avec la bonne valeur', () => {
  it('entracte=false, phase=READY → clic appelle setEntracte(true)', () => {
    const mock = makeGameMock({ gameState: { phase: 'READY', entracte: false } })
    useGame.mockReturnValue(mock)
    render(<GamePage />)
    fireEvent.click(getEntracteButton())
    expect(mock.setEntracte).toHaveBeenCalledTimes(1)
    expect(mock.setEntracte).toHaveBeenCalledWith(true)
  })

  it('entracte=true → clic appelle setEntracte(false)', () => {
    const mock = makeGameMock({ gameState: { phase: 'STOPPED', entracte: true } })
    useGame.mockReturnValue(mock)
    render(<GamePage />)
    fireEvent.click(getEntracteButton())
    expect(mock.setEntracte).toHaveBeenCalledWith(false)
  })

  it('bouton désactivé (phase STARTED, entracte=false) → un clic n\'appelle pas setEntracte', () => {
    const mock = makeGameMock({ gameState: { phase: 'STARTED', entracte: false } })
    useGame.mockReturnValue(mock)
    render(<GamePage />)
    fireEvent.click(getEntracteButton())
    expect(mock.setEntracte).not.toHaveBeenCalled()
  })
})
