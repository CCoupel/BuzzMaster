import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render } from '@testing-library/react'
import PlayerDisplay from './PlayerDisplay'

// ---------------------------------------------------------------------------
// PlayerDisplay (TV) — mention CA15 "le chrono démarre à la fin du son"
// (v11.1 #219, contrat sound.md §10.7, R11). Patron mocks :
// PlayerDisplay.rafale.test.jsx (fixture gameState complète évitant tout
// crash de rendu sur les blocs RAFALE/MEMORY/MEMOTION non pertinents ici).
//
// Dispatché en correctif de revue (finding MAJEUR,
// _work/reports/code-review-frontend-20260922-150000.md) — le plan initial
// ne listait aucun test JS pour ce lot.
//
// Périmètre : les DEUX zones réellement concernées (QCM STARTED, et
// SPEEDY/ARDOISE STARTED — confirmé exhaustif par le sous-reviewer
// frontend, section 2 du rapport de revue) — état diffusé par le serveur
// (gameState.ANSWER_TIMER_WAITING), jamais déduit côté client (R4).
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

const SPEEDY_QUESTION = { ID: 'q-speedy-1', TYPE: 'SPEEDY', QUESTION: 'Capitale de la France ?', ANSWER: 'Paris' }
const ARDOISE_QUESTION = { ID: 'q-ardoise-1', TYPE: 'ARDOISE', QUESTION: 'Écris un mot', ANSWER: 'mot' }
const QCM_QUESTION = {
  ID: 'q-qcm-1', TYPE: 'QCM', QUESTION: 'Capitale de l\'Italie ?',
  QCM_ANSWERS: { RED: 'Rome', GREEN: 'Milan', YELLOW: 'Turin', BLUE: 'Naples' },
}

function makeMock({ phase = 'STARTED', question = SPEEDY_QUESTION, answerTimerWaiting = false } = {}) {
  return {
    gameState: {
      phase,
      remote: 'GAME',
      timer: 15,
      totalTime: 20,
      question,
      ANSWER_TIMER_WAITING: answerTimerWaiting,
      QUESTION_SOUND_STATE: 'PLAYING',
      RAFALE_CURRENT_QUESTION: null,
      RAFALE_QUESTION_TIME: 0,
      RAFALE_PARTICIPATING_TEAMS: [],
      RAFALE_CURRENT_TEAM: '',
      RAFALE_CURRENT_TEAM_COLOR: [],
      RAFALE_TEAM_COUNTERS: {},
      ARDOISE_ANSWERS: {},
      MEMORY_PARTICIPATING_TEAMS: [],
      MEMOTION_PARTICIPATING_TEAMS: [],
      MEMOTION_CARD_STATES: {},
      MEMOTION_CARD_TEAMS: {},
      MEMOTION_CURRENT_TEAM: null,
      MEMOTION_SELECTED: null,
      newGameBackgrounds: [],
    },
    teams: {},
    bumpers: {},
    flipMemoryCard: vi.fn(),
    showQRCode: false,
    selectMotionCard: vi.fn(),
  }
}

const renderTV = (overrides = {}) => {
  useGame.mockReturnValue(makeMock(overrides))
  return render(<PlayerDisplay />)
}

beforeEach(() => {
  vi.clearAllMocks()
  global.fetch = vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve({ ssid: '', password: '' }) })
  Object.defineProperty(document.documentElement, 'requestFullscreen', {
    value: vi.fn().mockResolvedValue(undefined),
    writable: true,
    configurable: true,
  })
})

