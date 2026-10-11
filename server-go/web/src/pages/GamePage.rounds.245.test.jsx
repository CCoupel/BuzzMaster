import { describe, it, expect, beforeEach, vi } from 'vitest'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, resolve } from 'node:path'
import { render, screen, fireEvent, within } from '@testing-library/react'
import GamePage from './GamePage'
import { useGame } from '../hooks/GameContext'

// #245 (v11.2, lot V3-3) — page de jeu admin selon la maquette VALIDÉE (g‴) : titres de manche dans la liste (« Sans manche » en
// premier), « À suivre » PROPOSE les entractes (« À suivre : 🎬 … », violet), « → Manche 2 · … » au changement de manche,
// entrées jouées sautées, `nextUnplayedQuestion` = ordre du serveur (fixture partagée tests/fixtures/rounds_order_245.json,
// EXPECTED_ORDER écrit à la main). Une entracte se lance par clic sur sa ligne. Scénarios 18, 33, 39. `critical` : 33.

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




const here = dirname(fileURLToPath(import.meta.url))
const fixture = JSON.parse(readFileSync(resolve(here, '../../../tests/fixtures/rounds_order_245.json'), 'utf8'))
const withText = (q) => ({ QUESTION: q.TYPE === 'ENTRACTE' ? `Pause ${q.ID}` : `Question ${q.ID}`, ANSWER: 'a', POINTS: '1', TIME: '30', STATUS: 'AVAILABLE', CATEGORY: 'CULTURE', ...q })
const QUESTIONS = Object.fromEntries(fixture.QUESTIONS.map((q) => [q.ID, withText(q)]))
// sans manche (v11.1) : seules les questions à ORDER entier et ID numérique ont un ordre v11.1 défini
const V111_IDS = ['10', '11', '12', '20', '21', '22', '30', '31', '32', '40', '41']

const mk = (currentId, over = {}) => {
  const { statuses = {}, rounds = fixture.ROUNDS, phase = 'STOPPED' } = over
  const questions = Object.fromEntries(Object.entries(QUESTIONS).map(([id, q]) => [id, { ...q, STATUS: statuses[id] || q.STATUS }]))
  if (over.noRounds) Object.values(questions).forEach((q) => { delete q.ROUND_ID })
  return {
    gameState: { phase, question: currentId ? questions[currentId] : null, remote: 'GAME', timer: 0, totalTime: 30, MEMORY_PARTICIPATING_TEAMS: [], ARDOISE_ANSWERS: {}, qcmInvalidated: [], ROUNDS: over.noRounds ? [] : rounds },
    teams: {}, bumpers: {}, questions,
    awardedTeams: {}, awardedPlayers: {}, creditPoints: 1,
    startGame: vi.fn(), stopGame: vi.fn(), pauseGame: vi.fn(), continueGame: vi.fn(), revealAnswer: vi.fn(), selectQuestion: vi.fn(),
    setRemoteDisplay: vi.fn(), setBumperPoints: vi.fn(), setTeamPoints: vi.fn(), forceReady: vi.fn(), simulateButton: vi.fn(), simulatePong: vi.fn(),
    sendMessage: vi.fn(() => true), newGame: vi.fn(), newGameRefused: null, clearNewGameRefused: vi.fn(),
    teamPointsBatch: vi.fn(() => true), teamPointsBatchResult: null, clearTeamPointsBatchResult: vi.fn(),
    bumperPointsBatch: vi.fn(() => true), bumperPointsBatchResult: null, clearBumperPointsBatchResult: vi.fn(),
  }
}
let game
const show = (currentId, over) => { game = mk(currentId, over); useGame.mockReturnValue(game); return render(<GamePage />) }
const nextBtn = () => document.querySelector('.next-question-btn')

