import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { render, act } from '@testing-library/react'
import PlayerDisplay from './PlayerDisplay'
import { useGame } from '../hooks/GameContext'

// #194 (v13.0.0, LOT-2.3, contrat picto.md rév. 8 §6.5, §15) — révélation et crédit d'une manche PICTO sur la TV et les
// tablettes : le MOT SEUL à la révélation, ni « Trouvé par … » ni « +N pts » propres à Picto ; le crédit se voit par
// l'animation commune des points (TvPointsAward, #265) déclenchée par SCORE_CHANGE.GAINS — sur la TV, et sur les seules
// tablettes de l'équipe créditée ; un crédit de 0 point ne déclenche AUCUNE animation et la TV n'affiche aucun texte de
// points (revue M2). Scaffold : PlayerDisplay.points-award-reference.265.test.jsx.

vi.mock('nosleep.js', () => ({ default: class { enable() { return Promise.resolve() } disable() {} } }))
vi.mock('../hooks/GameContext', () => ({ useGame: vi.fn(), GameProvider: ({ children }) => children }))
vi.mock('../components/Timer', () => ({ default: ({ currentTime }) => <div data-testid="timer">{currentTime}</div> }))
vi.mock('../components/Podium', () => ({ default: () => <div data-testid="podium" /> }))
vi.mock('../components/QRCodeOverlay', () => ({ default: () => null }))
vi.mock('../components/QRCodeDisplay', () => ({ default: () => null }))
vi.mock('./QuestionsPage', () => ({ CATEGORIES: [] }))
vi.mock('../constants/colors', () => ({ getCategoryColor: vi.fn(() => '#8b5cf6') }))
vi.mock('../utils/colorUtils', () => ({ getRgbColor: vi.fn((c) => (Array.isArray(c) ? `rgb(${c.join(',')})` : c)) }))
vi.mock('./PlayerDisplay.css', () => ({}))
vi.mock('../styles/neon.css', () => ({}))
vi.mock('../components/TvPointsAward.css', () => ({}))

const L = 3200
const WORD = 'Zèbre-secret'
const teamsOf = (scores) => Object.fromEntries(Object.entries(scores).map(([n, s]) => [n, { NAME: n, SCORE: s, COLOR: n === 'Bleue' ? [0, 0, 255] : [255, 0, 0] }]))
const sc = (seq, display, gains) => ({ SEQ: seq, DISPLAY: display, TEAMS: gains.map((g) => g.TEAM), PLAYERS: [], GAINS: gains })

function game({ teams, scoreChange, status = 'connected', own, revealed = true }) {
  return {
    status,
    gameState: {
      phase: revealed ? 'REVEALED' : 'STARTED', remote: 'GAME', timer: 0, totalTime: 90,
      question: { ID: 'pq1', TYPE: 'PICTO', CATEGORY: 'ANIMALS', POINTS: 10, TIME: 90 },
      PICTO_TEAM: 'Bleue', PICTO_TEAM_COLOR: [0, 0, 255], PICTO_DRAWER: 'b1', PICTO_DRAWER_NAME: 'Alice', PICTO_DRAWER_CONNECTED: true,
      PICTO_CANDIDATES: [], PICTO_WORDS_COUNT: 3, PICTO_CANVAS_EPOCH: 2,
      PICTO_OUTCOME: revealed ? 'FOUND' : '', PICTO_REVEALED_WORD: revealed ? WORD : '', PICTO_REVEALED_CATEGORY: revealed ? 'ANIMALS' : '',
      ARDOISE_ANSWERS: {}, MEMORY_PARTICIPATING_TEAMS: [], MEMOTION_PARTICIPATING_TEAMS: [], MEMOTION_CARD_STATES: {}, MEMOTION_CARD_TEAMS: {},
      MEMOTION_CURRENT_TEAM: null, MEMOTION_SELECTED: null, newGameBackgrounds: [], SCORE_CHANGE: scoreChange,
    },
    teams,
    bumpers: own ? { a: { TEAM: own, NAME: 'Paul', IS_VPLAYER: true, CONNECTED: true, SCORE: 0, COLOR: [0, 0, 255] } } : {},
    pictoCanvasStore: null, pictoWord: null, pictoError: null,
    flipMemoryCard: vi.fn(), showQRCode: false, selectMotionCard: vi.fn(), sendMessage: vi.fn(),
  }
}

const awards = (container) => [...container.querySelectorAll('*')]
  .filter((el) => el.children.length === 0 && /^\s*\+\d+ pts\s*$/.test(el.textContent))
  .map((el) => el.textContent.trim())
const tick = (ms) => act(() => { vi.advanceTimersByTime(ms) })
const BASE = { Bleue: 0, Rouge: 0 }

function mount({ vplayer, own }) {
  let current = { teams: teamsOf(BASE), scoreChange: sc(1, true, []) } // 1er état = référence, sans animation (#265)
  const props = vplayer
    ? { isVPlayer: true, playerName: 'Paul', playerNameColor: [0, 0, 255], teamName: own, teamColor: [0, 0, 255] }
    : {}
  useGame.mockReturnValue(game({ ...current, own }))
  const utils = render(<PlayerDisplay {...props} />)
  const push = (patch) => {
    current = { ...current, ...patch }
    useGame.mockReturnValue(game({ ...current, own }))
    act(() => { utils.rerender(<PlayerDisplay {...props} />) })
  }
  return { ...utils, push }
}

