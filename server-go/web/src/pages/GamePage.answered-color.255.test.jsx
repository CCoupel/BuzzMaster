import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import GamePage from './GamePage'
import { useGame } from '../hooks/GameContext'

// #255 (v11.2) — l'admin lit ANSWERED_COLOR (réponse effective à la question courante, "" sinon) ; ANSWER_COLOR (couleur
// assignée / dernière pressée) ne sert plus à savoir qui a répondu. Repli : bumper SANS la clé → TIME > 0 && ANSWER_COLOR
// (correctif c516431a). Contrats : models.md §Bumper, websocket-actions.md §« QCM : clic sur une réponse ».
// Scénarios front 9, 10, 11 et 14 du plan (_work/reports/planner-20261012-171500.md). `critical` : 9.

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
const TEAMS = { 'Les Bleus': { NAME: 'Les Bleus', SCORE: 0, COLOR: [0, 0, 255] }, 'Les Rouges': { NAME: 'Les Rouges', SCORE: 0, COLOR: [255, 0, 0] }, 'Les Verts': { NAME: 'Les Verts', SCORE: 0, COLOR: [0, 255, 0] } }
// [équipe, joueur, ANSWER_COLOR (assignée), ANSWERED_COLOR (undefined = clé absente), TIME, indices au buzz]
const mkBumpers = (rows) => Object.fromEntries(rows.map(([team, name, assigned, answered, time, hints], i) => [`vj${i}`, {
  NAME: name, TEAM: team, IS_VIRTUAL: true, IS_VPLAYER: true, CONNECTED: true, SCORE: 0, ANSWER_COLOR: assigned, TIME: time, HINTS_AT_BUZZ: hints || 0,
  ...(answered === undefined ? {} : { ANSWERED_COLOR: answered }),
}]))
const QCM = (over = {}) => ({ ID: '9', TYPE: 'QCM', QUESTION: 'Quel fleuve traverse Paris ?', ANSWER: 'La Seine', POINTS: '3', QCM_ANSWERS: ANSWERS, QCM_CORRECT: 'GREEN', QCM_HINTS_ENABLED: false, QCM_WRONG_PENALTY: -2, STATUS: 'REVEALED', ...over })
const mk = (rows, over = {}) => {
  const { gameState: gsOver, ...rest } = over
  const question = gsOver?.question || QCM()
  return {
    gameState: { phase: 'REVEALED', question, remote: 'GAME', timer: 0, totalTime: 30, MEMORY_PARTICIPATING_TEAMS: [], ARDOISE_ANSWERS: {}, qcmInvalidated: [], ...(gsOver || {}) },
    teams: TEAMS, bumpers: mkBumpers(rows), questions: { [question.ID]: { ...question, STATUS: 'REVEALED', ORDER: 1 } },
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
const checkedLabels = () => screen.queryAllByRole('checkbox').filter((c) => c.checked).map((c) => c.getAttribute('aria-label'))
const LEVELS = [['équipe', {}], ['joueur', { POINTS_TARGET: 'PLAYER' }]]

describe('ANSWERED_COLOR — le sélecteur et les montants lisent la réponse effective (#255)', () => {
  beforeEach(() => { useGame.mockReset() })

  it.each(LEVELS)('Critical (9) — couleur ASSIGNÉE mais ANSWERED_COLOR "" (même avec un TIME > 0 résiduel) : compteurs à 0 et aucune sélection au clic, niveau %s', (_n, q) => {
    const rows = [['Les Bleus', 'Paul', 'RED', '', 1234], ['Les Rouges', 'Mamie', 'GREEN', '', 0], ['Les Verts', 'Vera', 'GREEN', '', 0]]
    show(rows, { gameState: { question: QCM(q) } })
    for (const label of ['La Loire', 'La Seine', 'Le Rhône', 'La Garonne']) {
      expect(countOf(label), label).toMatch(/^0 /)
      fireEvent.click(answerBtn(label))
      expect(checkedLabels(), label).toEqual([])
      expect(action().disabled).toBe(true)
    }
  })

  it.each(LEVELS)('(10) réponses effectives (ANSWERED_COLOR posée) : compteurs, sélection et montants corrects, niveau %s', (_n, q) => {
    const rows = [['Les Bleus', 'Paul', 'RED', 'GREEN', 1000], ['Les Rouges', 'Mamie', 'GREEN', 'RED', 2000], ['Les Verts', 'Vera', 'RED', 'GREEN', 3000]]
    const { game } = show(rows, { gameState: { question: QCM(q) } })
    expect(countOf('La Seine')).toMatch(/^2 /)
    expect(countOf('La Loire')).toMatch(/^1 /)
    fireEvent.click(answerBtn('La Seine'))
    expect(action().textContent).toBe('Attribuer les points (2)')
    fireEvent.click(action())
    const calls = (q.POINTS_TARGET ? game.bumperPointsBatch : game.teamPointsBatch).mock.calls
    expect(calls[0][1].map((e) => e.points)).toEqual([3, 3])
  })

  it('(10) la couleur répondue (ANSWERED_COLOR) prime sur la couleur assignée pour le choix de la sélection', () => {
    const rows = [['Les Bleus', 'Paul', 'RED', 'GREEN', 1000]] // assigné RED, a répondu GREEN
    show(rows)
    expect(countOf('La Seine')).toMatch(/^1 /)
    expect(countOf('La Loire')).toMatch(/^0 /)
  })

  it('(10) montants d’indices selon la réponse effective : 3 / 2 / 1 (calcQcmTeamAward)', () => {
    const rows = [['Les Bleus', 'Paul', 'RED', 'GREEN', 1000, 0], ['Les Rouges', 'Mamie', 'RED', 'GREEN', 2000, 1], ['Les Verts', 'Vera', 'RED', 'GREEN', 3000, 2]]
    const { game } = show(rows, { gameState: { question: QCM({ QCM_HINTS_ENABLED: true }), qcmInvalidated: ['YELLOW', 'BLUE'] } })
    fireEvent.click(answerBtn('La Seine'))
    fireEvent.click(action())
    expect(Object.fromEntries(game.teamPointsBatch.mock.calls[0][1].map((e) => [e.team, e.points]))).toEqual({ 'Les Bleus': 3, 'Les Rouges': 2, 'Les Verts': 1 })
  })

  it.each(LEVELS)('(11) repli : bumpers SANS la clé ANSWERED_COLOR → TIME > 0 && ANSWER_COLOR (correctif c516431a), niveau %s', (_n, q) => {
    const rows = [['Les Bleus', 'Paul', 'GREEN', undefined, 1000], ['Les Rouges', 'Mamie', 'GREEN', undefined, 0], ['Les Verts', 'Vera', 'RED', undefined, 3000]]
    show(rows, { gameState: { question: QCM(q) } })
    expect(countOf('La Seine')).toMatch(/^1 /) // Mamie : assignée GREEN mais TIME 0 → n'a pas répondu
    expect(countOf('La Loire')).toMatch(/^1 /)
    fireEvent.click(answerBtn('La Seine'))
    expect(checkedLabels()).toHaveLength(1)
  })

  it('(14) non-régression : la carte d’équipe garde la couleur ASSIGNÉE (ANSWER_COLOR) sur le joueur, même sans réponse (ANSWERED_COLOR vide)', () => {
    const rows = [['Les Bleus', 'Paul', 'RED', '', 0], ['Les Rouges', 'Mamie', 'BLUE', '', 0]]
    const { container } = show(rows)
    const marks = container.querySelectorAll('.buzzer-mini.has-answer-color')
    expect(marks.length).toBe(2)
    expect([...marks].map((m) => m.style.getPropertyValue('--answer-color')).every(Boolean)).toBe(true)
  })
})
