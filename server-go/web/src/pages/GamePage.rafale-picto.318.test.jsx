import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import GamePage from './GamePage'
import { useGame } from '../hooks/GameContext'

// #318 (v13.1.0, LOT-3, T30) — page Jeu admin, Rafale Picto (picto.md §17.7, award-preselect.md §2, maquette M7).
// Boîte noire par libellés ARIA, même harnais que GamePage.team-award.244-251. Les tests existants ne sont pas modifiés.

vi.mock('../hooks/GameContext', () => ({ useGame: vi.fn(), GameProvider: ({ children }) => children }))
vi.mock('../components/Timer', () => ({ default: () => <div data-testid="timer" /> }))
vi.mock('../components/QuestionPreview', () => ({ default: () => <div /> }))
vi.mock('../components/QuestionCard', () => ({
  default: ({ question, onClick }) => <div data-testid={`question-card-${question.ID}`} onClick={() => onClick && onClick(question)} />,
  CATEGORIES: {},
}))
vi.mock('../components/Card', () => ({ default: ({ children, className, padding, variant, ...rest }) => <div className={className} {...rest}>{children}</div> }))
vi.mock('../components/Button', () => ({ default: ({ children, onClick, disabled, variant, size, ...rest }) => <button onClick={onClick} disabled={disabled} {...rest}>{children}</button> }))
vi.mock('./GamePage.css', () => ({}))

const TEAMS = {
  Bleus: { NAME: 'Bleus', SCORE: 0, COLOR: [0, 0, 255] },
  Rouges: { NAME: 'Rouges', SCORE: 0, COLOR: [255, 0, 0] },
}
const BUMPERS = {
  b1: { NAME: 'Alice', TEAM: 'Bleus', IS_VIRTUAL: true, IS_VPLAYER: true, CONNECTED: true, SCORE: 0, ANSWER_COLOR: '', TIME: 0 },
  b2: { NAME: 'Rita', TEAM: 'Rouges', IS_VIRTUAL: true, IS_VPLAYER: true, CONNECTED: true, SCORE: 0, ANSWER_COLOR: '', TIME: 0 },
}
const RP = (over = {}) => ({ ID: 'rp1', TYPE: 'RAFALE_PICTO', QUESTION: 'Faites deviner', POINTS: '10', TIME: '120', CATEGORY: 'GEOGRAPHY',
  RAFALE_PICTO_PASS_COST: 5, RAFALE_PICTO_PASS_LIMIT: 1, STATUS: 'REVEALED', ...over })
const mk = (gameState, over = {}) => ({
  gameState: { phase: 'REVEALED', remote: 'GAME', timer: 0, totalTime: 120, MEMORY_PARTICIPATING_TEAMS: [], ARDOISE_ANSWERS: {},
    PICTO_TEAM: 'Bleus', PICTO_TEAM_COLOR: [0, 0, 255], PICTO_DRAWER: 'b1', PICTO_DRAWER_NAME: 'Alice', PICTO_DRAWER_CONNECTED: true,
    PICTO_CANDIDATES: [], PICTO_CANVAS_EPOCH: 9, PICTO_OUTCOME: 'TIMEOUT', PICTO_REVEALED_WORD: 'Maison', PICTO_REVEALED_CATEGORY: 'GEOGRAPHY',
    PICTO_FOUND_COUNT: 3, PICTO_FOUND_WORDS: ['Chat', 'Chien', 'Lion'], PICTO_PASS_COUNT: 1, PICTO_PASSES_LEFT: 0, PICTO_POOL_AVAILABLE: 4, PICTO_ROUND_POINTS: 25,
    ...gameState },
  teams: TEAMS, bumpers: BUMPERS, questions: { [gameState.question.ID]: { ...gameState.question, ORDER: 1 } },
  awardedTeams: {}, awardedPlayers: {}, creditPoints: 10,
  startGame: vi.fn(), stopGame: vi.fn(), pauseGame: vi.fn(), continueGame: vi.fn(), revealAnswer: vi.fn(), selectQuestion: vi.fn(),
  setRemoteDisplay: vi.fn(), setBumperPoints: vi.fn(), setTeamPoints: vi.fn(), forceReady: vi.fn(), simulateButton: vi.fn(), simulatePong: vi.fn(),
  sendMessage: vi.fn(), newGame: vi.fn(), newGameRefused: null, clearNewGameRefused: vi.fn(),
  teamPointsBatch: vi.fn(() => true), teamPointsBatchResult: null, clearTeamPointsBatchResult: vi.fn(),
  pictoWord: null, pictoError: null, pictoCanvasStore: null, pictoFound: vi.fn(), pictoPass: vi.fn(),
  ...over,
})
const reveal = (gameState, over) => {
  useGame.mockReturnValue(mk({ ...gameState, phase: 'STOPPED' }, over))
  const utils = render(<GamePage />)
  const card = screen.queryByTestId(`question-card-${gameState.question.ID}`)
  if (card) fireEvent.click(card)
  const game = mk(gameState, over)
  useGame.mockReturnValue(game)
  utils.rerender(<GamePage />)
  return { game, ...utils }
}
const box = (n) => screen.getByRole('checkbox', { name: `Sélectionner ${n}` })
const pointsInput = () => screen.getAllByLabelText('Points').find((el) => el.tagName === 'INPUT' && el.type === 'number')