describe('PlayerDisplay (TV) — mention CA15, zone SPEEDY/ARDOISE', () => {
  it('ANSWER_TIMER_WAITING=true, TYPE=SPEEDY, STARTED : la mention est affichée', () => {
    const { container } = renderTV({ question: SPEEDY_QUESTION, answerTimerWaiting: true })
    const badge = container.querySelector('.sound-wait-badge')
    expect(badge).not.toBeNull()
    expect(badge.textContent).toContain('le chrono démarre à la fin du son')
  })

  it('ANSWER_TIMER_WAITING=false, TYPE=SPEEDY, STARTED : aucune mention (jamais un chiffre figé sans explication, mais pas non plus une mention fausse)', () => {
    const { container } = renderTV({ question: SPEEDY_QUESTION, answerTimerWaiting: false })
    expect(container.querySelector('.sound-wait-badge')).toBeNull()
  })

  it('ANSWER_TIMER_WAITING=true, TYPE=ARDOISE, STARTED : la mention est affichée (même zone que SPEEDY)', () => {
    const { container } = renderTV({ question: ARDOISE_QUESTION, answerTimerWaiting: true })
    expect(container.querySelector('.sound-wait-badge')).not.toBeNull()
  })
})

describe('PlayerDisplay (TV) — mention CA15, zone QCM', () => {
  it('ANSWER_TIMER_WAITING=true, TYPE=QCM, STARTED : la mention est affichée', () => {
    const { container } = renderTV({ question: QCM_QUESTION, answerTimerWaiting: true })
    const badge = container.querySelector('.sound-wait-badge')
    expect(badge).not.toBeNull()
    expect(badge.textContent).toContain('le chrono démarre à la fin du son')
  })

  it('ANSWER_TIMER_WAITING=false, TYPE=QCM, STARTED : aucune mention', () => {
    const { container } = renderTV({ question: QCM_QUESTION, answerTimerWaiting: false })
    expect(container.querySelector('.sound-wait-badge')).toBeNull()
  })
})

describe('PlayerDisplay (TV) — la mention n\'apparaît jamais avant que la question soit montée à l\'écran', () => {
  // COUNTDOWN/READY sont exclus de showGameContent (PlayerDisplay.jsx) — le
  // bloc .game-content-zones entier (dont le badge) n'est pas rendu, quelle
  // que soit la valeur d'ANSWER_TIMER_WAITING. STOPPED/REVEALED sont
  // volontairement HORS de ce test : ils sont inclus dans showGameContent
  // (l'écran affiche la question arrêtée avant révélation), donc le badge
  // peut légitimement s'y rendre si le serveur envoyait ANSWER_TIMER_WAITING
  // =true — un cas que le moteur ne produit normalement pas (Stop()/Reveal()
  // remettent toujours ce champ à false, contract §10.7), mais que ce
  // composant n'a pas à re-vérifier lui-même (R4 : il fait confiance à
  // l'état serveur, jamais de garde locale dupliquée).
  it.each(['COUNTDOWN', 'READY'])('phase=%s : ANSWER_TIMER_WAITING=true n\'affiche rien (la zone STARTED elle-même n\'est pas montée)', (phase) => {
    const { container } = renderTV({ phase, question: SPEEDY_QUESTION, answerTimerWaiting: true })
    expect(container.querySelector('.sound-wait-badge')).toBeNull()
  })
})

// ---------------------------------------------------------------------------
// Contrainte TV STATIQUE (CLAUDE.md) — la mention ne doit ajouter aucune
// hauteur au flux (position absolue, revue confirmée : réserve non
// bloquante sur un chevauchement visuel possible, pas un manquement).
// ---------------------------------------------------------------------------

describe('PlayerDisplay.css — .sound-wait-badge reste en position absolue (TV statique)', () => {
  it('PlayerDisplay.css positionne .sound-wait-badge en absolute, jamais dans le flux normal', async () => {
    const fs = await import('node:fs')
    const path = await import('node:path')
    const { fileURLToPath } = await import('node:url')
    const cssPath = path.join(path.dirname(fileURLToPath(import.meta.url)), 'PlayerDisplay.css')
    const css = fs.readFileSync(cssPath, 'utf-8')
    const rule = css.match(/\.sound-wait-badge\s*\{([^}]*)\}/)
    expect(rule).not.toBeNull()
    expect(rule[1]).toMatch(/position:\s*absolute/)
  })
})
