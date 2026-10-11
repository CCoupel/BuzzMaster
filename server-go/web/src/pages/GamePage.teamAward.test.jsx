import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, fireEvent, screen } from '@testing-library/react'
import GamePage from './GamePage'
import { useGame } from '../hooks/GameContext'

// #244/#251/#254 (dev-frontend) — complément des suites du test-writer : envoi, résultat, refus,
// bornes ±999 et niveau JOUEUR (BUMPER_POINTS_BATCH). TeamCard réel (cases à cocher).

vi.mock('../hooks/GameContext', () => ({
  useGame: vi.fn(),
  GameProvider: ({ children }) => children,
}))

vi.mock('../hooks/useCategoryFilter', () => ({
  useCategoryFilter: vi.fn((questions) => ({
    selectedCategories: new Set(),
    availableCategories: [],
    filteredQuestions: questions ? Object.values(questions) : [],
    toggleCategoryFilter: vi.fn(),
    clearCategoryFilters: vi.fn(),
  })),
}))

vi.mock('../components/Timer', () => ({
  default: ({ phase }) => <div data-testid="timer" data-phase={phase} />,
}))

vi.mock('../components/QuestionPreview', () => ({
  default: () => <div data-testid="question-preview" />,
}))

vi.mock('../components/TeamCard', async () => {
  const actual = await vi.importActual('../components/TeamCard')
  return actual
})

