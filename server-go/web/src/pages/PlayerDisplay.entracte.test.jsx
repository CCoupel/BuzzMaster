import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import PlayerDisplay from './PlayerDisplay'

// ---------------------------------------------------------------------------
// PlayerDisplay (TV) — mode ENTRACTE (#119), tâches F4/T3 du plan
// _work/reports/plan-entracte-119-20260820-140825.md.
//
// Point central : le filtre `.entracte-dim` est conditionné par !isVPlayer
// (F4) pour éviter le double filtrage quand VPlayerPage monte
// <PlayerDisplay isVPlayer>. Mêmes mocks que PlayerDisplay.qrcode.test.jsx.
// ---------------------------------------------------------------------------

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
  default: () => <div data-testid="qrcode-overlay" />,
}))

vi.mock('../components/QRCodeDisplay', () => ({
  default: ({ url }) => <div data-testid="qr-display" data-url={url} />,
}))

vi.mock('../components/EntractePanel', () => ({
  default: ({ config }) => <div data-testid="entracte-panel" data-title={config?.TITLE} />,
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
vi.mock('../styles/entracte.css', () => ({}))

import { useGame } from '../hooks/GameContext'

const BASE_GAME_STATE = {
  phase: 'STOPPED',
  remote: null,
  question: null,
  timer: 0,
  totalTime: 0,
  virtualPlayerCount: 3,
  virtualPlayerLimit: 20,
  backgrounds: [],
  currentBackgroundIndex: 0,
  memoryMatchedPairs: [],
  neonEffect: { enabled: false },
  entracte: false,
  entracteConfig: { TITLE: 'ENTRACTE', SUBTITLE: 'Retour dans 20mn', IMAGE_IS_CUSTOM: false, PANEL_SIZE: 65, ANIM_PERIOD: 10, ANIM_INTENSITY: 20 },
}

function mockUseGame(overrides = {}) {
  useGame.mockReturnValue({
    gameState: { ...BASE_GAME_STATE, ...overrides },
    teams: {},
    bumpers: {},
    flipMemoryCard: vi.fn(),
    showQRCode: vi.fn(),
    selectMotionCard: vi.fn(),
  })
}

beforeEach(() => {
  global.fetch = vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve({ ssid: '', password: '' }) })
})

afterEach(() => {
  vi.clearAllMocks()
})

describe('PlayerDisplay (TV) — filtre .entracte-dim (T3)', () => {
  it('aucun filtre appliqué quand entracte est false', () => {
    mockUseGame({ entracte: false })
    const { container } = render(<PlayerDisplay />)
    const content = container.querySelector('.entracte-content')
    expect(content).not.toBeNull()
    expect(content.classList.contains('entracte-dim')).toBe(false)
  })

  it('filtre appliqué quand entracte est true (surface TV, isVPlayer=false)', () => {
    mockUseGame({ entracte: true })
    const { container } = render(<PlayerDisplay />)
    const content = container.querySelector('.entracte-content')
    expect(content.classList.contains('entracte-dim')).toBe(true)
  })

  // C3 (delta #119, plan-entracte-119-fixes-20260820-155123.md) — la durée
  // du fondu (TRANSITION_MS) doit être posée sur .entracte-content, la
  // classe de base — jamais seulement dans .entracte-dim, sans quoi la
  // transition jouerait à l'entrée mais pas au retour (cf. commentaire C3
  // du plan).
  it('pose --ep-transition sur .entracte-content depuis entracteConfig.TRANSITION_MS', () => {
    mockUseGame({ entracte: true, entracteConfig: { ...BASE_GAME_STATE.entracteConfig, TRANSITION_MS: 1500 } })
    const { container } = render(<PlayerDisplay />)
    const content = container.querySelector('.entracte-content')
    expect(content.style.getPropertyValue('--ep-transition')).toBe('1500ms')
  })

  it('PAS de filtre côté TV quand isVPlayer=true, même si entracte est true (F4 : évite le double filtrage avec VPlayerPage)', () => {
    mockUseGame({ entracte: true })
    const { container } = render(<PlayerDisplay isVPlayer={true} />)
    const content = container.querySelector('.entracte-content')
    expect(content.classList.contains('entracte-dim')).toBe(false)
  })
})

describe('PlayerDisplay (TV) — panneau ENTRACTE (T3)', () => {
  it('rend le panneau avec la config reçue quand entracte est actif (TV)', () => {
    mockUseGame({ entracte: true, entracteConfig: { ...BASE_GAME_STATE.entracteConfig, TITLE: 'Pause déjeuner' } })
    render(<PlayerDisplay />)
    const panel = screen.getByTestId('entracte-panel')
    expect(panel).toBeInTheDocument()
    expect(panel.getAttribute('data-title')).toBe('Pause déjeuner')
  })

  it("n'affiche aucun panneau quand entracte est false", () => {
    mockUseGame({ entracte: false })
    render(<PlayerDisplay />)
    expect(screen.queryByTestId('entracte-panel')).toBeNull()
  })

  it("ne rend PAS son propre panneau côté isVPlayer (c'est VPlayerPage qui en monte un) même si entracte est true", () => {
    mockUseGame({ entracte: true })
    render(<PlayerDisplay isVPlayer={true} />)
    expect(screen.queryByTestId('entracte-panel')).toBeNull()
  })
})

describe('PlayerDisplay (TV) — éléments position:fixed hors du wrapper filtré (non-régression)', () => {
  it("QRCodeOverlay reste monté (hors .entracte-content) même filtre actif", () => {
    mockUseGame({ entracte: true, phase: 'ENROLL' })
    const { container } = render(<PlayerDisplay />)
    const overlay = screen.getByTestId('qrcode-overlay')
    const content = container.querySelector('.entracte-content')
    // L'overlay ne doit pas être un descendant du nœud filtré (piège position:fixed)
    expect(content.contains(overlay)).toBe(false)
  })
})

// ---------------------------------------------------------------------------
// TYPE=ENTRACTE — dispatch positif (#214, milestone v9.0.0, Batch 3).
//
// ⚠️ Piège signalé explicitement au cadrage : sans un dispatch POSITIF
// (isKnownOtherType), un type de question inconnu du switch tombe
// SILENCIEUSEMENT dans le rendu par défaut SPEEDY (PlayerDisplay.jsx:646-670)
// — pas d'erreur, pas de crash, juste le mauvais contenu affiché. Ce fichier
// vérifie le symptôme observable : le contenu SPEEDY (texte de la question)
// ne doit JAMAIS apparaître pour une question ENTRACTE, y compris PENDANT la
// fenêtre entre START et la propagation du drapeau `entracte` (le panneau
// EntractePanel lui-même est couvert par les describe ci-dessus — hors
// périmètre de ce bloc, qui teste le DISPATCH, pas le panneau).
// ---------------------------------------------------------------------------

describe('PlayerDisplay (TV) — TYPE=ENTRACTE : dispatch positif, jamais de chute silencieuse vers SPEEDY (#214)', () => {
  const ENTRACTE_QUESTION = { ID: 'q-entracte-1', TYPE: 'ENTRACTE', QUESTION: 'CE TEXTE NE DOIT JAMAIS APPARAÎTRE (repli SPEEDY)' }

  it("phase STARTED, entracte pas encore levé (fenêtre START→propagation) : le texte SPEEDY de la question n'apparaît jamais", () => {
    mockUseGame({ phase: 'STARTED', question: ENTRACTE_QUESTION, entracte: false })
    render(<PlayerDisplay />)
    expect(screen.queryByText(ENTRACTE_QUESTION.QUESTION)).toBeNull()
  })

  it('phase STARTED, entracte levé : le texte SPEEDY de la question n\'apparaît toujours pas (le panneau ENTRACTE porte le message)', () => {
    mockUseGame({ phase: 'STARTED', question: ENTRACTE_QUESTION, entracte: true })
    render(<PlayerDisplay />)
    expect(screen.queryByText(ENTRACTE_QUESTION.QUESTION)).toBeNull()
  })

  it('ne plante pas et rend une zone de contenu (pas un écran cassé/vide) pour TYPE=ENTRACTE', () => {
    mockUseGame({ phase: 'STARTED', question: ENTRACTE_QUESTION, entracte: false })
    const { container } = render(<PlayerDisplay />)
    expect(container.querySelector('.game-content-zones')).not.toBeNull()
  })
})
