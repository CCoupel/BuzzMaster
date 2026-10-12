import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import GamePage from './GamePage'
import { useGame } from '../hooks/GameContext'

// Bug #255 — le sélecteur de réponses QCM comptait des équipes (ou joueurs) qui n'avaient PAS répondu : `ANSWER_COLOR`
// est assignée à l'affectation d'équipe et jamais remise à zéro, alors que `TIME` (instant du buzz) vaut 0 tant que le
// joueur n'a pas répondu. Seul un joueur avec TIME > 0 a donné sa réponse. Test de reproduction : rouge avant le
// correctif, `regression`, `critical`. Contrat : websocket-actions.md §« QCM : clic sur une réponse ».

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

vi.mock('./GamePage.css', () => ({}))




const ANSWERS = { RED: 'La Loire', GREEN: 'La Seine', YELLOW: 'Le Rhône', BLUE: 'La Garonne' }
// équipe → [joueur, couleur assignée, TIME (0 = n'a pas répondu), indices au buzz]
const bumpers = (rows) => Object.fromEntries(rows.map(([team, name, color, time, hints], i) => [`vj${i}`, {
  NAME: name, TEAM: team, IS_VIRTUAL: true, IS_VPLAYER: true, CONNECTED: true, SCORE: 0, TIME: time, ANSWER_COLOR: color, HINTS_AT_BUZZ: hints || 0,
}]))
const TEAMS = { 'Les Bleus': { NAME: 'Les Bleus', SCORE: 0, COLOR: [0, 0, 255] }, 'Les Rouges': { NAME: 'Les Rouges', SCORE: 0, COLOR: [255, 0, 0] }, 'Les Verts': { NAME: 'Les Verts', SCORE: 0, COLOR: [0, 255, 0] } }
const NO_ANSWER = [['Les Bleus', 'Paul', 'RED', 0], ['Les Rouges', 'Mamie', 'GREEN', 0], ['Les Verts', 'Vera', 'GREEN', 0]]
const QCM = (over = {}) => ({ ID: '9', TYPE: 'QCM', QUESTION: 'Quel fleuve traverse Paris ?', ANSWER: 'La Seine', POINTS: '3', QCM_ANSWERS: ANSWERS, QCM_CORRECT: 'GREEN', QCM_HINTS_ENABLED: false, QCM_WRONG_PENALTY: -2, STATUS: 'REVEALED', ...over })

const mk = (rows, over = {}) => {
  const { gameState: gsOver, ...rest } = over
  const question = gsOver?.question || QCM()
  return {
    gameState: { phase: 'REVEALED', question, remote: 'GAME', timer: 0, totalTime: 30, MEMORY_PARTICIPATING_TEAMS: [], ARDOISE_ANSWERS: {}, qcmInvalidated: [], ...(gsOver || {}) },
    teams: TEAMS, bumpers: bumpers(rows), questions: { [question.ID]: { ...question, STATUS: 'REVEALED', ORDER: 1 } },
    awardedTeams: {}, awardedPlayers: {}, creditPoints: 3,
    startGame: vi.fn(), stopGame: vi.fn(), pauseGame: vi.fn(), continueGame: vi.fn(), revealAnswer: vi.fn(), selectQuestion: vi.fn(),
    setRemoteDisplay: vi.fn(), setBumperPoints: vi.fn(), setTeamPoints: vi.fn(), forceReady: vi.fn(), simulateButton: vi.fn(), simulatePong: vi.fn(),
    sendMessage: vi.fn(() => true), newGame: vi.fn(), newGameRefused: null, clearNewGameRefused: vi.fn(),
    teamPointsBatch: vi.fn(() => true), teamPointsBatchResult: null, clearTeamPointsBatchResult: vi.fn(),
    bumperPointsBatch: vi.fn(() => true), bumperPointsBatchResult: null, clearBumperPointsBatchResult: vi.fn(),
    ...rest,
  }
}
const show = (rows, over = {}) => {
  const game = mk(rows, over)
  useGame.mockReturnValue(mk(rows, { ...over, gameState: { ...(over.gameState || {}), phase: 'STOPPED' } }))
  const utils = render(<GamePage />)
  const card = screen.queryByTestId(`question-card-${game.gameState.question.ID}`)
  if (card) fireEvent.click(card)
  useGame.mockReturnValue(game)
  utils.rerender(<GamePage />)
  return { game, ...utils }
}
const answerBtn = (label) => screen.getByRole('button', { name: new RegExp(label) })
const countOf = (label) => answerBtn(label).querySelector('.qcm-picker-n').textContent.trim()
const action = () => screen.getByRole('button', { name: /^(Attribuer les points|Appliquer les pénalités) \(\d+\)$/ })
const checkedBoxes = () => screen.queryAllByRole('checkbox').filter((c) => c.checked)
const ALL = ['La Loire', 'La Seine', 'Le Rhône', 'La Garonne']

