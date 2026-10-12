import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { render, act } from '@testing-library/react'
import confetti from 'canvas-confetti'
import PlayerDisplay from './PlayerDisplay'
import { useGame } from '../hooks/GameContext'

// #265 — attribution de points à plusieurs équipes : feu d'artifice par lots de 6 au plus sur la TV,
// gain de SA propre équipe seulement sur la tablette, calé sur le lot de la TV.
// Maquettes validées : docs/mockup/v11.2.1/ui/tv_points_award__multi_team.html et
// vplayer_points_award__own_team.html ; contrat game-state.md §SCORE_CHANGE.GAINS ; DECISIONS.md.
// Observables seulement (aucun nom de classe interne) : les éléments « +X pts » et le texte de leur carte,
// à des instants simulés. Fenêtre de lecture d'un lot : cartes visibles à 0,7 × durée du lot (zoom atteint
// avant 38 %, fondu après 86 %). Lot normal 3 200 ms.
// Les rendus visuels réels (fluidité, nom tronqué, zone sûre) restent à la procédure manuelle.

vi.mock('nosleep.js', () => ({ default: class { enable() { return Promise.resolve() } disable() {} } }))
vi.mock('canvas-confetti', () => ({ default: vi.fn() }))
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

const L = 3200
const name = (i) => `Equipe ${String(i).padStart(2, '0')}`
const NAMES = Array.from({ length: 24 }, (_, i) => name(i + 1))

// scores courants par équipe (cumulés au fil des UPDATE)
function makeTeams(scores) {
  const out = {}
  for (const n of NAMES) out[n] = { NAME: n, SCORE: scores[n] || 0, COLOR: [30, 120, 250] }
  return out
}
const bumpersFor = (own) => (own ? { a: { TEAM: own, NAME: 'Paul', IS_VPLAYER: true, CONNECTED: true, SCORE: 0, COLOR: [30, 120, 250] } } : {})
const sc = (seq, display, gains) => ({ SEQ: seq, DISPLAY: display, TEAMS: gains.map((g) => g.TEAM), PLAYERS: [], GAINS: gains })
const gainsOf = (idx, pts = (i) => i) => idx.map((i) => ({ TEAM: name(i), POINTS: pts(i) }))

function game({ scores, scoreChange, remote = 'GAME', own }) {
  return {
    gameState: {
      phase: 'STOPPED', remote, timer: 0, totalTime: 30, question: null, ARDOISE_ANSWERS: {},
      MEMORY_PARTICIPATING_TEAMS: [], MEMOTION_PARTICIPATING_TEAMS: [], MEMOTION_CARD_STATES: {}, MEMOTION_CARD_TEAMS: {},
      MEMOTION_CURRENT_TEAM: null, MEMOTION_SELECTED: null, newGameBackgrounds: [], SCORE_CHANGE: scoreChange,
    },
    teams: makeTeams(scores), bumpers: bumpersFor(own), flipMemoryCard: vi.fn(), showQRCode: false, selectMotionCard: vi.fn(),
  }
}

// Éléments « +X pts » et texte de leur carte (parent), quel que soit le balisage.
function awards(container) {
  return [...container.querySelectorAll('*')]
    .filter((el) => el.children.length === 0 && /^\s*\+\d+ pts\s*$/.test(el.textContent))
    .map((el) => {
      const card = el.parentElement
      const team = NAMES.find((n) => card.textContent.includes(n)) || null
      return { pts: el.textContent.trim(), team, text: card.textContent.replace(/\s+/g, ' ') }
    })
}
const teamsShown = (c) => awards(c).map((a) => a.team)

