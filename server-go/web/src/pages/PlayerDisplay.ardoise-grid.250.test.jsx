import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render } from '@testing-library/react'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import PlayerDisplay from './PlayerDisplay'

// #250 (v11.2) — TV, REVEAL ARDOISE : toutes les équipes, grille adaptative, aucun défilement.
// Maquette VALIDÉE : docs/mockup/v11.2/ui/tv_ardoise_reveal__adaptive_grid.html (4/8/10/12/16 équipes :
// une rangée jusqu'à 5, deux rangées de 6 à 16 — texte sur 2 lignes jusqu'à 12 équipes, 1 ligne de 13 à 16,
// trois rangées au-delà —, police proportionnelle à la largeur d'une carte, réponse longue coupée « … »).
// Contrainte TV STATIQUE (CLAUDE.md) : overflow hidden (jamais auto/scroll), unités viewport, min-height: 0.
// Les attentes ci-dessous viennent de la maquette, PAS du code (la règle de mise en page est recalculée ici).

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

// Règle de la maquette (fonction layout) — indépendante de utils/ardoiseTvLayout.js
const expectedLayout = (n) => {
  if (n <= 5) return { rows: 1, cols: n, lines: 2 }
  if (n <= 12) return { rows: 2, cols: Math.ceil(n / 2), lines: 2 }
  if (n <= 16) return { rows: 2, cols: Math.ceil(n / 2), lines: 1 }
  return { rows: 3, cols: Math.ceil(n / 3), lines: 1 }
}

const QUESTION = { ID: 'q-ard', TYPE: 'ARDOISE', QUESTION: 'Capitale ?', ANSWER: 'Paris', ARDOISE_KEYBOARD_TYPE: 'AZERTY' }
const teamsN = (n) => Object.fromEntries(Array.from({ length: n }, (_, i) => [`Équipe ${i + 1}`, { NAME: `Équipe ${i + 1}`, SCORE: i, COLOR: [20 + i * 10, 100, 200] }]))
const bumpersN = (n) => Object.fromEntries(Array.from({ length: n }, (_, i) => [`AA:${i}`, { TEAM: `Équipe ${i + 1}`, NAME: `VJ-${i + 1}`, IS_VPLAYER: true, CONNECTED: true }]))
const answersN = (n, text = 'PARIS') => Object.fromEntries(Array.from({ length: n }, (_, i) => [`Équipe ${i + 1}`, { TEXT: `${text} ${i + 1}`, STARTED_AT: 100 + i, SUBMITTED_AT: 200 + i }]))

const mock = ({ phase = 'REVEALED', teams, bumpers, answers = {}, question = QUESTION } = {}) => ({
  gameState: {
    phase, remote: 'GAME', timer: 0, totalTime: 30, question, ARDOISE_ANSWERS: answers,
    MEMORY_PARTICIPATING_TEAMS: [], MEMOTION_PARTICIPATING_TEAMS: [], MEMOTION_CARD_STATES: {}, MEMOTION_CARD_TEAMS: {},
    MEMOTION_CURRENT_TEAM: null, MEMOTION_SELECTED: null, newGameBackgrounds: [],
  },
  teams, bumpers, flipMemoryCard: vi.fn(), showQRCode: false, selectMotionCard: vi.fn(),
})
const renderTV = (over) => { useGame.mockReturnValue(mock(over)); return render(<PlayerDisplay />) }
const cards = (c) => [...c.querySelectorAll('.ardoise-team-card')]
const grid = (c) => c.querySelector('.ardoise-teams-grid')
const v = (el, name) => el.style.getPropertyValue(name).trim()