describe('Bug #255 — le sélecteur QCM ne compte que les joueurs qui ont répondu', () => {
  beforeEach(() => { useGame.mockReset() })

  it.each([['équipe', {}], ['joueur', { POINTS_TARGET: 'PLAYER' }]])('Critical (1) — personne n’a répondu (ANSWER_COLOR assignée, TIME = 0) : « 0 » pour chaque réponse, niveau %s', (_n, q) => {
    show(NO_ANSWER, { gameState: { question: QCM(q) } })
    for (const label of ALL) expect(countOf(label), label).toMatch(/^0 /)
  })

  it.each([['équipe', {}], ['joueur', { POINTS_TARGET: 'PLAYER' }]])('Critical (2) — clic sur chaque réponse sans aucune réponse donnée : rien n’est sélectionné, compte 0, niveau %s', (_n, q) => {
    show(NO_ANSWER, { gameState: { question: QCM(q) } })
    for (const label of ALL) {
      fireEvent.click(answerBtn(label))
      expect(checkedBoxes(), label).toHaveLength(0)
      expect(action().textContent).toMatch(/\(0\)$/)
      expect(action().disabled).toBe(true)
    }
  })

  it.each([['équipe', {}, 'Les Rouges', 'Les Verts'], ['joueur', { POINTS_TARGET: 'PLAYER' }, 'Mamie', 'Vera']])('Critical (3) — mélange : A a répondu rouge, B non (verte assignée, TIME 0) → rouge = 1, verte = 0 ; clic sur rouge sélectionne A seulement, niveau %s', (_n, q, a, b) => {
    const rows = [['Les Rouges', 'Mamie', 'RED', 1234], ['Les Verts', 'Vera', 'GREEN', 0], ['Les Bleus', 'Paul', 'BLUE', 0]]
    show(rows, { gameState: { question: QCM(q) } })
    expect(countOf('La Loire')).toMatch(/^1 /)
    expect(countOf('La Seine')).toMatch(/^0 /)
    expect(countOf('La Garonne')).toMatch(/^0 /)
    fireEvent.click(answerBtn('La Loire'))
    expect(checkedBoxes().map((c) => c.getAttribute('aria-label'))).toEqual([`Sélectionner ${a}`])
    fireEvent.click(answerBtn('La Seine'))
    expect(checkedBoxes()).toHaveLength(0)
    expect(b).toBeTruthy()
  })

  it('Critical (4) — question suivante : TIME remis à 0, ANSWER_COLOR de la question précédente conservée → comptes à 0', () => {
    const answered = [['Les Rouges', 'Mamie', 'RED', 1234], ['Les Verts', 'Vera', 'GREEN', 2345], ['Les Bleus', 'Paul', 'GREEN', 3456]]
    const utils = show(answered)
    expect(countOf('La Seine')).toMatch(/^2 /)
    // question suivante : sélection (STOPPED) puis REVEAL ; les joueurs gardent leur ancienne couleur mais TIME = 0
    const next = QCM({ ID: '12' })
    const reset = [['Les Rouges', 'Mamie', 'RED', 0], ['Les Verts', 'Vera', 'GREEN', 0], ['Les Bleus', 'Paul', 'GREEN', 0]]
    useGame.mockReturnValue(mk(reset, { gameState: { question: next, phase: 'STOPPED' } }))
    utils.rerender(<GamePage />)
    const card = screen.queryByTestId('question-card-12')
    if (card) fireEvent.click(card)
    useGame.mockReturnValue(mk(reset, { gameState: { question: next } }))
    utils.rerender(<GamePage />)
    for (const label of ALL) expect(countOf(label), label).toMatch(/^0 /)
  })

  it.each([['équipe', {}], ['joueur', { POINTS_TARGET: 'PLAYER' }]])('(5) non-régression : une réponse effective (TIME > 0) est comptée comme avant, niveau %s', (_n, q) => {
    const rows = [['Les Bleus', 'Paul', 'GREEN', 1000], ['Les Rouges', 'Mamie', 'GREEN', 2000], ['Les Verts', 'Vera', 'RED', 3000]]
    const { game } = show(rows, { gameState: { question: QCM(q) } })
    expect(countOf('La Seine')).toMatch(/^2 /)
    expect(countOf('La Loire')).toMatch(/^1 /)
    expect(countOf('Le Rhône')).toMatch(/^0 /)
    fireEvent.click(answerBtn('La Seine'))
    expect(checkedBoxes()).toHaveLength(2)
    expect(action().textContent).toBe('Attribuer les points (2)')
    expect(game.teamPointsBatch).not.toHaveBeenCalled()
  })

  it('(5) non-régression : avec indices, une réponse effective garde son montant d’indices (3 / 2 / 1)', () => {
    const rows = [['Les Bleus', 'Paul', 'GREEN', 1000, 0], ['Les Rouges', 'Mamie', 'GREEN', 2000, 1], ['Les Verts', 'Vera', 'GREEN', 3000, 2]]
    const { game } = show(rows, { gameState: { question: QCM({ QCM_HINTS_ENABLED: true }), qcmInvalidated: ['RED', 'BLUE'] } })
    fireEvent.click(answerBtn('La Seine'))
    fireEvent.click(action())
    expect(Object.fromEntries(game.teamPointsBatch.mock.calls[0][1].map((e) => [e.team, e.points]))).toEqual({ 'Les Bleus': 3, 'Les Rouges': 2, 'Les Verts': 1 })
  })

  it('un joueur qui n’a pas répondu n’est jamais sélectionné par « mauvaise réponse » non plus (pénalité)', () => {
    const rows = [['Les Rouges', 'Mamie', 'RED', 1234], ['Les Verts', 'Vera', 'RED', 0]]
    show(rows)
    fireEvent.click(answerBtn('La Loire'))
    expect(checkedBoxes().map((c) => c.getAttribute('aria-label'))).toEqual(['Sélectionner Les Rouges'])
    expect(action().textContent).toBe('Appliquer les pénalités (1)')
  })
})
