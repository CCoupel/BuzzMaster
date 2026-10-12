import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, within, fireEvent } from '@testing-library/react'
import AnimPage from './AnimPage'
import { useGame } from '../hooks/GameContext'
import { useCategories } from '../hooks/useCategories'

// #245 (v11.2, lot V3-3) — /anim selon la maquette VALIDÉE (g‴) game_round_indicator__admin_anim_tv_next_entracte : pastille de manche
// (« Manche 2/3 · nom · 3/5 », « · entracte », « Sans manche · 2/4 », rien sans manche), bouton « Manche suivante : … »,
// « Lancer l'entracte » (la suivante est une ENTRACTE), « Terminer l'entracte » (seulement pendant une entracte) ; aucun bouton de
// déclenchement manuel. Scénarios 19, 34 (UI), 39. Le libellé normal (« À suivre ») est inchangé (#166).

vi.mock('nosleep.js', () => ({
  default: class NoSleep { constructor() { this.isEnabled = false } enable() { this.isEnabled = true; return Promise.resolve() } disable() { this.isEnabled = false } },
}))
vi.mock('../hooks/GameContext', () => ({ useGame: vi.fn(), GameProvider: ({ children }) => children }))
vi.mock('../hooks/useCategories', () => ({ useCategories: vi.fn() }))
vi.mock('../components/Timer', () => ({ default: () => <div data-testid="timer" /> }))
vi.mock('../components/AnimAnswerZone', () => ({ default: () => <div data-testid="answer-zone" /> }))
vi.mock('../components/AnimArdoiseList', () => ({ default: () => <div data-testid="ardoise-list" /> }))
vi.mock('./AnimPage.css', () => ({}))
vi.mock('../styles/entracte.css', () => ({}))

function makeGameMock(overrides = {}) {
  return {
    status: 'connected',
    gameState: {
      phase: 'STOPPED',
      timer: 30,
      totalTime: 30,
      gameTime: 0,
      question: null,
      entracte: false,
      entracteConfig: { TITLE: 'ENTRACTE', SUBTITLE: '', IMAGE_IS_CUSTOM: false, PANEL_SIZE: 65, ANIM_PERIOD: 10, ANIM_INTENSITY: 20 },
      ...overrides.gameState,
    },
    teams: {},
    bumpers: {},
    nextQuestion: null,
    questionPosition: { position: 0, total: 0 },
    awardedTeams: {},
    creditPoints: 0,
    startGame: vi.fn(),
    stopGame: vi.fn(),
    pauseGame: vi.fn(),
    continueGame: vi.fn(),
    revealAnswer: vi.fn(),
    selectQuestion: vi.fn(),
    setTeamPoints: vi.fn(),
    setBumperPoints: vi.fn(),
    flipMemoryCard: vi.fn(),
    regieMessage: { ACTIVE: false, TEXT: '', SENT_AT: 0, CLEARED_BY: '' },
    clearRegieMessage: vi.fn(),
    selectMotionCard: vi.fn(),
    flipMotionCard: vi.fn(),
    stopMotionTimer: vi.fn(),
    revealMotionCard: vi.fn(),
    doneMotionCard: vi.fn(),
    ...overrides,
  }
}



const ROUNDS = [{ ID: 'r1', NAME: 'Un' }, { ID: 'r2', NAME: 'Deux' }, { ID: 'r3', NAME: 'Trois' }]
const ctx = (over = {}) => ({ ROUND_ID: 'r2', ROUND_INDEX: 2, ROUND_COUNT: 3, POSITION: 3, TOTAL: 5, FIRST_OF_ROUND: false, ...over })
const mount = (gs = {}, extra = {}) => {
  const g = makeGameMock({ gameState: { phase: 'STOPPED', question: { ID: '9', TYPE: 'SPEEDY', QUESTION: 'Q ?', STATUS: 'STOPPED' }, ROUNDS, ROUND_CONTEXT: ctx(), ...gs }, setEntracte: vi.fn(), selectQuestion: vi.fn(), ...extra })
  useGame.mockReturnValue(g)
  return { g, ...render(<AnimPage />) }
}
beforeEach(() => { useCategories.mockReturnValue({ categories: [] }) })

describe('AnimPage — pastille de manche (#245, scénario 19)', () => {
  it('Critical — « Manche 2/3 · Deux · 3/5 » (positions du serveur, sans les entractes)', () => {
    mount()
    expect(screen.getByText('Manche 2/3 · Deux · 3/5')).toBeInTheDocument()
  })

  it('une ENTRACTE de manche (POSITION 0) : « Manche 2/3 · Deux · entracte »', () => {
    mount({ ROUND_CONTEXT: ctx({ POSITION: 0 }) })
    expect(screen.getByText('Manche 2/3 · Deux · entracte')).toBeInTheDocument()
  })

  it('question sans manche alors que le quiz en compte : « Sans manche · 2/4 »', () => {
    mount({ ROUND_CONTEXT: ctx({ ROUND_ID: '', ROUND_INDEX: 0, POSITION: 2, TOTAL: 4 }) })
    expect(screen.getByText('Sans manche · 2/4')).toBeInTheDocument()
  })

  it('Critical — quiz sans manche (ROUND_COUNT 0) : aucune pastille de manche', () => {
    mount({ ROUNDS: [], ROUND_CONTEXT: ctx({ ROUND_ID: '', ROUND_INDEX: 0, ROUND_COUNT: 0, POSITION: 0, TOTAL: 0 }) })
    expect(screen.queryByText(/Manche \d\/\d/)).toBeNull()
    expect(screen.queryByText(/Sans manche ·/)).toBeNull()
  })
})

