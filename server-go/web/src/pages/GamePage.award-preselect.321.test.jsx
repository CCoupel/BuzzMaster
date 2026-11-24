import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import GamePage from './GamePage'
import { useGame } from '../hooks/GameContext'

// #321 (v13.1.0, LOT-1) — présélection des gagnants à l'entrée en REVEALED (contracts/award-preselect.md §1, §2, §4).
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
  Verts: { NAME: 'Verts', SCORE: 0, COLOR: [0, 255, 0] },
}
const BUMPERS = {
  b1: { NAME: 'J1', TEAM: 'Bleus', IS_VIRTUAL: true, IS_VPLAYER: true, CONNECTED: true, SCORE: 0, ANSWER_COLOR: 'GREEN', TIME: 3000 },
  b2: { NAME: 'J2', TEAM: 'Rouges', IS_VIRTUAL: true, IS_VPLAYER: true, CONNECTED: true, SCORE: 0, ANSWER_COLOR: 'RED', TIME: 1500 },
  b3: { NAME: 'J3', TEAM: 'Verts', IS_VIRTUAL: true, IS_VPLAYER: true, CONNECTED: true, SCORE: 0, ANSWER_COLOR: 'GREEN', TIME: 2000 },
}
const mk = (gameState, over = {}) => ({
  gameState: { phase: 'REVEALED', remote: 'GAME', timer: 0, totalTime: 30, MEMORY_PARTICIPATING_TEAMS: [], ARDOISE_ANSWERS: {}, ...gameState },
  teams: TEAMS, bumpers: BUMPERS, questions: { [gameState.question.ID]: { ...gameState.question, ORDER: 1 } },
  awardedTeams: {}, awardedPlayers: {}, creditPoints: 3,
  startGame: vi.fn(), stopGame: vi.fn(), pauseGame: vi.fn(), continueGame: vi.fn(), revealAnswer: vi.fn(), selectQuestion: vi.fn(),
  setRemoteDisplay: vi.fn(), setBumperPoints: vi.fn(), setTeamPoints: vi.fn(), forceReady: vi.fn(), simulateButton: vi.fn(), simulatePong: vi.fn(),
  sendMessage: vi.fn(), newGame: vi.fn(), newGameRefused: null, clearNewGameRefused: vi.fn(),
  teamPointsBatch: vi.fn(() => true), teamPointsBatchResult: null, clearTeamPointsBatchResult: vi.fn(),
  ...over,
})
// Parcours réel : sélection de la question en STOPPED, puis bascule en REVEALED (l'entrée en REVEALED déclenche la présélection)
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
const checked = () => Object.keys(TEAMS).filter((n) => screen.queryByRole('checkbox', { name: `Sélectionner ${n}` })?.checked)

describe('GamePage — présélection à l’entrée en REVEALED (#321)', () => {
  beforeEach(() => { useGame.mockReset() })

  it('QCM : les équipes ayant donné la bonne réponse sont cochées d’office', () => {
    reveal({ question: { ID: '1', TYPE: 'QCM', QUESTION: 'Q', ANSWER: 'x', POINTS: '3', QCM_ANSWERS: { RED: 'a', GREEN: 'b' }, QCM_CORRECT: 'GREEN', STATUS: 'REVEALED' } })
    expect(checked().sort()).toEqual(['Bleus', 'Verts'])
  })

  it('SPEEDY niveau équipe : le premier buzzeur est coché (équipe de plus petit TIME)', () => {
    reveal({ question: { ID: '2', TYPE: 'SPEEDY', QUESTION: 'Q', ANSWER: 'x', POINTS: '3', POINTS_TARGET: 'TEAM', STATUS: 'REVEALED' } })
    expect(checked()).toEqual(['Rouges'])
  })

  it('ARDOISE : les équipes dont la réponse repliée égale ANSWER sont cochées', () => {
    reveal({ question: { ID: '3', TYPE: 'ARDOISE', QUESTION: 'Q', ANSWER: 'Éléphant', POINTS: '2', STATUS: 'REVEALED' }, ARDOISE_ANSWERS: { Bleus: { TEXT: 'elephant' }, Rouges: { TEXT: 'girafe' } } })
    expect(checked()).toEqual(['Bleus'])
  })

  it('la sélection reste modifiable (décocher / « Tout sélectionner ») et rien n’est envoyé avant « Attribuer »', () => {
    const { game } = reveal({ question: { ID: '1', TYPE: 'QCM', QUESTION: 'Q', ANSWER: 'x', POINTS: '3', QCM_ANSWERS: { GREEN: 'b' }, QCM_CORRECT: 'GREEN', STATUS: 'REVEALED' } })
    fireEvent.click(box('Bleus'))
    expect(checked()).toEqual(['Verts'])
    expect(game.teamPointsBatch).not.toHaveBeenCalled()
    expect(game.setTeamPoints).not.toHaveBeenCalled()
  })

  it('une unité déjà créditée n’est pas cochée', () => {
    reveal({ question: { ID: '1', TYPE: 'QCM', QUESTION: 'Q', ANSWER: 'x', POINTS: '3', QCM_ANSWERS: { GREEN: 'b' }, QCM_CORRECT: 'GREEN', STATUS: 'REVEALED' } },
      { awardedTeams: { Bleus: { POINTS: 3 } } })
    expect(checked()).toEqual(['Verts'])
  })

  it('ROUE et ENTRACTE : aucune case cochée', () => {
    reveal({ question: { ID: '4', TYPE: 'ROUE', QUESTION: 'Q', POINTS: '3', STATUS: 'REVEALED' } })
    expect(checked()).toEqual([])
  })

  it('changement de question : la sélection est vidée (comportement existant conservé)', () => {
    const { rerender } = reveal({ question: { ID: '1', TYPE: 'QCM', QUESTION: 'Q', ANSWER: 'x', POINTS: '3', QCM_ANSWERS: { GREEN: 'b' }, QCM_CORRECT: 'GREEN', STATUS: 'REVEALED' } })
    expect(checked().length).toBe(2)
    useGame.mockReturnValue(mk({ phase: 'STOPPED', question: { ID: '5', TYPE: 'QCM', QUESTION: 'Q2', ANSWER: 'y', POINTS: '3', QCM_ANSWERS: { GREEN: 'b' }, QCM_CORRECT: 'GREEN', STATUS: 'STOPPED' } }))
    rerender(<GamePage />)
    expect(checked()).toEqual([])
  })

  it('modification manuelle du champ « Points » après REVEALED : ne recoche ni ne décoche rien', () => {
    reveal({ question: { ID: '1', TYPE: 'QCM', QUESTION: 'Q', ANSWER: 'x', POINTS: '3', QCM_ANSWERS: { GREEN: 'b' }, QCM_CORRECT: 'GREEN', STATUS: 'REVEALED' } })
    fireEvent.click(box('Bleus'))
    const input = screen.getAllByLabelText('Points').find((el) => el.tagName === 'INPUT' && el.type === 'number')
    fireEvent.change(input, { target: { value: '7' } })
    expect(checked()).toEqual(['Verts'])
  })
})

