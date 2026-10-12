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



const Q = (id, type = 'QCM', over = {}) => ({ ID: String(id), TYPE: type, QUESTION: `Q${id}`, ANSWER: 'A', POINTS: '1', TIME: '30', ORDER: Number(id), STATUS: 'AVAILABLE', ...over })
const ROUNDS = [{ ID: 'r1', NAME: 'Échauffement' }, { ID: 'r2', NAME: 'Finale' }]
const QUESTIONS = Object.fromEntries([
  Q(5), Q(1, 'QCM', { ROUND_ID: 'r1' }), Q(2, 'ENTRACTE', { ROUND_ID: 'r1', QUESTION: 'Fin de la manche Échauffement' }), Q(3, 'QCM', { ROUND_ID: 'r2' }),
].map((q) => [q.ID, q]))

const mk = (over = {}) => {
  const { gameState: gs, ...rest } = over
  return {
    gameState: { phase: 'STOPPED', question: QUESTIONS[1], remote: 'GAME', timer: 0, totalTime: 30, qcmInvalidated: [], MEMORY_PARTICIPATING_TEAMS: [], ARDOISE_ANSWERS: {}, ROUNDS, ...(gs || {}) },
    teams: {}, bumpers: {}, questions: QUESTIONS, awardedTeams: {}, awardedPlayers: {}, creditPoints: 3,
    startGame: vi.fn(), stopGame: vi.fn(), pauseGame: vi.fn(), continueGame: vi.fn(), revealAnswer: vi.fn(), selectQuestion: vi.fn(),
    setRemoteDisplay: vi.fn(), setBumperPoints: vi.fn(), setTeamPoints: vi.fn(), forceReady: vi.fn(), simulateButton: vi.fn(), simulatePong: vi.fn(),
    sendMessage: vi.fn(() => true), newGame: vi.fn(), newGameRefused: null, clearNewGameRefused: vi.fn(),
    teamPointsBatch: vi.fn(() => true), teamPointsBatchResult: null, clearTeamPointsBatchResult: vi.fn(),
    bumperPointsBatch: vi.fn(() => true), bumperPointsBatchResult: null, clearBumperPointsBatchResult: vi.fn(),
    ...rest,
  }
}
const show = (over) => { const g = mk(over); useGame.mockReturnValue(g); return { g, ...render(<GamePage />) } }
const nextBtn = () => document.querySelector('.next-question-btn')

// #245 (lot V3-3) — page de jeu : titres de manche repliables dans la liste, « à suivre » qui PROPOSE les entractes et
// signale le changement de manche (maquette (g‴)).
describe('GamePage — manches (#245, lot V3-3)', () => {
  beforeEach(() => { useGame.mockReset() })

  it('Critical — titres de manche : « Sans manche » en premier, puis les manches ; compteur « n/m jouées »', () => {
    show({ questions: { ...QUESTIONS, 1: { ...QUESTIONS[1], STATUS: 'STOPPED' } } })
    const titles = [...document.querySelectorAll('.game-round-title')].map((n) => n.textContent)
    expect(titles).toEqual(['Sans manche', 'Manche 1 · Échauffement', 'Manche 2 · Finale'])
    const counts = [...document.querySelectorAll('.game-round-count')].map((n) => n.textContent)
    expect(counts).toEqual(['0/1 jouées', '1/1 jouées', '0/1 jouées'])
    const order = [...document.querySelectorAll('[data-testid^="question-card-"]')].map((n) => n.getAttribute('data-testid').replace('question-card-', ''))
    expect(order).toEqual(['5', '1', '2', '3'])
  })

  it('repli d’une manche : ses cartes disparaissent, le titre reste', () => {
    show()
    fireEvent.click(screen.getByRole('button', { name: 'Replier Manche 2 · Finale' }))
    expect(screen.queryByTestId('question-card-3')).toBeNull()
    expect(screen.getByRole('button', { name: 'Déplier Manche 2 · Finale' })).toBeTruthy()
  })

  it('sans manche : aucun titre, liste inchangée', () => {
    show({ gameState: { ROUNDS: [] } })
    expect(document.querySelectorAll('.game-round-title')).toHaveLength(0)
    expect(screen.getByTestId('question-card-1')).toBeTruthy()
  })

  it('Critical — « à suivre » PROPOSE une ENTRACTE (plus de saut) : « 🎬 titre », style violet', () => {
    show()
    expect(nextBtn().textContent).toMatch(/À suivre : 🎬 Fin de la manche Échauffement/)
    expect(nextBtn().className).toMatch(/nq-entracte/)
  })

  it('Critical — la question suivante est dans une autre manche : « → Manche 2 · Finale »', () => {
    show({ gameState: { question: QUESTIONS[2] } })
    expect(nextBtn().textContent).toMatch(/À suivre : #3/)
    expect(nextBtn().textContent).toMatch(/→ Manche 2 · Finale/)
  })

  it('même manche : pas de pastille de changement', () => {
    show() // question 1 (Échauffement) → à suivre = l'entracte de la même manche
    expect(document.querySelector('.nq-newround')).toBeNull()
  })

  it('cliquer l’entracte dans la liste la sélectionne (lancement par clic)', () => {
    const { g } = show()
    fireEvent.click(screen.getByTestId('question-card-2'))
    expect(g.selectQuestion).toHaveBeenCalled()
  })
})