// Monte l'écran (TV ou tablette) ; `send` rejoue un UPDATE avec un nouveau SEQ et les scores cumulés.
function mount({ own = null, initialSeq = 0, initialGains = [], vplayer = false } = {}) {
  const scores = {}
  let remote = 'GAME'
  const props = vplayer
    ? { isVPlayer: true, playerName: 'Paul', playerNameColor: [30, 120, 250], teamName: own, teamColor: [30, 120, 250] }
    : {}
  useGame.mockReturnValue(game({ scores, scoreChange: sc(initialSeq, true, initialGains), own }))
  const utils = render(<PlayerDisplay {...props} />)
  const rerender = (scoreChange) => {
    useGame.mockReturnValue(game({ scores, scoreChange, remote, own }))
    act(() => { utils.rerender(<PlayerDisplay {...props} />) })
  }
  return {
    ...utils,
    // nouvelle annonce : les scores montent des gains, SEQ + GAINS
    send: (seq, gains, display = true) => {
      for (const g of gains) scores[g.TEAM] = (scores[g.TEAM] || 0) + g.POINTS
      rerender(sc(seq, display, gains))
    },
    // même SEQ rediffusé (UPDATE suivant, ex. tick du chrono)
    resend: (seq, gains, display = true) => rerender(sc(seq, display, gains)),
    // crédit automatique : les scores montent SANS nouveau SEQ
    credit: (seq, gains) => {
      for (const g of gains) scores[g.TEAM] = (scores[g.TEAM] || 0) + g.POINTS
      rerender(sc(seq, true, []))
    },
    view: (r, seq, gains) => { remote = r; rerender(sc(seq, true, gains)) },
  }
}
const tick = (ms) => act(() => { vi.advanceTimersByTime(ms) })

beforeEach(() => {
  vi.clearAllMocks()
  vi.useFakeTimers()
  document.documentElement.requestFullscreen = vi.fn().mockResolvedValue(undefined)
  window.matchMedia = window.matchMedia || ((q) => ({ matches: false, media: q, addEventListener() {}, removeEventListener() {}, addListener() {}, removeListener() {} }))
})
afterEach(() => { vi.useRealTimers() })

function stubReducedMotion() {
  window.matchMedia = (q) => ({ matches: /prefers-reduced-motion/.test(q), media: q, addEventListener() {}, removeEventListener() {}, addListener() {}, removeListener() {}, onchange: null, dispatchEvent: () => false })
}

describe('TV — une animation par équipe, par lots de 6 au plus (#265)', () => {
  it('Critical — 4 équipes : 4 cartes « +X pts » (une par équipe, bon gain) dans la fenêtre du lot, puis plus rien', () => {
    const m = mount()
    m.send(1, gainsOf([1, 2, 3, 4]))
    tick(0.7 * L)
    const a = awards(m.container)
    expect(a.map((x) => x.team).sort()).toEqual([name(1), name(2), name(3), name(4)])
    expect(a.find((x) => x.team === name(3)).pts).toBe('+3 pts')
    tick(L)
    expect(awards(m.container)).toEqual([])
  })

  it('Critical — 12 équipes : 6 cartes au 1er lot (équipes 1 à 6), puis 6 au 2ᵉ (7 à 12) sans superposition', () => {
    const m = mount()
    m.send(1, gainsOf([...Array(12).keys()].map((i) => i + 1)))
    tick(0.7 * L)
    expect(teamsShown(m.container).sort()).toEqual([1, 2, 3, 4, 5, 6].map(name))
    tick(L) // 1,7 × L
    expect(teamsShown(m.container).sort()).toEqual([7, 8, 9, 10, 11, 12].map(name))
    tick(L) // 2,7 × L > fin
    expect(awards(m.container)).toEqual([])
  })

  it('plus de salve unique de confettis par équipe lancée en même temps (6 équipes → au plus une salve)', () => {
    const m = mount()
    m.send(1, gainsOf([1, 2, 3, 4, 5, 6]))
    expect(confetti.mock.calls.length).toBeLessThanOrEqual(1)
  })

  it('pénalité ou compensation (POINTS ≤ 0) : aucune animation', () => {
    const m = mount()
    m.send(1, [{ TEAM: name(1), POINTS: -3 }, { TEAM: name(2), POINTS: 0 }])
    tick(0.7 * L)
    expect(awards(m.container)).toEqual([])
  })

  it('une équipe à gain négatif parmi d\'autres n\'est pas animée, les autres le sont', () => {
    const m = mount()
    m.send(1, [{ TEAM: name(1), POINTS: 5 }, { TEAM: name(2), POINTS: -3 }])
    tick(0.7 * L)
    expect(teamsShown(m.container)).toEqual([name(1)])
  })
})

