import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, fireEvent, screen } from '@testing-library/react'
import GamePage from './GamePage'
import { useGame } from '../hooks/GameContext'

// #244/#251 — sélection d'équipes et « Donner les points (N) » (maquette admin_game_team_award__selection).
// TeamCard réel (cases à cocher), mocks légers pour le reste.

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
const TEAMS = { 'Les Bleus': T('Les Bleus', [0, 0, 255]), 'Les Rouges': T('Les Rouges', [255, 0, 0]), 'Les Verts': T('Les Verts', [0, 255, 0]), 'Les Gris': T('Les Gris', [128, 128, 128]) }
const bp = (team, color) => ({ TEAM: team, NAME: `VJ-${team}`, SCORE: 0, IS_VIRTUAL: true, IS_VPLAYER: true, CONNECTED: true, COLOR: [1, 2, 3], ANSWER_COLOR: color, TIME: 1 })
const BUMPERS = { b1: bp('Les Bleus', 'GREEN'), b2: bp('Les Rouges', 'GREEN'), b3: bp('Les Verts', 'RED'), b4: { ...bp('Les Gris', null), ANSWER_COLOR: undefined, TIME: 0 } }
const QCM = { ID: '9', TYPE: 'QCM', QUESTION: 'Fleuve ?', ANSWER: 'La Seine', POINTS: '3', STATUS: 'REVEALED',
  QCM_CORRECT: 'GREEN', QCM_ANSWERS: { RED: 'La Loire', GREEN: 'La Seine', YELLOW: 'Le Rhône', BLUE: 'La Garonne' } }

const mkGame = (phase, extra = {}) => ({
  gameState: { phase, question: QCM, remote: 'GAME', timer: 0, totalTime: 30, MEMORY_PARTICIPATING_TEAMS: [], gameTime: 1000000 },
  teams: TEAMS, bumpers: BUMPERS, questions: { '9': { ID: '9', STATUS: phase, ORDER: 1 } },
  startGame: vi.fn(), stopGame: vi.fn(), pauseGame: vi.fn(), continueGame: vi.fn(), revealAnswer: vi.fn(),
  selectQuestion: vi.fn(), setRemoteDisplay: vi.fn(), setBumperPoints: vi.fn(), setTeamPoints: vi.fn(),
  forceReady: vi.fn(), simulateButton: vi.fn(), simulatePong: vi.fn(), sendMessage: vi.fn(),
  teamPointsBatch: vi.fn(() => true), awardedTeams: {}, teamPointsBatchResult: null, clearTeamPointsBatchResult: vi.fn(),
  ...extra,
})
const give = () => screen.getByRole('button', { name: /(Donner|Retirer) les points/ })
const box = (name) => screen.getByRole('checkbox', { name: `Sélectionner ${name}` })

