import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import AnimPage from './AnimPage'
import { useGame } from '../hooks/GameContext'
import { useCategories } from '../hooks/useCategories'

// #321 (v13.1.0, LOT-1) — MEMOTION sur l'anim : le sélecteur de gagnant part sur l'équipe qui a la main (§4).
// Le payload de doneMotionCard(cardId, winnerTeam) est inchangé ; rien n'est envoyé avant le clic de validation.

vi.mock('nosleep.js', () => ({ default: class NoSleep { enable() { return Promise.resolve() } disable() {} } }))
vi.mock('../hooks/GameContext', () => ({ useGame: vi.fn(), GameProvider: ({ children }) => children }))
vi.mock('../hooks/useCategories', () => ({ useCategories: vi.fn() }))
vi.mock('../components/Timer', () => ({ default: () => <div /> }))
vi.mock('../components/AnimArdoiseList', () => ({ default: () => <div /> }))
vi.mock('./AnimPage.css', () => ({}))
vi.mock('../components/AnimConductPanel.css', () => ({}))
vi.mock('../components/LaunchGameHost.css', () => ({}))
vi.mock('../styles/entracte.css', () => ({}))

const game = (currentTeam) => ({
  status: 'connected',
  gameState: {
    phase: 'STARTED', timer: 30, totalTime: 30, gameTime: 0, entracte: false,
    entracteConfig: { TITLE: 'E', SUBTITLE: '', IMAGE_IS_CUSTOM: false, PANEL_SIZE: 65, ANIM_PERIOD: 10, ANIM_INTENSITY: 20 },
    question: { ID: 'm1', TYPE: 'MEMOTION', STATUS: 'READY', MOTION_CARDS: [{ ID: 'c1', RECTO_THEME: 'Thème', ANSWER_TEXT: 'Rép', DIFFICULTY: 1 }] },
    MEMOTION_SUBPHASE: 'REVEAL', MEMOTION_SELECTED: 'c1', MEMOTION_CARD_STATES: { c1: 'FLIPPED' }, MEMOTION_CARD_TEAMS: {},
    MEMOTION_CURRENT_TEAM: currentTeam, MEMOTION_CURRENT_TEAM_COLOR: [255, 0, 0], MEMOTION_PARTICIPATING_TEAMS: ['Bleus', 'Rouges'],
  },
  teams: { Bleus: { NAME: 'Bleus', COLOR: [0, 0, 255], SCORE: 0 }, Rouges: { NAME: 'Rouges', COLOR: [255, 0, 0], SCORE: 0 } },
  bumpers: {}, nextQuestion: null, questionPosition: { position: 0, total: 0 }, awardedTeams: {}, creditPoints: 0,
  startGame: vi.fn(), stopGame: vi.fn(), pauseGame: vi.fn(), continueGame: vi.fn(), revealAnswer: vi.fn(), selectQuestion: vi.fn(),
  setTeamPoints: vi.fn(), setBumperPoints: vi.fn(), flipMemoryCard: vi.fn(),
  regieMessage: { ACTIVE: false, TEXT: '', SENT_AT: 0, CLEARED_BY: '' }, clearRegieMessage: vi.fn(),
  selectMotionCard: vi.fn(), flipMotionCard: vi.fn(), stopMotionTimer: vi.fn(), revealMotionCard: vi.fn(), doneMotionCard: vi.fn(),
})

const btn = (re) => screen.getByRole('button', { name: re })
const teamChip = (name) => screen.getAllByRole('button', { name: new RegExp(name) }).find((b) => b.hasAttribute('aria-pressed'))

describe('AnimPage — MEMOTION : gagnant présélectionné (#321)', () => {
  beforeEach(() => { useCategories.mockReturnValue({ categories: [] }) })

  it('l’équipe qui a la main est présélectionnée (aria-pressed) ; aucun envoi au rendu', () => {
    const g = game('Rouges'); useGame.mockReturnValue(g); render(<AnimPage />)
    expect(teamChip('Rouges').getAttribute('aria-pressed')).toBe('true')
    expect(g.doneMotionCard).not.toHaveBeenCalled()
  })

  it('« VALIDER » sans autre geste envoie (cardId, équipe qui a la main)', () => {
    const g = game('Rouges'); useGame.mockReturnValue(g); render(<AnimPage />)
    fireEvent.click(btn(/VALIDER/))
    expect(g.doneMotionCard).toHaveBeenCalledWith('c1', 'Rouges')
  })

  it('« PERSONNE » reste sélectionnable ; VALIDER envoie (cardId, "") — payload inchangé', () => {
    const g = game('Rouges'); useGame.mockReturnValue(g); render(<AnimPage />)
    fireEvent.click(btn(/PERSONNE/))
    expect(g.doneMotionCard).not.toHaveBeenCalled()
    fireEvent.click(btn(/VALIDER/))
    expect(g.doneMotionCard).toHaveBeenCalledWith('c1', '')
  })

  it('seuls l’équipe qui a la main et « PERSONNE » sont proposés (règle de jeu existante) ; pas d’autre équipe', () => {
    const g = game('Rouges'); useGame.mockReturnValue(g); render(<AnimPage />)
    expect(screen.queryByRole('button', { name: /Bleus/ })).toBeNull()
    expect(teamChip('Rouges')).toBeTruthy()
    expect(btn(/PERSONNE/).hasAttribute('aria-pressed')).toBe(true)
  })
})