describe('Page de jeu — liste en manches (#245, scénario 18)', () => {
  beforeEach(() => { useGame.mockReset() })

  it('Critical — les cartes suivent l’ordre de jeu partagé : « Sans manche » d’abord, puis les manches', () => {
    show('12')
    const order = [...document.querySelectorAll('[data-testid^="question-card-"]')].map((n) => n.getAttribute('data-testid').replace('question-card-', ''))
    expect(order).toEqual(fixture.EXPECTED_ORDER)
  })

  it('titres de manche : « Sans manche », « Manche 1 · Un », « Manche 2 · Deux », dans cet ordre', () => {
    show('12')
    const titles = [...document.querySelectorAll('.game-round-title')].map((n) => n.textContent)
    expect(titles).toEqual(['Sans manche', 'Manche 1 · Un', 'Manche 2 · Deux'])
  })

  it('un groupe se replie (bouton de bascule) et masque ses cartes', () => {
    show('12')
    const head = [...document.querySelectorAll('.game-round-head')].find((h) => h.textContent.includes('Manche 1'))
    fireEvent.click(within(head).getByRole('button'))
    expect(screen.queryByTestId('question-card-31')).toBeNull()
    expect(screen.getByTestId('question-card-12')).toBeTruthy()
  })

  it('Critical — sans manche : liste identique à v11.1, aucun titre de manche', () => {
    game = mk('12', { noRounds: true })
    Object.keys(game.questions).forEach((id) => { if (!V111_IDS.includes(id)) delete game.questions[id] })
    useGame.mockReturnValue(game)
    render(<GamePage />)
    expect(document.querySelector('.game-round-title')).toBeNull()
    const order = [...document.querySelectorAll('[data-testid^="question-card-"]')].map((n) => n.getAttribute('data-testid').replace('question-card-', ''))
    expect(order).toEqual([...V111_IDS].sort((a, b) => {
      const oa = QUESTIONS[a].ORDER ?? Number(a), ob = QUESTIONS[b].ORDER ?? Number(b)
      return oa - ob || Number(a) - Number(b)
    }))
  })
})

describe('Page de jeu — « À suivre » (#245, scénarios 33 et 39)', () => {
  beforeEach(() => { useGame.mockReset() })

  it('Critical (33) — de chaque entrée à la suivante : le clic sélectionne l’entrée suivante de l’ordre partagé, ENTRACTE comprises', () => {
    for (let i = 0; i + 1 < fixture.EXPECTED_ORDER.length; i++) {
      useGame.mockReset()
      const { unmount } = show(fixture.EXPECTED_ORDER[i])
      expect(nextBtn(), `après ${fixture.EXPECTED_ORDER[i]}`).not.toBeNull()
      fireEvent.click(nextBtn())
      expect(game.selectQuestion, `après ${fixture.EXPECTED_ORDER[i]}`).toHaveBeenCalledWith(fixture.EXPECTED_ORDER[i + 1])
      unmount()
    }
  })

  it('après la dernière entrée : plus de bouton « À suivre »', () => {
    show(fixture.EXPECTED_ORDER[fixture.EXPECTED_ORDER.length - 1])
    expect(nextBtn()).toBeNull()
  })

  it('l’entracte proposée s’écrit « À suivre : 🎬 <titre> » (violet) ; une question ordinaire reste « à suivre : #ID »', () => {
    show('30') // suivante = 32 (ENTRACTE de fin de manche 1)
    expect(nextBtn().textContent).toMatch(/[Àà] suivre : 🎬 Pause 32/)
    expect(nextBtn().className).toMatch(/nq-entracte/)
    useGame.mockReset()
    document.body.innerHTML = ''
    show('31') // suivante = 30
    expect(nextBtn().textContent).toMatch(/[Àà] suivre : #30/)
    expect(nextBtn().className).not.toMatch(/nq-entracte/)
  })

  it('une entrée déjà jouée reste sautée (32 jouée : après 30, on propose 21)', () => {
    show('30', { statuses: { 32: 'STOPPED' } })
    fireEvent.click(nextBtn())
    expect(game.selectQuestion).toHaveBeenCalledWith('21')
  })

  it('changement de manche signalé : « → Manche 2 · Deux » ; rien dans la même manche', () => {
    show('32', { statuses: { 32: 'REVEALED' } }) // suivante = 21 (début de manche 2)
    expect(nextBtn().textContent).toMatch(/→ Manche 2 · Deux/)
    useGame.mockReset()
    document.body.innerHTML = ''
    show('31') // suivante = 30, même manche
    expect(nextBtn().textContent).not.toMatch(/→/)
  })

  it('« Sans manche » → première manche : « → Manche 1 · Un »', () => {
    show('61') // dernière entrée de « Sans manche » dans la fixture étendue
    expect(nextBtn().textContent).toMatch(/→ Manche 1 · Un/)
  })

  it('(32) une ENTRACTE de la liste se lance par clic sur sa ligne (info-bulle)', () => {
    show('12')
    const row = document.querySelector('.game-entracte-row')
    expect(row.getAttribute('title')).toBe('Entracte : cliquer pour la lancer')
    fireEvent.click(within(row).getByTestId(/question-card-/))
    expect(game.selectQuestion).toHaveBeenCalledWith(expect.stringMatching(/^(21|32)$/))
  })
})