beforeEach(() => {
  vi.clearAllMocks()
  vi.useFakeTimers()
  document.documentElement.requestFullscreen = vi.fn().mockResolvedValue(undefined)
})
afterEach(() => { vi.useRealTimers() })

describe('TV — révélation PICTO : le mot seul (§6.5 rév. 8)', () => {
  it('REVEALED : le mot dans la zone réponse, ni « Trouvé par », ni points, ni « +10 pts » propre à Picto', () => {
    const { container } = mount({ vplayer: false, own: null })
    const answers = container.querySelector('.zone-answers')
    expect(answers.textContent).toContain(WORD)
    expect(container.textContent).not.toMatch(/Trouvé par/)
    expect(container.textContent).not.toMatch(/\d+\s*pts?\b/) // aucun texte de points avant le crédit
    expect(container.querySelector('[data-testid="picto-found-by"]')).toBeNull()
  })
})

describe('Crédit visible par l\'animation commune TvPointsAward (§6.5 point 3, #265)', () => {
  it('Critical — TV : SCORE_CHANGE.GAINS {Bleue, +10} déclenche l\'animation de l\'équipe créditée, le mot reste seul', () => {
    const m = mount({ vplayer: false, own: null })
    m.push({ teams: teamsOf({ Bleue: 10, Rouge: 0 }), scoreChange: sc(2, true, [{ TEAM: 'Bleue', POINTS: 10 }]) })
    tick(0.7 * L)
    expect(awards(m.container)).toEqual(['+10 pts'])
    expect(m.container.querySelector('[data-testid="points-award-layer"]').textContent).toMatch(/Bleue/)
    // Le texte de points n'appartient qu'à l'animation commune : jamais à la zone réponse de Picto.
    expect(m.container.querySelector('.zone-answers').textContent).toBe(WORD)
    expect(m.container.textContent).not.toMatch(/Trouvé par/)
  })

  it('tablette de l\'équipe créditée : l\'animation de SON équipe', () => {
    const m = mount({ vplayer: true, own: 'Bleue' })
    m.push({ teams: teamsOf({ Bleue: 10, Rouge: 0 }), scoreChange: sc(2, true, [{ TEAM: 'Bleue', POINTS: 10 }]) })
    tick(0.7 * L)
    expect(awards(m.container)).toEqual(['+10 pts'])
  })

  it('Critical — tablette d\'une AUTRE équipe : aucune animation, à aucun instant', () => {
    const m = mount({ vplayer: true, own: 'Rouge' })
    m.push({ teams: teamsOf({ Bleue: 10, Rouge: 0 }), scoreChange: sc(2, true, [{ TEAM: 'Bleue', POINTS: 10 }]) })
    for (let i = 0; i < 10; i++) { tick(0.5 * L); expect(awards(m.container)).toEqual([]) }
  })

  it('Critical (revue M2) — crédit de 0 point : AUCUNE animation, et aucun texte de points sur la TV', () => {
    const m = mount({ vplayer: false, own: null })
    m.push({ teams: teamsOf(BASE), scoreChange: sc(2, true, [{ TEAM: 'Bleue', POINTS: 0 }]) })
    for (let i = 0; i < 10; i++) {
      tick(0.5 * L)
      expect(awards(m.container)).toEqual([])
      expect(m.container.textContent).not.toMatch(/\d+\s*pts?\b/)
    }
    expect(m.container.querySelector('.zone-answers').textContent).toBe(WORD) // le mot reste seul, rien d'autre
  })

  it('crédit de 0 point sur la tablette de l\'équipe : aucune animation non plus', () => {
    const m = mount({ vplayer: true, own: 'Bleue' })
    m.push({ teams: teamsOf(BASE), scoreChange: sc(2, true, [{ TEAM: 'Bleue', POINTS: 0 }]) })
    for (let i = 0; i < 10; i++) { tick(0.5 * L); expect(awards(m.container)).toEqual([]) }
  })

  it('correction cachée (DISPLAY = false) : aucune animation, comme pour tous les types', () => {
    const m = mount({ vplayer: false, own: null })
    m.push({ teams: teamsOf({ Bleue: 10, Rouge: 0 }), scoreChange: sc(2, false, [{ TEAM: 'Bleue', POINTS: 10 }]) })
    for (let i = 0; i < 10; i++) { tick(0.5 * L); expect(awards(m.container)).toEqual([]) }
  })

  it('pas de crédit pendant la manche (STARTED) : aucun texte de points sur la TV', () => {
    const useStarted = mount({ vplayer: false, own: null })
    useGame.mockReturnValue(game({ teams: teamsOf(BASE), scoreChange: sc(1, true, []), revealed: false }))
    act(() => { useStarted.rerender(<PlayerDisplay />) })
    expect(useStarted.container.textContent).not.toMatch(/\d+\s*pts?\b/)
    expect(useStarted.container.textContent).not.toContain(WORD)
  })
})