describe('TV — file d\'attente, correction cachée, reconnexion (#265)', () => {
  it('Critical — un 2ᵉ SEQ pendant l\'animation est mis en file à la suite (rien de coupé ni de superposé)', () => {
    const m = mount()
    m.send(1, gainsOf([1, 2]))
    tick(1000)
    m.send(2, gainsOf([3]))
    tick(1240) // t = 0,7 × L : lot du SEQ 1 seulement
    expect(teamsShown(m.container).sort()).toEqual([name(1), name(2)])
    tick(L) // t = 5 440 ms : la 2ᵉ annonce démarre à 3 200 ms (fin du lot 1), visible 0,7 × L plus tard
    expect(teamsShown(m.container)).toEqual([name(3)])
  })

  it('Critical — correction cachée (DISPLAY = false) : aucune animation et n\'entre pas dans la file', () => {
    const m = mount()
    m.send(1, gainsOf([1, 2]), false)
    tick(0.7 * L)
    expect(awards(m.container)).toEqual([])
    m.send(2, gainsOf([3])) // visible juste après : part tout de suite, sans attendre la correction cachée
    tick(0.7 * L)
    expect(teamsShown(m.container)).toEqual([name(3)])
  })

  it('le même SEQ rediffusé (UPDATE suivant) ne rejoue rien', () => {
    const m = mount()
    m.send(1, gainsOf([1, 2]))
    tick(2 * L)
    expect(awards(m.container)).toEqual([])
    m.resend(1, gainsOf([1, 2]))
    tick(0.7 * L)
    expect(awards(m.container)).toEqual([])
  })

  it('Critical — reconnexion : le premier SEQ reçu (même non nul) sert de référence, sans animer', () => {
    const m = mount({ initialSeq: 9, initialGains: gainsOf([1, 2, 3]) })
    tick(0.7 * L)
    expect(awards(m.container)).toEqual([])
    m.resend(9, gainsOf([1, 2, 3]))
    tick(0.7 * L)
    expect(awards(m.container)).toEqual([])
    m.send(10, gainsOf([4])) // le suivant, lui, anime
    tick(0.7 * L)
    expect(teamsShown(m.container)).toEqual([name(4)])
  })

  it('crédit automatique (scores qui montent sans nouveau SEQ) : repli par comparaison des scores, équipes triées par nom', () => {
    const m = mount({ initialSeq: 0 })
    m.credit(0, [{ TEAM: name(7), POINTS: 2 }, { TEAM: name(2), POINTS: 5 }])
    tick(0.7 * L)
    expect(teamsShown(m.container)).toEqual([name(2), name(7)]) // ordre alphabétique, identique sur la TV et la tablette
    expect(awards(m.container).map((x) => x.pts)).toEqual(['+5 pts', '+2 pts'])
  })

  it('changement de vue (SCORE) : la file est vidée, plus aucune carte', () => {
    const m = mount()
    m.send(1, gainsOf([1, 2, 3]))
    tick(500)
    m.view('SCORE', 1, gainsOf([1, 2, 3]))
    tick(0.7 * L)
    expect(awards(m.container)).toEqual([])
  })

  it('prefers-reduced-motion : cartes en fondu à leur place dès le début du lot (pas de vol), même découpage', () => {
    stubReducedMotion()
    const m = mount()
    m.send(1, gainsOf([1, 2, 3, 4]))
    tick(0.2 * L) // sans réduction, la fusée est encore en vol à cet instant
    expect(teamsShown(m.container).sort()).toEqual([1, 2, 3, 4].map(name))
    tick(L)
    expect(awards(m.container)).toEqual([])
  })
})