describe('GamePage — sélection d\'équipes et attribution groupée (#244/#251)', () => {
  beforeEach(() => useGame.mockReset())

  it('avant le REVEAL : ni cases à cocher, ni barre, ni réponses cliquables', () => {
    useGame.mockReturnValue(mkGame('STARTED'))
    render(<GamePage />)
    expect(screen.queryByRole('checkbox', { name: /Sélectionner/ })).toBeNull()
    expect(screen.queryByText('Points par équipe')).toBeNull()
    expect(screen.queryByRole('group', { name: 'Réponses QCM' })).toBeNull()
  })

  it('après le REVEAL : cases, barre (bouton désactivé à N = 0), réponses QCM avec compteurs', () => {
    useGame.mockReturnValue(mkGame('REVEALED'))
    render(<GamePage />)
    expect(box('Les Bleus').checked).toBe(false)
    expect(give().textContent).toBe('Donner les points (0)')
    expect(give().disabled).toBe(true)
    const group = screen.getByRole('group', { name: 'Réponses QCM' })
    expect(group.textContent).toMatch(/La Seine ✓ bonne réponse2 équipes/)
    expect(group.textContent).toMatch(/La Loire1 équipe/)
  })

  it('clic sur la bonne réponse : sélectionne ses équipes et règle B = points de la question (champ Points de la page, pointsInput)', () => {
    const g = mkGame('REVEALED')
    useGame.mockReturnValue(g)
    render(<GamePage />)
    fireEvent.click(screen.getByRole('button', { name: /La Seine/ }))
    expect(box('Les Bleus').checked).toBe(true)
    expect(box('Les Rouges').checked).toBe(true)
    expect(box('Les Verts').checked).toBe(false)
    expect(give().textContent).toBe('Donner les points (2)')
    // B = champ « Points » de la page (pointsInput, 1 par défaut tant que la question n'a pas été sélectionnée ici)
    expect(screen.getByLabelText('Points par équipe', { selector: 'input' }).value).toBe('1')
    fireEvent.click(give())
    expect(g.teamPointsBatch).toHaveBeenCalledWith('9', [
      { team: 'Les Bleus', points: 1 }, { team: 'Les Rouges', points: 1 },
    ])
    expect(give().disabled).toBe(true) // envoi en cours : pas de double envoi
  })

  it('clic sur une mauvaise réponse : désélectionne tout, sélectionne ses équipes, B = −1, « Retirer »', () => {
    useGame.mockReturnValue(mkGame('REVEALED'))
    render(<GamePage />)
    fireEvent.click(screen.getByRole('button', { name: /La Seine/ }))
    fireEvent.click(screen.getByRole('button', { name: /La Loire/ }))
    expect(box('Les Bleus').checked).toBe(false)
    expect(box('Les Verts').checked).toBe(true)
    expect(screen.getByLabelText('Points par équipe', { selector: 'input' }).value).toBe('-1')
    expect(give().textContent).toBe('Retirer les points (1)')
    expect(give().className).toMatch(/neg/)
  })

  it('équipes déjà créditées (AWARDED_TEAMS) : case grisée, « déjà créditée », non sélectionnée par un clic de réponse', () => {
    useGame.mockReturnValue(mkGame('REVEALED', { awardedTeams: { 'Les Bleus': { POINTS: 3, TIMESTAMP: 1 } } }))
    render(<GamePage />)
    expect(box('Les Bleus').disabled).toBe(true)
    expect(screen.getByText('✓ déjà créditée +3')).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: /La Seine/ }))
    expect(box('Les Bleus').checked).toBe(false)
    expect(give().textContent).toBe('Donner les points (1)')
  })

  it('sélection manuelle + montant modifié à la main : montant uniforme, source indiquée', () => {
    const g = mkGame('REVEALED')
    useGame.mockReturnValue(g)
    render(<GamePage />)
    fireEvent.click(box('Les Verts'))
    fireEvent.click(box('Les Gris'))
    fireEvent.change(screen.getByLabelText('Points par équipe', { selector: 'input' }), { target: { value: '5' } })
    expect(screen.getByText('montant modifié à la main')).toBeTruthy()
    fireEvent.click(give())
    expect(g.teamPointsBatch).toHaveBeenCalledWith('9', [
      { team: 'Les Verts', points: 5 }, { team: 'Les Gris', points: 5 },
    ])
  })

  it('« Tout sélectionner / Tout désélectionner »', () => {
    useGame.mockReturnValue(mkGame('REVEALED', { awardedTeams: { 'Les Gris': { POINTS: 3, TIMESTAMP: 1 } } }))
    render(<GamePage />)
    fireEvent.click(screen.getByText('Tout sélectionner'))
    expect(give().textContent).toBe('Donner les points (3)') // les équipes créditées sont exclues
    fireEvent.click(screen.getByText('Tout désélectionner'))
    expect(give().textContent).toBe('Donner les points (0)')
  })

  it('résultat APPLIED : confirmation, sélection vidée, équipes déjà créditées ignorées signalées', () => {
    const g = mkGame('REVEALED')
    useGame.mockReturnValue(g)
    const { rerender } = render(<GamePage />)
    fireEvent.click(screen.getByRole('button', { name: /La Seine/ }))
    fireEvent.click(give())
    useGame.mockReturnValue({ ...g, teamPointsBatchResult: {
      status: 'APPLIED', questionId: '9', teams: [],
      applied: [{ team: 'Les Bleus', points: 3 }, { team: 'Les Rouges', points: 3 }],
      skipped: [{ team: 'Les Verts', reason: 'ALREADY_AWARDED' }],
    } })
    rerender(<GamePage />)
    expect(screen.getByRole('status', { name: '' }) || true).toBeTruthy()
    expect(screen.getByText(/✓ \+3 points donnés à 2 équipes\. 1 équipe déjà créditée ignorée\./)).toBeTruthy()
    expect(give().textContent).toBe('Donner les points (0)')
    expect(g.clearTeamPointsBatchResult).toHaveBeenCalled()
  })

  it('résultat UNKNOWN_TEAM : message et équipes retirées de la sélection', () => {
    const g = mkGame('REVEALED')
    useGame.mockReturnValue(g)
    const { rerender } = render(<GamePage />)
    fireEvent.click(screen.getByRole('button', { name: /La Seine/ }))
    useGame.mockReturnValue({ ...g, teamPointsBatchResult: { status: 'REFUSED', reason: 'UNKNOWN_TEAM', teams: ['Les Bleus'], applied: [], skipped: [] } })
    rerender(<GamePage />)
    expect(screen.getByText(/Équipe introuvable \(renommée ou supprimée\) : sélection mise à jour\./)).toBeTruthy()
    expect(box('Les Bleus').checked).toBe(false)
    expect(box('Les Rouges').checked).toBe(true)
  })

  it('le clic sur la carte d\'équipe (crédit unitaire) reste inchangé', () => {
    const g = mkGame('REVEALED')
    useGame.mockReturnValue(g)
    render(<GamePage />)
    fireEvent.click(screen.getByText('Les Bleus', { selector: 'h3' }))
    expect(g.setTeamPoints).toHaveBeenCalledWith('Les Bleus', expect.any(Number))
    expect(g.teamPointsBatch).not.toHaveBeenCalled()
  })

  it('changement de question : sélection remise à zéro', () => {
    const g = mkGame('REVEALED')
    useGame.mockReturnValue(g)
    const { rerender } = render(<GamePage />)
    fireEvent.click(box('Les Verts'))
    expect(give().textContent).toBe('Donner les points (1)')
    useGame.mockReturnValue({ ...g, gameState: { ...g.gameState, question: { ...QCM, ID: '10' } } })
    rerender(<GamePage />)
    expect(give().textContent).toBe('Donner les points (0)')
  })
})