vi.mock('../components/QuestionCard', () => ({
  default: ({ question, onClick }) => (
    <div
      data-testid={`question-card-${question.ID}`}
      onClick={() => onClick && onClick(question)}
    />
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

vi.mock('../utils/colorUtils', async () => {
  const actual = await vi.importActual('../utils/colorUtils')
  return actual
})

vi.mock('./GamePage.css', () => ({}))


const T = (n, c) => ({ NAME: n, SCORE: 0, COLOR: c })
const TEAMS = { 'Les Bleus': T('Les Bleus', [0, 0, 255]), 'Les Rouges': T('Les Rouges', [255, 0, 0]) }
const bp = (team, name, color) => ({ TEAM: team, NAME: name, SCORE: 0, IS_VIRTUAL: true, IS_VPLAYER: true, CONNECTED: true, COLOR: [1, 2, 3], ANSWER_COLOR: color, TIME: 1 })
const BUMPERS = { b1: bp('Les Bleus', 'Paul', 'GREEN'), b2: bp('Les Rouges', 'Mamie', 'RED') }
const QCM = (over = {}) => ({ ID: '9', TYPE: 'QCM', QUESTION: 'Fleuve ?', ANSWER: 'La Seine', POINTS: '3', STATUS: 'REVEALED',
  QCM_CORRECT: 'GREEN', QCM_ANSWERS: { RED: 'La Loire', GREEN: 'La Seine', YELLOW: 'Le Rhône', BLUE: 'La Garonne' }, ...over })

const mkGame = (extra = {}, question = QCM()) => ({
  gameState: { phase: 'REVEALED', question, remote: 'GAME', timer: 0, totalTime: 30, MEMORY_PARTICIPATING_TEAMS: [], gameTime: 1000000, qcmInvalidated: [] },
  teams: TEAMS, bumpers: BUMPERS, questions: { '9': { ID: '9', STATUS: 'REVEALED', ORDER: 1 } },
  startGame: vi.fn(), stopGame: vi.fn(), pauseGame: vi.fn(), continueGame: vi.fn(), revealAnswer: vi.fn(),
  selectQuestion: vi.fn(), setRemoteDisplay: vi.fn(), setBumperPoints: vi.fn(), setTeamPoints: vi.fn(),
  forceReady: vi.fn(), simulateButton: vi.fn(), simulatePong: vi.fn(), sendMessage: vi.fn(),
  teamPointsBatch: vi.fn(() => true), bumperPointsBatch: vi.fn(() => true),
  awardedTeams: {}, awardedPlayers: {},
  teamPointsBatchResult: null, clearTeamPointsBatchResult: vi.fn(),
  bumperPointsBatchResult: null, clearBumperPointsBatchResult: vi.fn(),
  ...extra,
})
const action = () => screen.getByRole('button', { name: /^(Attribuer les points|Appliquer les pénalités) \(\d+\)$/ })
const box = (name) => screen.getByRole('checkbox', { name: `Sélectionner ${name}` })
const pointsField = () => screen.getByLabelText('Points')

describe('GamePage — attribution groupée : envoi, résultat, bornes, niveau joueur (#244/#251/#254)', () => {
  beforeEach(() => useGame.mockReset())

  it('niveau équipe : un lot TEAM_POINTS_BATCH avec le montant par défaut, bouton désactivé pendant l\'envoi', () => {
    const g = mkGame(); useGame.mockReturnValue(g)
    render(<GamePage />)
    fireEvent.click(box('Les Bleus'))
    fireEvent.click(action())
    expect(g.teamPointsBatch).toHaveBeenCalledWith('9', [{ team: 'Les Bleus', points: 1 }])
    expect(action().disabled).toBe(true)
    expect(g.bumperPointsBatch).not.toHaveBeenCalled()
  })

  it('niveau JOUEUR (POINTS_TARGET PLAYER) : cases sur les lignes de joueurs, aucune case d\'équipe, BUMPER_POINTS_BATCH', () => {
    const g = mkGame({}, QCM({ POINTS_TARGET: 'PLAYER' })); useGame.mockReturnValue(g)
    render(<GamePage />)
    expect(screen.queryByRole('checkbox', { name: 'Sélectionner Les Bleus' })).toBeNull()
    fireEvent.click(box('Paul'))
    fireEvent.click(box('Mamie'))
    fireEvent.click(action())
    expect(g.bumperPointsBatch).toHaveBeenCalledWith('9', [{ id: 'b1', points: 1 }, { id: 'b2', points: 1 }])
    expect(g.teamPointsBatch).not.toHaveBeenCalled()
  })

  it('joueurs déjà crédités (AWARDED_TEAMS.PLAYERS) : case grisée, « ✓ déjà crédité +N »', () => {
    const g = mkGame({ awardedPlayers: { b1: { POINTS: 3, TIMESTAMP: 1, TEAM: 'Les Bleus' } } }, QCM({ POINTS_TARGET: 'PLAYER' }))
    useGame.mockReturnValue(g)
    render(<GamePage />)
    expect(box('Paul').disabled).toBe(true)
    expect(screen.getByText('✓ déjà crédité +3')).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: 'Tout sélectionner' }))
    expect(box('Paul').checked).toBe(false) // jamais re-sélectionné
    expect(box('Mamie').checked).toBe(true)
    expect(action().textContent).toBe('Attribuer les points (1)')
  })

  it('résultat APPLIED : confirmation, sélection vidée, déjà crédités ignorés signalés', () => {
    const g = mkGame(); useGame.mockReturnValue(g)
    const { rerender } = render(<GamePage />)
    fireEvent.click(box('Les Bleus'))
    fireEvent.click(action())
    useGame.mockReturnValue({ ...g, teamPointsBatchResult: {
      status: 'APPLIED', questionId: '9', teams: [],
      applied: [{ team: 'Les Bleus', points: 1 }],
      skipped: [{ team: 'Les Rouges', reason: 'ALREADY_AWARDED' }],
    } })
    rerender(<GamePage />)
    expect(screen.getByText(/✓ Points attribués à 1 équipe\. 1 équipe déjà créditée ignorée\./)).toBeTruthy()
    expect(action().textContent).toBe('Attribuer les points (0)')
    expect(g.clearTeamPointsBatchResult).toHaveBeenCalled()
  })

  it('résultat pénalité : « ✓ Pénalité −2 appliquée à 1 équipe. »', () => {
    const g = mkGame(); useGame.mockReturnValue(g)
    const { rerender } = render(<GamePage />)
    useGame.mockReturnValue({ ...g, teamPointsBatchResult: { status: 'APPLIED', questionId: '9', teams: [], applied: [{ team: 'Les Rouges', points: -2 }], skipped: [] } })
    rerender(<GamePage />)
    expect(screen.getByText('✓ Pénalité −2 appliquée à 1 équipe.')).toBeTruthy()
  })

  it('résultat UNKNOWN_PLAYER : message et joueur retiré de la sélection', () => {
    const g = mkGame({}, QCM({ POINTS_TARGET: 'PLAYER' })); useGame.mockReturnValue(g)
    const { rerender } = render(<GamePage />)
    fireEvent.click(box('Paul'))
    fireEvent.click(box('Mamie'))
    useGame.mockReturnValue({ ...g, bumperPointsBatchResult: { status: 'REFUSED', reason: 'UNKNOWN_PLAYER', players: ['b1'], applied: [], skipped: [], questionId: '9' } })
    rerender(<GamePage />)
    expect(screen.getByText('Joueur introuvable (supprimé) : sélection mise à jour.')).toBeTruthy()
    expect(box('Paul').checked).toBe(false)
    expect(box('Mamie').checked).toBe(true)
  })

  it('montant hors bornes (±999) : bouton désactivé, raison VISIBLE dans le texte', () => {
    const g = mkGame(); useGame.mockReturnValue(g)
    render(<GamePage />)
    fireEvent.click(box('Les Bleus'))
    fireEvent.change(pointsField(), { target: { value: '1000' } })
    expect(action().disabled).toBe(true)
    expect(screen.getByText('Le montant doit être compris entre −999 et 999')).toBeTruthy()
    fireEvent.change(pointsField(), { target: { value: '999' } })
    expect(action().disabled).toBe(false)
  })

  it('montant nul : raison visible', () => {
    useGame.mockReturnValue(mkGame())
    render(<GamePage />)
    fireEvent.click(box('Les Bleus'))
    fireEvent.change(pointsField(), { target: { value: '0' } })
    expect(action().disabled).toBe(true)
    expect(screen.getByText(/Montant nul ou invalide pour : Les Bleus/)).toBeTruthy()
  })

  it('verrou « Lancer la partie » d\'un autre poste : attribution suspendue avec le motif', () => {
    const g = mkGame(); g.connectionId = 'me'
    g.gameState.LAUNCH_LOCK = { ACTIVE: true, OWNER: 'other', SINCE: 1, EXPIRES_AT: 2 }
    useGame.mockReturnValue(g)
    render(<GamePage />)
    fireEvent.click(box('Les Bleus'))
    expect(action().disabled).toBe(true)
    expect(screen.getByText(/Lancement d’une nouvelle partie en cours sur un autre poste/)).toBeTruthy()
  })
})