describe('Tablette — seulement le gain de sa propre équipe, calé sur le lot de la TV (#265)', () => {
  it('Critical — équipe du 2ᵉ lot (12 équipes) : rien pendant le lot 1, puis UNE carte « pour votre équipe » avec son gain', () => {
    const own = name(8)
    const m = mount({ own, vplayer: true })
    m.send(1, gainsOf([...Array(12).keys()].map((i) => i + 1)))
    tick(0.7 * L)
    expect(awards(m.container)).toEqual([]) // pendant l'attente, rien d'affiché
    tick(L) // 1,7 × L : lot 2 de la TV
    const a = awards(m.container)
    expect(a).toHaveLength(1)
    expect(a[0].team).toBe(own)
    expect(a[0].pts).toBe('+8 pts')
    expect(a[0].text).toMatch(/pour votre équipe/)
    // jamais le nom d'une autre équipe
    expect(NAMES.filter((n) => n !== own && m.container.textContent.includes(n) && awards(m.container).some((x) => x.text.includes(n)))).toEqual([])
    tick(L)
    expect(awards(m.container)).toEqual([])
  })

  it('équipe du 1er lot : visible dans la fenêtre du lot 1, une seule carte', () => {
    const m = mount({ own: name(2), vplayer: true })
    m.send(1, gainsOf([...Array(12).keys()].map((i) => i + 1)))
    tick(0.7 * L)
    expect(teamsShown(m.container)).toEqual([name(2)])
  })

  it('Critical — plus de célébration involontaire : seules d\'autres équipes gagnent → rien, aucun confetti', () => {
    const m = mount({ own: name(1), vplayer: true })
    m.send(1, gainsOf([2, 3, 4]))
    tick(0.7 * L)
    expect(awards(m.container)).toEqual([])
    expect(confetti).not.toHaveBeenCalled()
  })

  it('son équipe gagne : aucun confetti sur la tablette (l\'animation de la maquette remplace la célébration)', () => {
    const m = mount({ own: name(1), vplayer: true })
    m.send(1, gainsOf([1]))
    tick(0.7 * L)
    expect(teamsShown(m.container)).toEqual([name(1)])
    expect(confetti).not.toHaveBeenCalled()
  })

  it('correction cachée sur son équipe : rien', () => {
    const m = mount({ own: name(1), vplayer: true })
    m.send(1, gainsOf([1]), false)
    tick(0.7 * L)
    expect(awards(m.container)).toEqual([])
  })

  it('son équipe perd des points (pénalité) : rien', () => {
    const m = mount({ own: name(1), vplayer: true })
    m.send(1, [{ TEAM: name(1), POINTS: -2 }, { TEAM: name(2), POINTS: 3 }])
    tick(0.7 * L)
    expect(awards(m.container)).toEqual([])
  })

  it('joueur sans équipe : rien', () => {
    const m = mount({ own: null, vplayer: true })
    m.send(1, gainsOf([1, 2]))
    tick(0.7 * L)
    expect(awards(m.container)).toEqual([])
  })

  it('Critical — la file avance même sans sa propre équipe : le 2ᵉ SEQ (son équipe) démarre après les 2 lots du 1er', () => {
    const own = name(20)
    const m = mount({ own, vplayer: true })
    m.send(1, gainsOf([...Array(12).keys()].map((i) => i + 1))) // 2 lots à la TV = 6,4 s, sans son équipe
    tick(1000)
    m.send(2, gainsOf([20]))
    tick(2 * L - 1000 - 100) // juste avant la fin des 2 lots de la TV
    expect(awards(m.container)).toEqual([])
    tick(100 + 0.7 * L) // 2 × L + 0,7 × L : lot du 2ᵉ SEQ
    expect(teamsShown(m.container)).toEqual([own])
  })
})
