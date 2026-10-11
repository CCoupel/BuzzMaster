import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, fireEvent, act } from '@testing-library/react'
import GamePage from './GamePage'
import { useGame } from '../hooks/GameContext'

// #254 (v11.2, lot V2-4) — attribution groupée : bloc compact (2 lignes, sans montant propre, icônes accessibles),
// niveau équipe OU joueur selon la question, montant = champ « Points » de la page, clic sur une réponse QCM
// (pénalité de la question, 0 = bouton désactivé avec la raison), commutateur « Tenir compte des indices »,
// base du barème. Maquettes VALIDÉES : admin_game_award__compact_block_hints_toggle,
// admin_game_award__team_or_player_level_hints_toggle. Contrat : websocket-actions.md §TEAM_POINTS_BATCH
// (« Niveau d'attribution », « Montant », « QCM : clic sur une réponse », « SET_CREDIT_POINTS ≤ 0 »).
// Scénarios front 10 à 16, 19, 22 à 31 du plan. Boîte noire par libellés / rôles ARIA.

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
// équipe → [joueur, couleur de réponse, indices au buzz]
const ROWS = [['Les Bleus', 'Paul', 'GREEN', 0], ['Les Rouges', 'Mamie', 'GREEN', 1], ['Les Jaunes', 'Jade', 'RED', 0], ['Les Verts', 'Vera', 'GREEN', 2]]
const TEAMS = Object.fromEntries(ROWS.map(([n], i) => [n, { NAME: n, SCORE: 10 + i, COLOR: [20 * i, 80, 160] }]))
const bumpers = (rows = ROWS) => Object.fromEntries(rows.map(([team, name, color, hints], i) => [`vj${i}`, {
  NAME: name, TEAM: team, IS_VIRTUAL: true, IS_VPLAYER: true, CONNECTED: true, SCORE: 0, TIME: 1000 + i, ANSWER_COLOR: color || '', HINTS_AT_BUZZ: hints,
}]))
const QCM = (over = {}) => ({ ID: '9', TYPE: 'QCM', QUESTION: 'Quel fleuve traverse Paris ?', ANSWER: 'La Seine', POINTS: '3', QCM_ANSWERS: ANSWERS, QCM_CORRECT: 'GREEN', QCM_HINTS_ENABLED: false, QCM_WRONG_PENALTY: 0, STATUS: 'REVEALED', ...over })

const mk = (over = {}) => {
  const { gameState: gsOver, ...rest } = over
  const question = gsOver?.question || QCM()
  return {
    gameState: { phase: 'REVEALED', question, remote: 'GAME', timer: 0, totalTime: 30, MEMORY_PARTICIPATING_TEAMS: [], ARDOISE_ANSWERS: {}, qcmInvalidated: [], ...(gsOver || {}) },
    teams: TEAMS, bumpers: bumpers(), questions: { [question.ID]: { ...question, STATUS: 'REVEALED', ORDER: 1 } },
    awardedTeams: {}, awardedPlayers: {}, creditPoints: 3,
    startGame: vi.fn(), stopGame: vi.fn(), pauseGame: vi.fn(), continueGame: vi.fn(), revealAnswer: vi.fn(), selectQuestion: vi.fn(),
    setRemoteDisplay: vi.fn(), setBumperPoints: vi.fn(), setTeamPoints: vi.fn(), forceReady: vi.fn(), simulateButton: vi.fn(), simulatePong: vi.fn(),
    sendMessage: vi.fn(() => true), newGame: vi.fn(), newGameRefused: null, clearNewGameRefused: vi.fn(),
    teamPointsBatch: vi.fn(() => true), teamPointsBatchResult: null, clearTeamPointsBatchResult: vi.fn(),
    bumperPointsBatch: vi.fn(() => true), bumperPointsBatchResult: null, clearBumperPointsBatchResult: vi.fn(),
    ...rest,
  }
}
// Parcours réel : sélection de la question (clic sur sa carte), puis passage en REVEALED (la base du barème est fixée à
// l'entrée en REVEALED). `before(game)` permet d'agir en STOPPED (ex. régler « Points » avant le REVEAL).
const show = (over = {}, before) => {
  const game = mk(over)
  useGame.mockReturnValue(mk({ ...over, gameState: { ...(over.gameState || {}), phase: 'STOPPED' } }))
  const utils = render(<GamePage />)
  const card = screen.queryByTestId(`question-card-${game.gameState.question.ID}`)
  if (card) fireEvent.click(card)
  if (before) before(game)
  useGame.mockReturnValue(game)
  utils.rerender(<GamePage />)
  return { game, ...utils }
}
const reshow = (utils, game, patch) => {
  const next = { ...game, ...patch, gameState: { ...game.gameState, ...(patch.gameState || {}) } }
  useGame.mockReturnValue(next)
  utils.rerender(<GamePage />)
  return next
}
const box = (name) => screen.getByRole('checkbox', { name: `Sélectionner ${name}` })
const queryBox = (name) => screen.queryByRole('checkbox', { name: `Sélectionner ${name}` })
const checkedOf = (names) => names.filter((n) => queryBox(n)?.checked)
const TEAM_NAMES = ROWS.map((r) => r[0])
const PLAYER_NAMES = ROWS.map((r) => r[1])
const action = () => screen.getByRole('button', { name: /^(Attribuer les points|Appliquer les pénalités) \(\d+\)$/ })
const queryAction = () => screen.queryByRole('button', { name: /^(Attribuer les points|Appliquer les pénalités) \(\d+\)$/ })
const pointsField = () => screen.getAllByLabelText('Points').find((el) => el.tagName === 'INPUT' && el.type === 'number')
const setPoints = (v) => fireEvent.change(pointsField(), { target: { value: String(v) } })
const answerBtn = (label) => screen.getByRole('button', { name: new RegExp(label) })