describe('PlayerDisplay — REVEAL ARDOISE : toutes les équipes, grille adaptative (#250)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    document.documentElement.requestFullscreen = vi.fn().mockResolvedValue(undefined)
  })

  it.each([4, 8, 10, 12, 16])('Critical — %i équipes : TOUTES les cartes sont rendues (plus de plafond à 8), avec leur nom', (n) => {
    const { container } = renderTV({ teams: teamsN(n), bumpers: bumpersN(n), answers: answersN(n) })
    expect(cards(container).length).toBe(n)
    for (let i = 1; i <= n; i++) {
      expect(cards(container).some((c) => c.querySelector('.ardoise-team-card-header').textContent === `Équipe ${i}`)).toBe(true)
    }
  })

  it('Critical — 17 et 20 équipes : encore toutes rendues, sur trois rangées', () => {
    for (const n of [17, 20]) {
      const { container, unmount } = renderTV({ teams: teamsN(n), bumpers: bumpersN(n), answers: answersN(n) })
      expect(cards(container).length).toBe(n)
      expect(Number(v(grid(container), '--rows'))).toBe(3)
      unmount()
    }
  })

  it.each([1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18])('%i équipes : colonnes, rangées et lignes de texte selon la règle de la maquette', (n) => {
    const { container } = renderTV({ teams: teamsN(n), bumpers: bumpersN(n), answers: answersN(n) })
    const exp = expectedLayout(n)
    const g = grid(container)
    expect({ rows: Number(v(g, '--rows')), cols: Number(v(g, '--cols')), lines: Number(v(g, '--lines')) }).toEqual(exp)
  })

  it('une seule rangée jusqu’à 5 équipes, deux rangées de 6 à 16', () => {
    for (const [n, rows] of [[3, 1], [5, 1], [6, 2], [10, 2], [16, 2]]) {
      const { container, unmount } = renderTV({ teams: teamsN(n), bumpers: bumpersN(n) })
      expect(Number(v(grid(container), '--rows')), `${n} équipes`).toBe(rows)
      unmount()
    }
  })

  it('le texte rétrécit quand le nombre d’équipes augmente (polices proportionnelles à la largeur d’une carte), sans dépasser les plafonds', () => {
    const fa = (n) => { const r = renderTV({ teams: teamsN(n), bumpers: bumpersN(n) }); const x = parseFloat(v(grid(r.container), '--fa')); r.unmount(); return x }
    const fh = (n) => { const r = renderTV({ teams: teamsN(n), bumpers: bumpersN(n) }); const x = parseFloat(v(grid(r.container), '--fh')); r.unmount(); return x }
    expect(fa(4)).toBeGreaterThanOrEqual(fa(8)) // plafonnées à 0,026 pour peu d'équipes
    expect(fa(8)).toBeGreaterThan(fa(16))
    expect(fa(16)).toBeLessThan(0.026)
    expect(fh(4)).toBeGreaterThanOrEqual(fh(16))
    for (const n of [1, 4, 8, 16]) {
      expect(fa(n)).toBeLessThanOrEqual(0.026 + 1e-9)
      expect(fh(n)).toBeLessThanOrEqual(0.016 + 1e-9)
      expect(fa(n)).toBeGreaterThan(0)
    }
  })

  it('seules les équipes ayant au moins un VJoueur sont affichées (inchangé) — 12 équipes + 1 sans tablette = 12 cartes', () => {
    const teams = { ...teamsN(12), 'Sans tablette': { NAME: 'Sans tablette', SCORE: 0, COLOR: [1, 2, 3] } }
    const { container } = renderTV({ teams, bumpers: bumpersN(12), answers: answersN(12) })
    expect(cards(container).length).toBe(12)
    expect(container.textContent).not.toMatch(/Sans tablette/)
  })

  it('Critical — équipe résolue par la CLÉ : NAME périmé ignoré, la réponse enregistrée sous la clé s’affiche', () => {
    const teams = { 'Les Bleus': { NAME: 'Equipe 1', SCORE: 0, COLOR: [0, 0, 255] }, 'Les Rouges': { NAME: 'Les Rouges', SCORE: 0, COLOR: [255, 0, 0] } }
    const bumpers = { a: { TEAM: 'Les Bleus', IS_VPLAYER: true, CONNECTED: true, NAME: 'A' }, b: { TEAM: 'Les Rouges', IS_VPLAYER: true, CONNECTED: true, NAME: 'B' } }
    const { container } = renderTV({ teams, bumpers, answers: { 'Les Bleus': { TEXT: 'NAPOLEON', STARTED_AT: 1, SUBMITTED_AT: 2 } } })

    const bleus = cards(container).find((c) => c.querySelector('.ardoise-team-card-header').textContent === 'Les Bleus')
    expect(bleus, 'en-tête = clé de l’équipe').toBeTruthy()
    expect(bleus.querySelector('.ardoise-team-card-answer').textContent).toBe('NAPOLEON')
    expect(container.textContent).not.toMatch(/Equipe 1/)
  })

  it('réponse longue : le texte COMPLET reste dans le DOM (la coupe « … » est visuelle, par CSS) ; équipe sans réponse : « — »', () => {
    const long = 'LA CAPITALE C’EST PARIS BIEN SÛR, PAS LYON NI MARSEILLE'
    const { container } = renderTV({
      teams: teamsN(3), bumpers: bumpersN(3),
      answers: { 'Équipe 1': { TEXT: long, STARTED_AT: 1, SUBMITTED_AT: 2 } },
    })
    const [c1, c2] = cards(container)
    expect(c1.querySelector('.ardoise-team-card-answer').textContent).toBe(long)
    expect(c1.classList.contains('has-answer')).toBe(true)
    expect(c2.querySelector('.ardoise-team-card-answer').textContent).toBe('—')
    expect(c2.classList.contains('no-answer')).toBe(true)
    // une TV n'a pas de survol : aucune dépendance à une infobulle pour lire la réponse
    expect(container.querySelector('.ardoise-teams-grid').querySelectorAll('[class*="scroll"]').length).toBe(0)
  })

  it('non-régression (moins de 8 équipes) : 2 équipes, mêmes cartes, entête + réponse, zone de bonne réponse conservée', () => {
    const { container } = renderTV({ teams: teamsN(2), bumpers: bumpersN(2), answers: answersN(2) })
    expect(cards(container).length).toBe(2)
    expect(container.querySelector('.ardoise-correct-answer').textContent).toMatch(/Paris/)
    expect(cards(container)[0].querySelector('.ardoise-team-card-answer').textContent).toBe('PARIS 1')
  })

  it('non-régression : pas de grille ARDOISE pendant la saisie (STARTED) ni pour une autre sorte de question', () => {
    const started = renderTV({ phase: 'STARTED', teams: teamsN(10), bumpers: bumpersN(10) })
    expect(grid(started.container)).toBeNull()
    started.unmount()
    const speedy = renderTV({ teams: teamsN(10), bumpers: bumpersN(10), question: { ID: 'q-sp', TYPE: 'SPEEDY', QUESTION: 'Q', ANSWER: 'A' } })
    expect(grid(speedy.container)).toBeNull()
  })
})