describe('GamePage — crédit d’une manche Rafale Picto (#318, §17.7)', () => {
  beforeEach(() => { useGame.mockReset() })

  it('l’équipe qui dessinait est présélectionnée et le champ « Points » est pré-rempli à PICTO_ROUND_POINTS', () => {
    reveal({ question: RP() })
    expect(box('Bleus').checked).toBe(true)
    expect(box('Rouges').checked).toBe(false)
    expect(pointsInput().value).toBe('25')
  })

  it('le montant pré-rempli reste ajustable avant le crédit', () => {
    reveal({ question: RP() })
    fireEvent.change(pointsInput(), { target: { value: '30' } })
    expect(pointsInput().value).toBe('30')
    expect(box('Bleus').checked).toBe(true) // modifier « Points » ne décoche rien
  })

  it('manche à 0 point (plancher 0) : aucune équipe cochée d’office', () => {
    reveal({ question: RP(), PICTO_ROUND_POINTS: 0, PICTO_FOUND_COUNT: 0, PICTO_FOUND_WORDS: [], PICTO_PASS_COUNT: 2 }, { creditPoints: 0 })
    expect(box('Bleus').checked).toBe(false)
  })

  it('crédit au niveau équipe seulement : aucune case de joueur', () => {
    reveal({ question: RP() })
    expect(screen.queryByRole('checkbox', { name: 'Sélectionner Alice' })).toBeNull()
  })

  it('issue POOL_END : l’équipe est présélectionnée comme pour un temps écoulé', () => {
    reveal({ question: RP(), PICTO_OUTCOME: 'POOL_END', PICTO_REVEALED_WORD: '' })
    expect(box('Bleus').checked).toBe(true)
  })
})

describe('GamePage — panneau Rafale Picto en manche (#318, maquette M7)', () => {
  beforeEach(() => { useGame.mockReset() })
  const live = (over = {}) => mk({ phase: 'STARTED', question: RP({ STATUS: 'STARTED' }), PICTO_OUTCOME: '', PICTO_REVEALED_WORD: '', PICTO_PASSES_LEFT: 1, ...over },
    { pictoWord: { WORD: 'Maison', EPOCH: 9, CATEGORY: 'GEOGRAPHY' } })

  it('« Trouvé » et « Passer » de l’admin émettent PICTO_FOUND / PICTO_PASS avec l’époque courante', () => {
    const g = live(); useGame.mockReturnValue(g)
    render(<GamePage />)
    fireEvent.click(screen.getByRole('button', { name: /^Trouvé/ }))
    expect(g.pictoFound).toHaveBeenCalledWith(9)
    fireEvent.click(screen.getByRole('button', { name: /Passer/ }))
    expect(g.pictoPass).toHaveBeenCalledWith(9)
  })

  it('le panneau montre le mot en cours, les mots trouvés et le montant du crédit ; jamais de mots à venir', () => {
    useGame.mockReturnValue(live())
    const { container } = render(<GamePage />)
    const text = container.textContent
    expect(text).toContain('Maison')
    expect(text).toContain('Chat')
    expect(text).toMatch(/25/)
    expect(text).not.toMatch(/à venir|prochain/i)
  })
})