// #254 (revue) — pas de double crédit équipe puis joueur : au niveau JOUEUR, les joueurs d'une équipe créditée au niveau
// ÉQUIPE (AWARDED_TEAMS.TEAMS[].TEAM_LEVEL) sont grisés « ✓ équipe déjà créditée » ; contrat d14d9eaa.
describe('Niveau joueur — équipe déjà créditée au niveau équipe', () => {
  beforeEach(() => { useGame.mockReset() })
  const lvl = { 'Les Rouges': { POINTS: 3, TIMESTAMP: 1, TEAM_LEVEL: true } }

  it('Critical — le joueur est grisé, non cochable, « ✓ équipe déjà créditée », ignoré par « Tout sélectionner »', () => {
    show({ gameState: { question: QCM({ POINTS_TARGET: 'PLAYER' }) }, awardedTeams: lvl })
    expect(queryBox('Mamie').disabled).toBe(true)
    expect(queryBox('Mamie').checked).toBe(false)
    expect(screen.getByText('✓ équipe déjà créditée')).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: 'Tout sélectionner' }))
    expect(checkedOf(PLAYER_NAMES).sort()).toEqual(['Jade', 'Paul', 'Vera'])
  })

  it('équipe créditée seulement par ses joueurs (TEAM_LEVEL false) : joueurs non grisés', () => {
    show({ gameState: { question: QCM({ POINTS_TARGET: 'PLAYER' }) }, awardedTeams: { 'Les Rouges': { POINTS: 3, TIMESTAMP: 1, TEAM_LEVEL: false } } })
    expect(queryBox('Mamie').disabled).toBe(false)
    expect(screen.queryByText('✓ équipe déjà créditée')).toBeNull()
  })

  it('un clic sur une réponse ne sélectionne pas les joueurs d’une équipe déjà créditée', () => {
    show({ gameState: { question: QCM({ POINTS_TARGET: 'PLAYER' }) }, awardedTeams: lvl })
    fireEvent.click(answerBtn('La Seine'))
    expect(checkedOf(PLAYER_NAMES).sort()).toEqual(['Paul', 'Vera'])
  })

  it('résultat : TEAM_ALREADY_AWARDED → « N joueurs ignorés (équipe déjà créditée) »', () => {
    const utils = show({ gameState: { question: QCM({ POINTS_TARGET: 'PLAYER' }) } })
    const game = mk({ gameState: { question: QCM({ POINTS_TARGET: 'PLAYER' }) } })
    useGame.mockReturnValue({ ...game, bumperPointsBatchResult: {
      status: 'APPLIED', questionId: '9', applied: [{ id: 'vj0', points: 3 }],
      skipped: [{ id: 'vj1', reason: 'TEAM_ALREADY_AWARDED' }, { id: 'vj3', reason: 'TEAM_ALREADY_AWARDED' }], players: [],
    } })
    act(() => { utils.rerender(<GamePage />) })
    expect(screen.getByText(/2 joueurs ignorés \(équipe déjà créditée\)/)).toBeTruthy()
  })
})

// #255 — la couleur assignée sans buzz n'est pas une réponse
describe('QCM : équipes sans réponse non comptées (#255)', () => {
  beforeEach(() => { useGame.mockReset() })
  it('Critical — ANSWER_COLOR assignée mais TIME 0 : compteur 0 et aucune sélection au clic', () => {
    const noBuzz = Object.fromEntries(Object.entries(bumpers()).map(([k, b]) => [k, { ...b, TIME: 0 }]))
    show({ bumpers: noBuzz })
    fireEvent.click(answerBtn('La Seine'))
    expect(checkedOf(TEAM_NAMES)).toEqual([])
    expect(screen.queryByText(/(^|[^0-9])[1-9]\d* équipes?/)).toBeNull()
  })
  it('seules les équipes ayant buzzé sont comptées', () => {
    const b = bumpers()
    b.vj0 = { ...b.vj0, TIME: 0 }
    show({ bumpers: b })
    fireEvent.click(answerBtn('La Seine'))
    expect(checkedOf(TEAM_NAMES).sort()).toEqual(['Les Rouges', 'Les Verts'])
  })
})