// ---------------------------------------------------------------------------
// Contrainte TV statique : lecture du texte source de PlayerDisplay.css (jsdom n'applique pas la feuille)
// ---------------------------------------------------------------------------
describe('PlayerDisplay.css — grille ARDOISE TV : statique, unités viewport, texte coupé (#250)', () => {
  const css = fs.readFileSync(path.join(path.dirname(fileURLToPath(import.meta.url)), 'PlayerDisplay.css'), 'utf-8').replace(/\/\*[\s\S]*?\*\//g, '')
  const start = css.indexOf('.ardoise-teams-grid')
  const end = css.indexOf('.ardoise-team-card.no-answer')
  const block = css.slice(start, end)
  const rule = (selector) => {
    const m = block.match(new RegExp(selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&') + '\\s*\\{([^}]*)\\}'))
    return m ? m[1] : null
  }

  it('les règles de la grille existent', () => {
    expect(start).toBeGreaterThan(-1)
    for (const s of ['.ardoise-teams-grid', '.ardoise-team-card', '.ardoise-team-card-header', '.ardoise-team-card-answer']) expect(rule(s), s).not.toBeNull()
  })

  it('Critical — aucun défilement : jamais overflow auto/scroll, ni overflow-x/y auto/scroll, dans la grille ni les cartes', () => {
    expect(block).not.toMatch(/overflow(-x|-y)?\s*:\s*(auto|scroll)/)
    expect(rule('.ardoise-teams-grid')).toMatch(/overflow:\s*hidden/)
    expect(rule('.ardoise-team-card')).toMatch(/overflow:\s*hidden/)
  })

  it('flex/grille avec min-height: 0 (permet le rétrécissement) et colonnes minmax(0, 1fr)', () => {
    expect(rule('.ardoise-teams-grid')).toMatch(/min-height:\s*0/)
    expect(rule('.ardoise-team-card')).toMatch(/min-height:\s*0/)
    expect(rule('.ardoise-teams-grid')).toMatch(/grid-template-columns:[^;]*minmax\(0,\s*1fr\)/)
    expect(rule('.ardoise-teams-grid')).toMatch(/grid-template-rows:[^;]*minmax\(0,\s*1fr\)/)
  })

  it('tailles en unités viewport / relatives (aucun px pour les polices ni les espacements de la grille)', () => {
    const fonts = block.match(/font-size:[^;]*;/g) || []
    expect(fonts.length).toBeGreaterThan(0)
    for (const f of fonts) expect(f).not.toMatch(/\d\s*px/)
    expect(rule('.ardoise-teams-grid')).toMatch(/gap:\s*[\d.]+vw/)
    expect(block).toMatch(/calc\(var\(--f[ah][^)]*\)\s*\*\s*100vw\)/)
  })

  it('réponse longue coupée par « … » : -webkit-line-clamp piloté par --lines, overflow hidden, retour à la ligne forcé, jamais de scroll', () => {
    const a = rule('.ardoise-team-card-answer')
    expect(a).toMatch(/-webkit-line-clamp:\s*var\(--lines/)
    expect(a).toMatch(/display:\s*-webkit-box/)
    expect(a).toMatch(/overflow:\s*hidden/)
    expect(a).toMatch(/overflow-wrap:\s*anywhere/)
  })

  it('le nom d’équipe trop long est coupé par « … » sur une ligne', () => {
    const h = rule('.ardoise-team-card-header')
    expect(h).toMatch(/white-space:\s*nowrap/)
    expect(h).toMatch(/text-overflow:\s*ellipsis/)
    expect(h).toMatch(/overflow:\s*hidden/)
  })
})
