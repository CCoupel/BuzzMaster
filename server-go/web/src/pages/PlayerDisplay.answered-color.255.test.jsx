import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import PlayerDisplay from './PlayerDisplay'

// #255 (v11.2) — TV : en STOPPED / REVEALED, les badges d'équipes sur chaque réponse QCM ne concernent que les équipes qui
// ont RÉPONDU (ANSWERED_COLOR), jamais celles dont seule la couleur est assignée (ANSWER_COLOR). Repli sans la clé :
// TIME > 0 && ANSWER_COLOR. Scénario 12 du plan.

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

const TEAMS = { 'Les Bleus': { NAME: 'Les Bleus', SCORE: 0, COLOR: [0, 0, 255] }, 'Les Rouges': { NAME: 'Les Rouges', SCORE: 0, COLOR: [255, 0, 0] }, 'Les Verts': { NAME: 'Les Verts', SCORE: 0, COLOR: [0, 255, 0] } }
const QUESTION = { ID: '9', TYPE: 'QCM', QUESTION: 'Fleuve ?', ANSWER: 'La Seine', POINTS: '3', QCM_ANSWERS: { RED: 'La Loire', GREEN: 'La Seine', YELLOW: 'Le Rhône', BLUE: 'La Garonne' }, QCM_CORRECT: 'GREEN', CATEGORY: 'GEOGRAPHY' }
const bump = (team, name, assigned, answered, time) => ({ TEAM: team, NAME: name, IS_VPLAYER: true, CONNECTED: true, SCORE: 0, ANSWER_COLOR: assigned, TIME: time, HINTS_AT_BUZZ: 0, ...(answered === undefined ? {} : { ANSWERED_COLOR: answered }) })
const mount = (bumpers, phase = 'REVEALED') => {
  useGame.mockReturnValue({
    gameState: {
      phase, remote: 'GAME', timer: 0, totalTime: 30, question: QUESTION, qcmInvalidated: [], ARDOISE_ANSWERS: {},
      MEMORY_PARTICIPATING_TEAMS: [], MEMOTION_PARTICIPATING_TEAMS: [], MEMOTION_CARD_STATES: {}, MEMOTION_CARD_TEAMS: {},
      MEMOTION_CURRENT_TEAM: null, MEMOTION_SELECTED: null, newGameBackgrounds: [], SCORE_CHANGE: { SEQ: 0, DISPLAY: true, TEAMS: [], PLAYERS: [] },
    },
    teams: TEAMS, bumpers, flipMemoryCard: vi.fn(), showQRCode: false, selectMotionCard: vi.fn(),
  })
  return render(<PlayerDisplay />)
}
const badgesOf = (container, answerText) => {
  const item = [...container.querySelectorAll('.qcm-answer-item')].find((n) => n.textContent.includes(answerText))
  return item ? [...item.querySelectorAll('.qcm-team-badge-wrapper')].map((n) => n.getAttribute('title')) : null
}

describe('TV — badges des réponses QCM selon ANSWERED_COLOR (#255, scénario 12)', () => {
  beforeEach(() => { useGame.mockReset() })

  it.each(['STOPPED', 'REVEALED'])('Critical — équipes sans réponse (couleur assignée, ANSWERED_COLOR "") : AUCUN badge, phase %s', (phase) => {
    const { container } = mount({
      a: bump('Les Bleus', 'Paul', 'RED', '', 1234), b: bump('Les Rouges', 'Mamie', 'GREEN', '', 0), c: bump('Les Verts', 'Vera', 'GREEN', '', 0),
    }, phase)
    for (const t of ['La Loire', 'La Seine', 'Le Rhône', 'La Garonne']) expect(badgesOf(container, t), t).toEqual([])
  })

  it('seules les équipes qui ont répondu portent un badge, sur la couleur RÉPONDUE (pas la couleur assignée)', () => {
    const { container } = mount({
      a: bump('Les Bleus', 'Paul', 'RED', 'GREEN', 1000), b: bump('Les Rouges', 'Mamie', 'GREEN', '', 0), c: bump('Les Verts', 'Vera', 'GREEN', 'BLUE', 2000),
    })
    expect(badgesOf(container, 'La Seine')).toEqual(['Les Bleus'])
    expect(badgesOf(container, 'La Garonne')).toEqual(['Les Verts'])
    expect(badgesOf(container, 'La Loire')).toEqual([])
  })

  it('repli sans la clé : TIME > 0 && ANSWER_COLOR', () => {
    const { container } = mount({ a: bump('Les Bleus', 'Paul', 'GREEN', undefined, 1000), b: bump('Les Rouges', 'Mamie', 'GREEN', undefined, 0) })
    expect(badgesOf(container, 'La Seine')).toEqual(['Les Bleus'])
  })

  it('pas de badge avant STOPPED (STARTED)', () => {
    const { container } = mount({ a: bump('Les Bleus', 'Paul', 'GREEN', 'GREEN', 1000) }, 'STARTED')
    for (const t of ['La Loire', 'La Seine']) expect(badgesOf(container, t) ?? []).toEqual([])
  })
})
