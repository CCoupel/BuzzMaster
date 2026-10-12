import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { render, act } from '@testing-library/react'
import PlayerDisplay from './PlayerDisplay'
import { useGame } from '../hooks/GameContext'

// #265 — majeur de revue n°2 : le premier SEQ reçu par une connexion sert de RÉFÉRENCE, sans animer
// (contrat game-state.md §SCORE_CHANGE.GAINS, « Connexion ou reconnexion »). Au chargement ou au
// rechargement d'une TV / d'une tablette, l'état du contexte est d'abord vide (SEQ 0, aucune équipe),
// puis arrive le premier état réel (SEQ 12, scores déjà présents, GAINS non vide) : il ne doit RIEN animer.
// Même règle après une reconnexion WebSocket (statut déconnecté puis reconnecté, SEQ plus grand).
// Plus : l'annonce accessible de la tablette et l'absence d'animation sur toute la durée.
// Observables : textes « +X pts » (hors balisage) et région vivante accessible.

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
const name = (i) => `Equipe ${String(i).padStart(2, '0')}`
const NAMES = Array.from({ length: 24 }, (_, i) => name(i + 1))
const teamsOf = (scores) => Object.fromEntries(Object.entries(scores).map(([n, s]) => [n, { NAME: n, SCORE: s, COLOR: [30, 120, 250] }]))
const sc = (seq, display, gains) => ({ SEQ: seq, DISPLAY: display, TEAMS: gains.map((g) => g.TEAM), PLAYERS: [], GAINS: gains })
const gainsOf = (idx, pts = (i) => i) => idx.map((i) => ({ TEAM: name(i), POINTS: pts(i) }))

function game({ teams, scoreChange, status = 'connected', own }) {
  return {
    status,
    gameState: {
      phase: 'STOPPED', remote: 'GAME', timer: 0, totalTime: 30, question: null, ARDOISE_ANSWERS: {},
      MEMORY_PARTICIPATING_TEAMS: [], MEMOTION_PARTICIPATING_TEAMS: [], MEMOTION_CARD_STATES: {}, MEMOTION_CARD_TEAMS: {},
      MEMOTION_CURRENT_TEAM: null, MEMOTION_SELECTED: null, newGameBackgrounds: [], SCORE_CHANGE: scoreChange,
    },
    teams,
    bumpers: own ? { a: { TEAM: own, NAME: 'Paul', IS_VPLAYER: true, CONNECTED: true, SCORE: 0, COLOR: [30, 120, 250] } } : {},
    flipMemoryCard: vi.fn(), showQRCode: false, selectMotionCard: vi.fn(),
  }
}

function awards(container) {
  return [...container.querySelectorAll('*')]
    .filter((el) => el.children.length === 0 && /^\s*\+\d+ pts\s*$/.test(el.textContent))
    .map((el) => ({ pts: el.textContent.trim(), team: NAMES.find((n) => el.parentElement.textContent.includes(n)) || null }))
}
const teamsShown = (c) => awards(c).map((a) => a.team)
const tick = (ms) => act(() => { vi.advanceTimersByTime(ms) })