describe('GamePage — MEMOTION : gagnant de carte présélectionné (#321, §4)', () => {
  beforeEach(() => { useGame.mockReset() })
  const MOTION = {
    phase: 'STARTED',
    question: { ID: 'm1', TYPE: 'MEMOTION', STATUS: 'READY', MOTION_CARDS: [{ ID: 'c1', RECTO_THEME: 'Thème', ANSWER_TEXT: 'Rép', DIFFICULTY: 1 }] },
    MEMOTION_SUBPHASE: 'REVEAL', MEMOTION_SELECTED: 'c1', MEMOTION_CARD_STATES: { c1: 'FLIPPED' }, MEMOTION_CARD_TEAMS: {},
    MEMOTION_CURRENT_TEAM: 'Rouges', MEMOTION_CURRENT_TEAM_COLOR: [255, 0, 0], MEMOTION_PARTICIPATING_TEAMS: ['Bleus', 'Rouges'],
  }
  const chip = (name) => screen.getByRole('button', { name: new RegExp(`^(✓ )?${name}$`) })
  const validate = () => screen.getByRole('button', { name: 'Valider' })

  it('en REVEAL, l’équipe qui a la main est présélectionnée (aria-pressed) et rien n’est envoyé au rendu', () => {
    const game = mk(MOTION); useGame.mockReturnValue(game); render(<GamePage />)
    expect(chip('Rouges').getAttribute('aria-pressed')).toBe('true')
    expect(game.sendMessage).not.toHaveBeenCalledWith('MEMOTION_DONE', expect.anything())
  })

  it('le choix reste modifiable : « Perdu » puis « Valider » envoie {CARD_ID, WINNER_TEAM: ""} (payload inchangé)', () => {
    const game = mk(MOTION); useGame.mockReturnValue(game); render(<GamePage />)
    fireEvent.click(chip('Perdu'))
    expect(game.sendMessage).not.toHaveBeenCalledWith('MEMOTION_DONE', expect.anything())
    fireEvent.click(validate())
    const calls = game.sendMessage.mock.calls.filter((c) => c[0] === 'MEMOTION_DONE')
    expect(calls.length).toBe(1)
    expect(Object.keys(calls[0][1]).sort()).toEqual(['CARD_ID', 'WINNER_TEAM'])
    expect(calls[0][1]).toEqual({ CARD_ID: 'c1', WINNER_TEAM: '' })
  })

  it('sans toucher à la sélection, « Valider » envoie l’équipe qui a la main', () => {
    const game = mk(MOTION); useGame.mockReturnValue(game); render(<GamePage />)
    fireEvent.click(validate())
    expect(game.sendMessage).toHaveBeenCalledWith('MEMOTION_DONE', { CARD_ID: 'c1', WINNER_TEAM: 'Rouges' })
  })

  it('SOLO : l’équipe unique qui a la main est présélectionnée', () => {
    const game = mk({ ...MOTION, MEMOTION_PARTICIPATING_TEAMS: ['Rouges'] }); useGame.mockReturnValue(game); render(<GamePage />)
    expect(chip('Rouges').getAttribute('aria-pressed')).toBe('true')
  })
})
