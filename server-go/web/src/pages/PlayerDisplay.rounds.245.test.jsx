import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, resolve } from 'node:path'
import PlayerDisplay from './PlayerDisplay'

// #245 (v11.2, lot V3-3) — TV selon la maquette VALIDÉE (g‴) : pastille « Manche 2 · nom » si ROUND_INDEX > 0 ; titre « Manche 2 —
// nom » sur l'écran d'attente (READY) de la PREMIÈRE question de la manche (FIRST_OF_ROUND) ; rien sans manche ; TV STATIQUE :
// unités d'écran, rien ne défile ni ne déborde (règle du projet), vérifié sur les règles CSS (jsdom ne calcule pas de mise en page).
// Scénarios 20 et 38. `critical` : rien sans manche, pas de débordement.

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

const here = dirname(fileURLToPath(import.meta.url))
const css = readFileSync(resolve(here, './PlayerDisplay.css'), 'utf8')
const ROUNDS = [{ ID: 'r1', NAME: 'Un' }, { ID: 'r2', NAME: 'Deux' }]
const QUESTION = { ID: '9', TYPE: 'SPEEDY', QUESTION: 'Vite ?', ANSWER: 'oui', POINTS: '1', CATEGORY: 'GEOGRAPHY' }
const ctx = (over = {}) => ({ ROUND_ID: 'r2', ROUND_INDEX: 2, ROUND_COUNT: 2, POSITION: 1, TOTAL: 3, FIRST_OF_ROUND: false, ...over })
const mount = (phase, roundContext, over = {}) => {
  useGame.mockReturnValue({
    gameState: {
      phase, remote: 'GAME', timer: 0, totalTime: 30, question: over.question || QUESTION, qcmInvalidated: [], ARDOISE_ANSWERS: {},
      MEMORY_PARTICIPATING_TEAMS: [], MEMOTION_PARTICIPATING_TEAMS: [], MEMOTION_CARD_STATES: {}, MEMOTION_CARD_TEAMS: {},
      MEMOTION_CURRENT_TEAM: null, MEMOTION_SELECTED: null, newGameBackgrounds: [], SCORE_CHANGE: { SEQ: 0, DISPLAY: true, TEAMS: [], PLAYERS: [] },
      ROUNDS: over.rounds ?? ROUNDS, ROUND_CONTEXT: roundContext,
    },
    teams: {}, bumpers: {}, flipMemoryCard: vi.fn(), showQRCode: false, selectMotionCard: vi.fn(),
  })
  return render(<PlayerDisplay />)
}
const rule = (selector) => css.match(new RegExp(`${selector.replace('.', '\\.')}\\s*\\{[^}]*\\}`))?.[0] || ''

describe('TV — manche courante (#245, scénario 20)', () => {
  beforeEach(() => { useGame.mockReset() })

  it.each(['READY', 'STARTED', 'STOPPED', 'REVEALED'])('pastille « Manche 2 · Deux » en phase %s quand ROUND_INDEX > 0', (phase) => {
    mount(phase, ctx())
    expect(screen.getByText('Manche 2 · Deux')).toBeInTheDocument()
  })

  it('Critical — rien sans manche : ROUND_INDEX 0 et ROUNDS vide → ni pastille ni titre', () => {
    mount('READY', ctx({ ROUND_ID: '', ROUND_INDEX: 0, ROUND_COUNT: 0, POSITION: 0, TOTAL: 0, FIRST_OF_ROUND: false }), { rounds: [] })
    expect(screen.queryByText(/^Manche \d/)).toBeNull()
  })

  it('question « Sans manche » alors que le quiz a des manches (ROUND_INDEX 0) : pas de pastille', () => {
    mount('STARTED', ctx({ ROUND_ID: '', ROUND_INDEX: 0, POSITION: 2, TOTAL: 4 }))
    expect(screen.queryByText(/^Manche \d/)).toBeNull()
  })

  it('titre de manche « Manche 2 — Deux » SEULEMENT sur l’écran d’attente (READY) de la première question de la manche', () => {
    mount('READY', ctx({ FIRST_OF_ROUND: true }))
    expect(screen.getByText('Manche 2 — Deux')).toBeInTheDocument()
  })

  it.each([['READY', false], ['STARTED', true], ['REVEALED', true]])('pas de titre de manche en phase %s (FIRST_OF_ROUND %s)', (phase, first) => {
    mount(phase, ctx({ FIRST_OF_ROUND: first }))
    expect(screen.queryByText('Manche 2 — Deux')).toBeNull()
  })

  it('pas de titre pour une question qui n’est pas la première de sa manche', () => {
    mount('READY', ctx({ FIRST_OF_ROUND: false }))
    expect(screen.queryByText('Manche 2 — Deux')).toBeNull()
    expect(screen.getByText('Manche 2 · Deux')).toBeInTheDocument()
  })
})

describe('TV statique — aucun défilement ni débordement (#245, scénario 20)', () => {
  it.each(['.tv-round-pill', '.tv-round-title'])('Critical — %s : overflow hidden, jamais auto/scroll ; texte tronqué sur une ligne', (sel) => {
    const r = rule(sel)
    expect(r, `règle ${sel} introuvable`).not.toBe('')
    expect(r).toMatch(/overflow:\s*hidden/)
    expect(r).not.toMatch(/overflow(-[xy])?:\s*(auto|scroll)/)
    expect(r).toMatch(/white-space:\s*nowrap/)
    expect(r).toMatch(/text-overflow:\s*ellipsis/)
  })

  it.each(['.tv-round-pill', '.tv-round-title'])('%s : unités d’écran (vh / vw) pour la taille du texte et la position, aucune taille en px', (sel) => {
    const r = rule(sel)
    expect(r).toMatch(/font-size:\s*[\d.]+vh/)
    expect(r).toMatch(/(top|left):\s*[\d.]+(vh|vw)/)
    expect(r).not.toMatch(/(font-size|top|left|right|max-width):\s*[\d.]+px/)
  })

  it('les deux éléments sont positionnés en absolu : ils ne poussent aucun contenu (aucune mise en page ne peut déborder)', () => {
    for (const sel of ['.tv-round-pill', '.tv-round-title']) expect(rule(sel)).toMatch(/position:\s*absolute/)
  })
})