describe('AnimPage — « suivant », « Manche suivante », « Lancer l\'entracte » (#245, scénarios 34 et 39)', () => {
  it('Critical — la suivante est une ENTRACTE : bouton « Lancer l\'entracte » avec son libellé ; un clic envoie le READY de cette entrée (cycle habituel)', () => {
    const { g } = mount({}, { nextQuestion: { ID: '12', TYPE: 'ENTRACTE', QUESTION: 'Fin de la manche Deux', ROUND_ID: 'r2', ROUND_NAME: 'Deux', NEW_ROUND: false } })
    const btn = screen.getByRole('button', { name: /Lancer l'entracte/ })
    expect(btn.textContent).toMatch(/Fin de la manche Deux/)
    fireEvent.click(btn)
    expect(g.selectQuestion).toHaveBeenCalledWith('12')
  })

  it('la suivante ouvre une autre manche : « Manche suivante : Trois »', () => {
    const { g } = mount({}, { nextQuestion: { ID: '20', TYPE: 'SPEEDY', QUESTION: 'Q', ROUND_ID: 'r3', ROUND_NAME: 'Trois', NEW_ROUND: true } })
    const btn = screen.getByRole('button', { name: /Manche suivante : Trois/ })
    fireEvent.click(btn)
    expect(g.selectQuestion).toHaveBeenCalledWith('20')
  })

  it('NEW_ROUND vers « Sans manche » : « Manche suivante : Sans manche »', () => {
    mount({}, { nextQuestion: { ID: '3', TYPE: 'SPEEDY', QUESTION: 'Q', ROUND_ID: '', ROUND_NAME: '', NEW_ROUND: true } })
    expect(screen.getByRole('button', { name: /Manche suivante : Sans manche/ })).toBeTruthy()
  })

  it('une entracte proposée prime sur le changement de manche (« Lancer l\'entracte »)', () => {
    mount({}, { nextQuestion: { ID: '21', TYPE: 'ENTRACTE', QUESTION: 'Début de la manche Trois', ROUND_ID: 'r3', ROUND_NAME: 'Trois', NEW_ROUND: true } })
    expect(screen.getByRole('button', { name: /Lancer l'entracte/ })).toBeTruthy()
    expect(screen.queryByRole('button', { name: /Manche suivante/ })).toBeNull()
  })

  it('suivante ordinaire dans la même manche : ni « Lancer l\'entracte » ni « Manche suivante »', () => {
    mount({}, { nextQuestion: { ID: '10', TYPE: 'SPEEDY', QUESTION: 'Q ?', ROUND_ID: 'r2', ROUND_NAME: 'Deux', NEW_ROUND: false } })
    expect(screen.queryByRole('button', { name: /Lancer l'entracte|Manche suivante/ })).toBeNull()
  })
})

describe('AnimPage — « Terminer l\'entracte » (#245, scénarios 34, 36, 39)', () => {
  it('Critical — pendant une entracte (GameState.ENTRACTE) : « Terminer l\'entracte » envoie ENTRACTE_SET{ACTIVE:false} (setEntracte(false))', () => {
    const { g } = mount({ entracte: true, ENTRACTE: true, phase: 'STARTED', question: { ID: '11', TYPE: 'ENTRACTE', QUESTION: 'Début de la manche Deux' } })
    fireEvent.click(screen.getByRole('button', { name: /Terminer l'entracte/ }))
    expect(g.setEntracte).toHaveBeenCalledWith(false)
  })

  it('Critical — hors entracte : aucun bouton « Terminer l\'entracte »', () => {
    mount({ entracte: false, ENTRACTE: false })
    expect(screen.queryByRole('button', { name: /Terminer l'entracte/ })).toBeNull()
  })

  it('Critical — l\'anim n\'a AUCUN bouton de déclenchement manuel d\'une entracte (actif ou non)', () => {
    for (const entracte of [false, true]) {
      const { unmount } = mount({ entracte, ENTRACTE: entracte })
      const hits = screen.queryAllByRole('button').filter((b) => /entracte/i.test(b.textContent) && !/Terminer|Lancer l'entracte/.test(b.textContent))
      expect(hits).toHaveLength(0)
      expect(screen.queryByRole('button', { name: /Activer|Déclencher|Démarrer l'entracte/i })).toBeNull()
      unmount()
    }
  })
})

describe('AnimPage — libellé du bouton « suivant » selon la maquette (g‴) (#245, scénario 39)', () => {
  it('Critical — suivante ordinaire dans la même manche : « Question suivante » (libellé de la maquette), sous-libellé = la question', () => {
    const { g } = mount({}, { nextQuestion: { ID: '10', TYPE: 'SPEEDY', QUESTION: 'Quel fleuve ?', ROUND_ID: 'r2', ROUND_NAME: 'Deux', NEW_ROUND: false } })
    const btn = screen.getByRole('button', { name: /Question suivante/ })
    expect(btn.textContent).toMatch(/Quel fleuve/)
    fireEvent.click(btn)
    expect(g.selectQuestion).toHaveBeenCalledWith('10')
  })

  it('« Fin du quiz » quand il n’y a plus de suivante', () => {
    mount({}, { nextQuestion: null })
    expect(screen.getByRole('button', { name: /Fin du quiz|fin du quiz/i })).toBeTruthy()
  })
})