// Écran monté sur l'état INITIAL du contexte (aucune équipe, SEQ 0), comme au chargement de la page.
function mountFresh({ vplayer, own }) {
  let current = { teams: {}, scoreChange: sc(0, true, []), status: 'connected' }
  const props = vplayer
    ? { isVPlayer: true, playerName: 'Paul', playerNameColor: [30, 120, 250], teamName: own, teamColor: [30, 120, 250] }
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

const LOADED = { [name(1)]: 40, [name(2)]: 30, [name(3)]: 20 } // scores déjà présents à la connexion

describe.each([['TV', false], ['tablette', true]])('Premier SEQ = référence au chargement (%s) (#265)', (_label, vplayer) => {
  const own = name(1)

  it('Critical — état initial vide (SEQ 0) puis premier état réel (SEQ 12, GAINS non vide) : AUCUNE animation, sur toute la durée', () => {
    const m = mountFresh({ vplayer, own })
    m.push({ teams: teamsOf(LOADED), scoreChange: sc(12, true, gainsOf([1, 2, 3])) })
    for (let i = 0; i < 10; i++) {
      tick(0.2 * L)
      expect(awards(m.container)).toEqual([])
    }
    tick(10 * L)
    expect(awards(m.container)).toEqual([])
  })

  it('Critical — le SEQ suivant (13), lui, anime : la référence a bien été posée', () => {
    const m = mountFresh({ vplayer, own })
    m.push({ teams: teamsOf(LOADED), scoreChange: sc(12, true, gainsOf([1, 2, 3])) })
    tick(L)
    m.push({ teams: teamsOf({ ...LOADED, [name(1)]: 45 }), scoreChange: sc(13, true, [{ TEAM: name(1), POINTS: 5 }]) })
    tick(0.7 * L)
    expect(teamsShown(m.container)).toEqual([name(1)])
    expect(awards(m.container)[0].pts).toBe('+5 pts')
  })

  it('Critical — reconnexion WebSocket (déconnecté puis reconnecté avec un SEQ plus grand) : référence reposée, aucune animation', () => {
    const m = mountFresh({ vplayer, own })
    m.push({ teams: teamsOf(LOADED), scoreChange: sc(12, true, gainsOf([1, 2, 3])) })
    tick(L)
    m.push({ status: 'disconnected' })
    tick(L)
    // pendant la coupure, d'autres attributions ont eu lieu (SEQ 15) ; le 1er état de la nouvelle connexion
    m.push({ status: 'connected', teams: teamsOf({ ...LOADED, [name(1)]: 60, [name(2)]: 50 }), scoreChange: sc(15, true, gainsOf([1, 2], () => 10)) })
    for (let i = 0; i < 10; i++) {
      tick(0.5 * L)
      expect(awards(m.container)).toEqual([])
    }
    // l'attribution suivante (SEQ 16) anime de nouveau
    m.push({ teams: teamsOf({ ...LOADED, [name(1)]: 64, [name(2)]: 50 }), scoreChange: sc(16, true, [{ TEAM: name(1), POINTS: 4 }]) })
    tick(0.7 * L)
    expect(teamsShown(m.container)).toEqual([name(1)])
  })
})

describe('Absence d\'animation sur toute la durée de l\'annonce (#265, renforce les tests #252)', () => {
  const checkpoints = Array.from({ length: 14 }, () => 0.5 * L)

  it('TV — correction cachée (DISPLAY = false) sur 12 équipes : aucune carte à aucun instant, même pour un lot différé', () => {
    const m = mountFresh({ vplayer: false, own: null })
    const scores = Object.fromEntries(NAMES.slice(0, 12).map((n) => [n, 10]))
    m.push({ teams: teamsOf(scores), scoreChange: sc(1, true, []) }) // référence
    const after = Object.fromEntries(NAMES.slice(0, 12).map((n) => [n, 13]))
    m.push({ teams: teamsOf(after), scoreChange: sc(2, false, gainsOf([...Array(12).keys()].map((i) => i + 1), () => 3)) })
    for (const t of checkpoints) { tick(t); expect(awards(m.container)).toEqual([]) }
    expect(m.container.querySelector('[data-testid="points-award-layer"]')).toBeNull()
  })

  it('tablette — seules d\'autres équipes gagnent (12 équipes) : aucune carte à aucun instant', () => {
    const m = mountFresh({ vplayer: true, own: name(20) })
    const scores = Object.fromEntries(NAMES.slice(0, 20).map((n) => [n, 10]))
    m.push({ teams: teamsOf(scores), scoreChange: sc(1, true, []) })
    const after = { ...scores }
    for (let i = 1; i <= 12; i++) after[name(i)] = 14
    m.push({ teams: teamsOf(after), scoreChange: sc(2, true, gainsOf([...Array(12).keys()].map((i) => i + 1), () => 4)) })
    for (const t of checkpoints) { tick(t); expect(awards(m.container)).toEqual([]) }
  })
})

describe('Tablette — annonce accessible du gain (#265)', () => {
  it('Critical — une région vivante (role="status" ou aria-live) annonce « pour votre équipe +X pts », hors de tout aria-hidden', () => {
    const own = name(8)
    const m = mountFresh({ vplayer: true, own })
    const scores = Object.fromEntries(NAMES.slice(0, 12).map((n) => [n, 10]))
    m.push({ teams: teamsOf(scores), scoreChange: sc(1, true, []) })
    const after = { ...scores, [own]: 18 }
    m.push({ teams: teamsOf(after), scoreChange: sc(2, true, [{ TEAM: own, POINTS: 8 }]) })
    tick(0.7 * L)
    expect(teamsShown(m.container)).toEqual([own]) // la carte est bien visible
    const live = [...m.container.querySelectorAll('[role="status"], [aria-live]')]
      .filter((el) => /pour votre équipe/i.test(el.textContent) && /\+8 pts/.test(el.textContent))
    expect(live.length).toBeGreaterThan(0)
    expect(live.every((el) => el.closest('[aria-hidden="true"]') === null)).toBe(true)
  })
})
