import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, act } from '@testing-library/react'
import confetti from 'canvas-confetti'
import PlayerDisplay from './PlayerDisplay'

// #252 (v11.2, lot V2-3) — écrans TV / VJoueur : une hausse de score accompagnée de SCORE_CHANGE.DISPLAY = false
// (correction cachée) est affichée SANS aucune animation (pas de « +X pts », ni célébration/confettis, ni surbrillance,
// ni montée de rang) ; la valeur est pourtant à jour. La hausse visible suivante n'anime QUE sa propre différence.
// Contrat : game-state.md §SCORE_CHANGE (règle d'affichage). Scénarios 8 et 9 du plan (planner-20261011-213000.md).

vi.mock('nosleep.js', () => ({
  default: class NoSleep {
    enable() { return Promise.resolve() }
    disable() {}
  },
}))

vi.mock('canvas-confetti', () => ({ default: vi.fn() }))

vi.mock('../hooks/GameContext', () => ({
  useGame: vi.fn(),
  GameProvider: ({ children }) => children,
}))

vi.mock('../components/Timer', () => ({
  default: ({ currentTime }) => <div data-testid="timer">{currentTime}</div>,
}))

vi.mock('../components/Podium', () => ({
  default: () => <div data-testid="podium" />,
}))

vi.mock('../components/QRCodeOverlay', () => ({
  default: () => null,
}))

vi.mock('../components/QRCodeDisplay', () => ({
  default: () => null,
}))

vi.mock('./QuestionsPage', () => ({
  CATEGORIES: [],
}))

vi.mock('../constants/colors', () => ({
  getCategoryColor: vi.fn(() => '#8b5cf6'),
}))

vi.mock('../utils/colorUtils', () => ({
  getRgbColor: vi.fn((color) => (Array.isArray(color) ? `rgb(${color.join(',')})` : color)),
}))

vi.mock('./PlayerDisplay.css', () => ({}))
vi.mock('../styles/neon.css', () => ({}))



import { useGame } from '../hooks/GameContext'

const COLORS = { 'Les Bleus': [0, 0, 255], 'Les Rouges': [255, 0, 0] }
const mkTeams = (bleus, rouges) => ({
  'Les Bleus': { NAME: 'Les Bleus', SCORE: bleus, COLOR: COLORS['Les Bleus'] },
  'Les Rouges': { NAME: 'Les Rouges', SCORE: rouges, COLOR: COLORS['Les Rouges'] },
})
const BUMPERS = {
  a: { TEAM: 'Les Bleus', NAME: 'Paul', IS_VPLAYER: true, CONNECTED: true, SCORE: 0, COLOR: [0, 0, 255] },
  b: { TEAM: 'Les Rouges', NAME: 'Mamie', IS_VPLAYER: true, CONNECTED: true, SCORE: 0, COLOR: [255, 0, 0] },
}
const sc = (seq, display, teams = [], players = []) => ({ SEQ: seq, DISPLAY: display, TEAMS: teams, PLAYERS: players })

const mock = ({ remote = 'GAME', teams, bumpers = BUMPERS, scoreChange = sc(0, true) }) => ({
  gameState: {
    phase: 'STOPPED', remote, timer: 0, totalTime: 30, question: null, ARDOISE_ANSWERS: {},
    MEMORY_PARTICIPATING_TEAMS: [], MEMOTION_PARTICIPATING_TEAMS: [], MEMOTION_CARD_STATES: {}, MEMOTION_CARD_TEAMS: {},
    MEMOTION_CURRENT_TEAM: null, MEMOTION_SELECTED: null, newGameBackgrounds: [], SCORE_CHANGE: scoreChange,
  },
  teams, bumpers, flipMemoryCard: vi.fn(), showQRCode: false, selectMotionCard: vi.fn(),
})

// monte l'écran avec un état de départ, puis rejoue un UPDATE
function mount(over, isVPlayer = false) {
  useGame.mockReturnValue(mock(over))
  const props = isVPlayer ? { isVPlayer: true, playerName: 'Paul', playerNameColor: [0, 0, 255], teamName: 'Les Bleus', teamColor: [0, 0, 255] } : {}
  const utils = render(<PlayerDisplay {...props} />)
  return {
    ...utils,
    update: (next) => { useGame.mockReturnValue(mock(next)); act(() => { utils.rerender(<PlayerDisplay {...props} />) }) },
  }
}
const pointsValues = (c) => [...c.querySelectorAll('.points-animation .points-value')].map((e) => e.textContent)

