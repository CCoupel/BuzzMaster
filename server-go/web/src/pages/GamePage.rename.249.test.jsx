import { describe, it, expect, vi } from 'vitest'
import { render } from '@testing-library/react'
import GamePage from './GamePage'
import { useGame } from '../hooks/GameContext'

// #249 (v11.2) — reproduction (statut `regression`) de harness246_rename_gamepage : après un renommage d'équipe,
// l'objet équipe portait encore l'ANCIEN `NAME` ; le panneau ARDOISE cherchait la réponse sous `NAME`
// (« Equipe 1 ») alors que le serveur l'enregistre sous la clé (« Les Bleus ») → cellule « — » alors que le
// serveur détient le texte. Identité = clé : l'écran retrouve la réponse sous la clé, quel que soit `NAME`.

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

vi.mock('../components/TeamCard', () => ({
  default: ({ name }) => <div data-testid={`team-card-${name}`} />,
  OtaAllModal: () => null,
}))

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

vi.mock('../utils/colorUtils', () => ({
  getRgbColor: vi.fn((color) => (Array.isArray(color) ? `rgb(${color.join(',')})` : color || '#888')),
}))

vi.mock('./GamePage.css', () => ({}))



const base = (answers, teams) => ({
  gameState: { phase: 'STARTED', question: { ID: '10', TYPE: 'ARDOISE', QUESTION: 'Q', ANSWER: 'Paris', POINTS: '2', ARDOISE_KEYBOARD_TYPE: 'AZERTY' },
    remote: 'GAME', timer: 20, totalTime: 60, gameTime: 1000000, MEMORY_PARTICIPATING_TEAMS: [], ARDOISE_ANSWERS: answers },
  teams,
  bumpers: { vjA: { TEAM: 'Les Bleus', NAME: 'Alice', IS_VIRTUAL: true, IS_VPLAYER: true, CONNECTED: true },
             vjB: { TEAM: 'Equipe 2', NAME: 'Bob', IS_VIRTUAL: true, IS_VPLAYER: true, CONNECTED: true } },
  questions: { '10': { ID: '10', STATUS: 'STARTED', ORDER: 1 } },
  startGame: vi.fn(), stopGame: vi.fn(), pauseGame: vi.fn(), continueGame: vi.fn(), revealAnswer: vi.fn(), selectQuestion: vi.fn(),
  setRemoteDisplay: vi.fn(), setBumperPoints: vi.fn(), setTeamPoints: vi.fn(), forceReady: vi.fn(), simulateButton: vi.fn(), simulatePong: vi.fn(), sendMessage: vi.fn(),
})
const cells = (c) => Array.from(c.querySelectorAll('.ardoise-answer-row')).map((r) => [
  r.querySelector('.ardoise-answer-team-name span:not(.ardoise-team-dot):not(.ardoise-answer-rank):not(.ardoise-answer-delay)')?.textContent,
  r.querySelector('.ardoise-answer-text')?.textContent,
])

describe('GamePage — panneau ARDOISE après renommage d’équipe : identité = clé (#249)', () => {
  it('Critical — Regression : NAME périmé (« Equipe 1 »), réponse sous la clé « Les Bleus » → la cellule affiche la réponse, pas « — »', () => {
    useGame.mockReturnValue(base(
      { 'Les Bleus': { TEXT: 'NAPOLEON', STARTED_AT: 3, SUBMITTED_AT: 4 }, 'Equipe 2': { TEXT: 'LYON', STARTED_AT: 1, SUBMITTED_AT: 2 } },
      { 'Les Bleus': { NAME: 'Equipe 1', SCORE: 0, COLOR: [0, 0, 255] }, 'Equipe 2': { NAME: 'Equipe 2', SCORE: 0, COLOR: [255, 0, 0] } },
    ))
    const { container } = render(<GamePage />)

    const bleus = cells(container).find(([name]) => name === 'Les Bleus')
    expect(bleus, 'la ligne est libellée par la clé de l’équipe').toBeTruthy()
    expect(bleus[1]).toBe('NAPOLEON')
    expect(cells(container).find(([name]) => name === 'Equipe 1')).toBeUndefined() // plus d'identité par l'ancien NAME
  })

  it('une réponse enregistrée sous l’ancien NAME n’est pas attribuée à l’équipe renommée (« — »)', () => {
    useGame.mockReturnValue(base(
      { 'Equipe 1': { TEXT: 'ANCIEN', STARTED_AT: 3, SUBMITTED_AT: 4 } },
      { 'Les Bleus': { NAME: 'Equipe 1', SCORE: 0, COLOR: [0, 0, 255] }, 'Equipe 2': { NAME: 'Equipe 2', SCORE: 0, COLOR: [255, 0, 0] } },
    ))
    const { container } = render(<GamePage />)
    expect(cells(container).find(([name]) => name === 'Les Bleus')[1]).toBe('—')
  })

  it('non-régression : NAME = clé, comportement inchangé', () => {
    useGame.mockReturnValue(base(
      { 'Les Bleus': { TEXT: 'PARIS', STARTED_AT: 3, SUBMITTED_AT: 4 } },
      { 'Les Bleus': { NAME: 'Les Bleus', SCORE: 0, COLOR: [0, 0, 255] }, 'Equipe 2': { NAME: 'Equipe 2', SCORE: 0, COLOR: [255, 0, 0] } },
    ))
    const { container } = render(<GamePage />)
    expect(cells(container).find(([name]) => name === 'Les Bleus')[1]).toBe('PARIS')
  })
})