describe('PlayerDisplay — SCORE_CHANGE : hausse visible = comportement actuel (#252)', () => {
  beforeEach(() => { vi.clearAllMocks(); vi.useFakeTimers(); document.documentElement.requestFullscreen = vi.fn().mockResolvedValue(undefined) })

  it('Critical — hausse avec DISPLAY:true : « +X pts » et célébration', () => {
    const m = mount({ teams: mkTeams(10, 5) })
    m.update({ teams: mkTeams(13, 5), scoreChange: sc(1, true, ['Les Bleus']) })
    expect(pointsValues(m.container)).toEqual(['+3 pts'])
    expect(confetti).toHaveBeenCalled()
  })

  it('compatibilité : hausse SANS SCORE_CHANGE (serveur antérieur) = animée comme avant', () => {
    const m = mount({ teams: mkTeams(10, 5) })
    const next = mock({ teams: mkTeams(14, 5) })
    delete next.gameState.SCORE_CHANGE
    useGame.mockReturnValue(next)
    act(() => { m.rerender(<PlayerDisplay />) })
    expect(pointsValues(m.container)).toEqual(['+4 pts'])
  })
})

describe('PlayerDisplay — correction cachée (SCORE_CHANGE.DISPLAY = false) (#252)', () => {
  beforeEach(() => { vi.clearAllMocks(); vi.useFakeTimers(); document.documentElement.requestFullscreen = vi.fn().mockResolvedValue(undefined) })

  it('Critical — hausse cachée : AUCUN « +X pts », AUCUN confetti, mais le score est bien à jour', () => {
    const m = mount({ teams: mkTeams(10, 5) })
    m.update({ teams: mkTeams(13, 5), scoreChange: sc(1, false, ['Les Bleus']) })

    expect(pointsValues(m.container)).toEqual([])
    expect(confetti).not.toHaveBeenCalled()
    expect(m.container.querySelector('.points-animation')).toBeNull()
  })

  it('Critical — la hausse visible suivante n’anime QUE sa propre différence (pas celle de la correction cachée)', () => {
    const m = mount({ teams: mkTeams(10, 5) })
    m.update({ teams: mkTeams(13, 5), scoreChange: sc(1, false, ['Les Bleus']) }) // +3 caché
    m.update({ teams: mkTeams(15, 5), scoreChange: sc(2, true, ['Les Bleus']) })  // +2 visible

    expect(pointsValues(m.container)).toEqual(['+2 pts']) // jamais +5
    expect(confetti).toHaveBeenCalledTimes(1)
  })

  it('un nouvel UPDATE qui ne change pas le SEQ (ex. tick du chrono) ne rejoue ni n’annule rien', () => {
    const m = mount({ teams: mkTeams(10, 5) })
    m.update({ teams: mkTeams(13, 5), scoreChange: sc(1, false, ['Les Bleus']) })
    m.update({ teams: mkTeams(13, 5), scoreChange: sc(1, false, ['Les Bleus']) }) // même SEQ rediffusé
    expect(pointsValues(m.container)).toEqual([])
    expect(confetti).not.toHaveBeenCalled()
  })

  it('une correction cachée ne couvre que les équipes listées : une autre équipe qui monte reste animée', () => {
    const m = mount({ teams: mkTeams(10, 5) })
    m.update({ teams: mkTeams(13, 9), scoreChange: sc(1, false, ['Les Bleus']) }) // Rouges +4 non listée
    expect(pointsValues(m.container)).toEqual(['+4 pts'])
  })

  it('hausse cachée en vue SCORE : aucune surbrillance de ligne ni montée de rang, même pour l’équipe dépassée', () => {
    const m = mount({ remote: 'SCORE', teams: mkTeams(10, 12) })
    m.update({ remote: 'SCORE', teams: mkTeams(20, 12), scoreChange: sc(1, false, ['Les Bleus']) }) // Bleus dépassent les Rouges
    expect(m.container.querySelectorAll('.score-changed').length).toBe(0)
    expect(m.container.querySelectorAll('.rank-up').length).toBe(0)
    expect(m.container.textContent).toMatch(/20/) // la nouvelle valeur est affichée
  })

  it('hausse visible en vue SCORE : surbrillance (comportement actuel)', () => {
    const m = mount({ remote: 'SCORE', teams: mkTeams(10, 12) })
    m.update({ remote: 'SCORE', teams: mkTeams(20, 12), scoreChange: sc(1, true, ['Les Bleus']) })
    expect(m.container.querySelectorAll('.score-changed').length).toBeGreaterThan(0)
  })

  it('vue JOUEURS : un score de joueur modifié en caché n’est pas surligné', () => {
    const players0 = { ...BUMPERS }
    const players1 = { ...BUMPERS, a: { ...BUMPERS.a, SCORE: 5 } }
    const m = mount({ remote: 'PLAYERS', teams: mkTeams(0, 0), bumpers: players0 })
    m.update({ remote: 'PLAYERS', teams: mkTeams(0, 0), bumpers: players1, scoreChange: sc(1, false, ['Les Bleus'], ['a']) })
    expect(m.container.querySelectorAll('.player-changed, .score-changed, .rank-up').length).toBe(0)
  })

  it('VJoueur (tablette) : même règle — hausse cachée sans « +X pts » ni confettis', () => {
    const m = mount({ teams: mkTeams(10, 5) }, true)
    m.update({ teams: mkTeams(13, 5), scoreChange: sc(1, false, ['Les Bleus']) })
    expect(pointsValues(m.container)).toEqual([])
    expect(confetti).not.toHaveBeenCalled()
  })
})
