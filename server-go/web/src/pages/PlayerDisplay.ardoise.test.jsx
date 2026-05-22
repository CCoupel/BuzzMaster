import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import PlayerDisplay from './PlayerDisplay'

// ---------------------------------------------------------------------------
// Mocks — PlayerDisplay imports
// framer-motion est auto-mocké via l'alias vite.config.js (src/mocks/framer-motion.jsx)
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

// ---------------------------------------------------------------------------
// Import useGame après les mocks
// ---------------------------------------------------------------------------
import { useGame } from '../hooks/GameContext'

// ---------------------------------------------------------------------------
// Données de test ARDOISE
// ---------------------------------------------------------------------------

const ARDOISE_QUESTION = {
  ID: 'q-ardoise-1',
  TYPE: 'ARDOISE',
  QUESTION: 'Quel est le plus long fleuve du monde ?',
  ANSWER: 'Le Nil',
  ARDOISE_KEYBOARD_TYPE: 'AZERTY',
}

const TEAMS_2 = {
  'Équipe A': { NAME: 'Équipe A', SCORE: 10, COLOR: [99, 102, 241] },
  'Équipe B': { NAME: 'Équipe B', SCORE: 5,  COLOR: [234, 179, 8] },
}

const TEAMS_10 = Object.fromEntries(
  Array.from({ length: 10 }, (_, i) => [
    `Équipe ${i + 1}`,
    { NAME: `Équipe ${i + 1}`, SCORE: i, COLOR: [100 + i * 10, 100, 200] },
  ])
)

const BUMPERS_10 = Object.fromEntries(
  Array.from({ length: 10 }, (_, i) => [
    `AA:00:00:00:00:${String(i + 1).padStart(2, '0')}`,
    { TEAM: `Équipe ${i + 1}`, NAME: `VJoueur-${i + 1}`, IS_VPLAYER: true, CONNECTED: true, COLOR: [100 + i * 10, 100, 200] },
  ])
)

/**
 * Bumpers par défaut : 2 VJoueurs (IS_VPLAYER: true) pour Équipe A et Équipe B.
 * Après fix #93, PlayerDisplay filtre les lignes sur IS_VPLAYER=true.
 */
const DEFAULT_BUMPERS = {
  'AA:00:00:00:00:01': { TEAM: 'Équipe A', NAME: 'VJoueur-A', IS_VPLAYER: true, CONNECTED: true, COLOR: [99, 102, 241] },
  'AA:00:00:00:00:02': { TEAM: 'Équipe B', NAME: 'VJoueur-B', IS_VPLAYER: true, CONNECTED: true, COLOR: [234, 179, 8] },
}

/**
 * Construit un mock useGame pour un état ARDOISE.
 */
const makeArdoiseMock = ({
  phase = 'REVEALED',
  ardoiseAnswers = {},
  teams = TEAMS_2,
  question = ARDOISE_QUESTION,
  bumpers = DEFAULT_BUMPERS,
} = {}) => ({
  gameState: {
    phase,
    remote: 'GAME',
    timer: 0,
    totalTime: 30,
    question,
    ARDOISE_ANSWERS: ardoiseAnswers,
    MEMORY_PARTICIPATING_TEAMS: [],
    MEMOTION_PARTICIPATING_TEAMS: [],
    MEMOTION_CARD_STATES: {},
    MEMOTION_CARD_TEAMS: {},
    MEMOTION_CURRENT_TEAM: null,
    MEMOTION_SELECTED: null,
    newGameBackgrounds: [],
  },
  teams,
  bumpers,
  flipMemoryCard: vi.fn(),
  showQRCode: false,
  selectMotionCard: vi.fn(),
})

// ---------------------------------------------------------------------------
// Helper : render TV (isVPlayer = false, par défaut)
// ---------------------------------------------------------------------------
const renderTV = (mockOverrides = {}) => {
  useGame.mockReturnValue(makeArdoiseMock(mockOverrides))
  return render(<PlayerDisplay />)
}

const renderVPlayer = (mockOverrides = {}) => {
  useGame.mockReturnValue(makeArdoiseMock(mockOverrides))
  return render(
    <PlayerDisplay
      isVPlayer={true}
      playerName="Joueur1"
      playerNameColor={[99, 102, 241]}
      teamName="Équipe A"
      teamColor={[99, 102, 241]}
    />
  )
}

// ---------------------------------------------------------------------------
// Tests : PlayerDisplay — ARDOISE TV Reveal (#90)
// ---------------------------------------------------------------------------

describe('PlayerDisplay — ARDOISE TV Reveal : affichage de l\'écran (#90)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    // Évite l'erreur "Not implemented: requestFullscreen" dans jsdom
    Object.defineProperty(document.documentElement, 'requestFullscreen', {
      value: vi.fn().mockResolvedValue(undefined),
      writable: true,
      configurable: true,
    })
  })

  it('affiche le bloc game-content-zones en phase REVEALED sur TV (non-VPlayer)', () => {
    const { container } = renderTV({ phase: 'REVEALED' })

    expect(container.querySelector('.game-content-zones')).not.toBeNull()
  })

  it('affiche la bonne réponse dans ardoise-correct-answer', () => {
    const { container } = renderTV({ phase: 'REVEALED' })

    expect(container.querySelector('.ardoise-correct-answer')?.textContent).toContain('Le Nil')
  })

  it('affiche le bloc ardoise-correct-answer', () => {
    const { container } = renderTV({ phase: 'REVEALED' })

    expect(container.querySelector('.ardoise-correct-answer')).not.toBeNull()
  })

  it('affiche la zone ardoise-correct-answer', () => {
    const { container } = renderTV({ phase: 'REVEALED' })

    expect(container.querySelector('.ardoise-correct-answer')).not.toBeNull()
  })
})

describe('PlayerDisplay — ARDOISE TV Reveal : réponses équipes (#90)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    Object.defineProperty(document.documentElement, 'requestFullscreen', {
      value: vi.fn().mockResolvedValue(undefined),
      writable: true,
      configurable: true,
    })
  })

  it('affiche une card par équipe dans ardoise-teams-grid', () => {
    const { container } = renderTV({
      phase: 'REVEALED',
      teams: TEAMS_2,
      ardoiseAnswers: {},
    })

    const teamCards = container.querySelectorAll('.ardoise-team-card')
    expect(teamCards.length).toBe(2)
  })

  it('affiche le badge avec le nom de l\'équipe', () => {
    renderTV({
      phase: 'REVEALED',
      teams: TEAMS_2,
      ardoiseAnswers: {},
    })

    expect(screen.getByText('Équipe A')).toBeInTheDocument()
    expect(screen.getByText('Équipe B')).toBeInTheDocument()
  })

  it('affiche le texte de réponse quand l\'équipe a répondu', () => {
    renderTV({
      phase: 'REVEALED',
      teams: TEAMS_2,
      ardoiseAnswers: {
        'Équipe A': { TEXT: 'Le Nil', SUBMITTED_AT: 1000 },
      },
    })

    expect(screen.getAllByText('Le Nil').length).toBeGreaterThanOrEqual(1)
  })

  it('affiche "—" pour une équipe sans réponse', () => {
    renderTV({
      phase: 'REVEALED',
      teams: TEAMS_2,
      ardoiseAnswers: {},
    })

    const emptyResponses = screen.getAllByText('—')
    expect(emptyResponses.length).toBe(2) // Both teams have no answer
  })

  it('la card d\'équipe avec réponse a la classe "has-answer"', () => {
    const { container } = renderTV({
      phase: 'REVEALED',
      teams: TEAMS_2,
      ardoiseAnswers: {
        'Équipe A': { TEXT: 'Le Nil', SUBMITTED_AT: 1000 },
      },
    })

    const hasAnswerCards = container.querySelectorAll('.ardoise-team-card.has-answer')
    expect(hasAnswerCards.length).toBe(1)
  })

  it('la card d\'équipe sans réponse n\'a pas la classe "has-answer"', () => {
    const { container } = renderTV({
      phase: 'REVEALED',
      teams: TEAMS_2,
      ardoiseAnswers: {},
    })

    const hasAnswerCards = container.querySelectorAll('.ardoise-team-card.has-answer')
    expect(hasAnswerCards.length).toBe(0)
  })

  it('.ardoise-team-card-answer contient le texte de réponse quand remplie', () => {
    const { container } = renderTV({
      phase: 'REVEALED',
      teams: TEAMS_2,
      ardoiseAnswers: {
        'Équipe A': { TEXT: 'Le Nil', SUBMITTED_AT: 1000 },
      },
    })

    const answerElements = container.querySelectorAll('.ardoise-team-card-answer')
    const withText = Array.from(answerElements).find(el => el.textContent === 'Le Nil')
    expect(withText).not.toBeUndefined()
  })

  it('la card sans réponse a la classe "no-answer"', () => {
    const { container } = renderTV({
      phase: 'REVEALED',
      teams: TEAMS_2,
      ardoiseAnswers: {},
    })

    const noAnswerCards = container.querySelectorAll('.ardoise-team-card.no-answer')
    expect(noAnswerCards.length).toBe(2)
  })
})

describe('PlayerDisplay — ARDOISE TV Reveal : contrainte max 8 équipes (#90)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    Object.defineProperty(document.documentElement, 'requestFullscreen', {
      value: vi.fn().mockResolvedValue(undefined),
      writable: true,
      configurable: true,
    })
  })

  it('affiche au maximum 8 équipes même si 10 sont présentes', () => {
    const { container } = renderTV({
      phase: 'REVEALED',
      teams: TEAMS_10,
      bumpers: BUMPERS_10,
      ardoiseAnswers: {},
    })

    const teamCards = container.querySelectorAll('.ardoise-team-card')
    expect(teamCards.length).toBeLessThanOrEqual(8)
  })
})

describe('PlayerDisplay — ARDOISE TV Reveal : pas de rendu sur VPlayer (#90)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    Object.defineProperty(document.documentElement, 'requestFullscreen', {
      value: vi.fn().mockResolvedValue(undefined),
      writable: true,
      configurable: true,
    })
  })

  it('n\'affiche pas le bloc ardoise-reveal-layout sur VPlayer', () => {
    const { container } = renderVPlayer({ phase: 'REVEALED' })

    expect(container.querySelector('.ardoise-reveal-layout')).toBeNull()
  })

  it('n\'affiche pas "Réponse correcte" sur VPlayer en REVEALED', () => {
    renderVPlayer({ phase: 'REVEALED' })

    expect(screen.queryByText('Réponse correcte')).toBeNull()
  })
})

describe('PlayerDisplay — ARDOISE : bloc NORMAL pendant STARTED (#90)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    Object.defineProperty(document.documentElement, 'requestFullscreen', {
      value: vi.fn().mockResolvedValue(undefined),
      writable: true,
      configurable: true,
    })
  })

  it('n\'affiche pas ardoise-reveal-layout en phase STARTED', () => {
    const { container } = renderTV({ phase: 'STARTED' })

    expect(container.querySelector('.ardoise-reveal-layout')).toBeNull()
  })

  it('n\'affiche pas ardoise-reveal-layout en phase PAUSED', () => {
    const { container } = renderTV({ phase: 'PAUSED' })

    expect(container.querySelector('.ardoise-reveal-layout')).toBeNull()
  })
})

// ---------------------------------------------------------------------------
// Tests #92 — Structure DOM zones REVEALED (#92)
// ---------------------------------------------------------------------------

describe('PlayerDisplay — ARDOISE TV Reveal : structure DOM zones (#92)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    Object.defineProperty(document.documentElement, 'requestFullscreen', {
      value: vi.fn().mockResolvedValue(undefined),
      writable: true,
      configurable: true,
    })
  })

  it('contient zone-question dans le bloc ARDOISE REVEALED TV', () => {
    const { container } = renderTV({ phase: 'REVEALED' })

    // Le bloc ardoise-reveal contient zone-answers.ardoise-reveal — son parent est game-content-zones
    const ardoiseReveal = container.querySelector('.zone-answers')
    const revealBlock = ardoiseReveal?.closest('.game-content-zones')
    expect(revealBlock?.querySelector('.zone-question')).not.toBeNull()
  })

  it('contient zone-answers dans le bloc ARDOISE REVEALED TV', () => {
    const { container } = renderTV({ phase: 'REVEALED' })

    expect(container.querySelector('.zone-answers')).not.toBeNull()
  })

  it('zone-timer absente dans le bloc ARDOISE REVEALED TV', () => {
    const { container } = renderTV({ phase: 'REVEALED' })

    const ardoiseReveal = container.querySelector('.zone-answers')
    const revealBlock = ardoiseReveal?.closest('.game-content-zones')
    // Le bloc ARDOISE REVEALED ne doit pas contenir de zone-timer
    expect(revealBlock?.querySelector('.zone-timer')).toBeNull()
  })
})

// ---------------------------------------------------------------------------
// Tests #93 — Filtre VJoueur en REVEALED TV (#93)
// ---------------------------------------------------------------------------

describe('PlayerDisplay — ARDOISE TV Reveal : filtre VJoueur (#93)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    Object.defineProperty(document.documentElement, 'requestFullscreen', {
      value: vi.fn().mockResolvedValue(undefined),
      writable: true,
      configurable: true,
    })
  })

  it('n\'affiche que les équipes avec un VJoueur dans ardoise-teams-grid', () => {
    // Équipe A : VJoueur présent | Équipe B : seulement buzzer physique
    const { container } = renderTV({
      phase: 'REVEALED',
      teams: TEAMS_2,
      bumpers: {
        'MAC:A1': { TEAM: 'Équipe A', IS_VPLAYER: true,  NAME: 'VJoueur A' },
        'MAC:B1': { TEAM: 'Équipe B', IS_VPLAYER: false, NAME: 'Buzzer B'  },
      },
    })

    const teamCards = container.querySelectorAll('.ardoise-team-card')
    // Seule Équipe A doit apparaître
    expect(teamCards.length).toBe(1)
    expect(container.querySelector('.ardoise-team-card-header').textContent).toBe('Équipe A')
  })

  it('masque une équipe avec uniquement des buzzers physiques dans ardoise-teams-grid', () => {
    const { container } = renderTV({
      phase: 'REVEALED',
      teams: TEAMS_2,
      bumpers: {
        'MAC:A1': { TEAM: 'Équipe A', IS_VPLAYER: false, NAME: 'Buzzer A' },
        'MAC:B1': { TEAM: 'Équipe B', IS_VPLAYER: false, NAME: 'Buzzer B' },
      },
    })

    // Aucune équipe affichée — toutes ont seulement des buzzers physiques
    const teamCards = container.querySelectorAll('.ardoise-team-card')
    expect(teamCards.length).toBe(0)
  })
})

// ---------------------------------------------------------------------------
// Tests #94 — Images question/réponse ARDOISE (#94)
// ---------------------------------------------------------------------------

describe('PlayerDisplay — ARDOISE TV Reveal : images question/réponse (#94)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    Object.defineProperty(document.documentElement, 'requestFullscreen', {
      value: vi.fn().mockResolvedValue(undefined),
      writable: true,
      configurable: true,
    })
  })

  it('zone-media affiche une img quand question.MEDIA est défini', () => {
    const { container } = renderTV({
      phase: 'REVEALED',
      question: {
        ...ARDOISE_QUESTION,
        MEDIA: '/files/question-image.jpg',
      },
    })

    const zoneMedia = container.querySelector('.zone-media')
    expect(zoneMedia).not.toBeNull()
    expect(zoneMedia.querySelector('img')).not.toBeNull()
  })

  it('zone-media absente (pas d\'img rendue) quand question.MEDIA est vide', () => {
    const { container } = renderTV({
      phase: 'REVEALED',
      question: {
        ...ARDOISE_QUESTION,
        MEDIA: '',
      },
    })

    // zone-media est conditionnelle — absente si MEDIA vide
    const ardoiseReveal = container.querySelector('.zone-answers')
    const revealBlock = ardoiseReveal?.closest('.game-content-zones')
    expect(revealBlock?.querySelector('.zone-media')).toBeNull()
  })

  it('affiche l\'image réponse (MEDIA_ANSWER) dans zone-media quand définie', () => {
    const { container } = renderTV({
      phase: 'REVEALED',
      question: {
        ...ARDOISE_QUESTION,
        MEDIA_ANSWER: '/files/answer-image.jpg',
      },
    })

    // MEDIA_ANSWER est rendu dans zone-media (pas dans zone-answers)
    const revealBlock = container.querySelector('.game-content-zones')
    const zoneMedia = revealBlock?.querySelector('.zone-media')
    expect(zoneMedia).not.toBeNull()
    expect(zoneMedia?.querySelector('img')).not.toBeNull()
    expect(zoneMedia?.querySelector('img').getAttribute('src')).toBe('/files/answer-image.jpg')
  })
})

// ---------------------------------------------------------------------------
// Tests — Layout cards ARDOISE REVEALED (#90 refacto)
// ---------------------------------------------------------------------------

describe('PlayerDisplay — ARDOISE TV Reveal : layout cards (#90 refacto)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    Object.defineProperty(document.documentElement, 'requestFullscreen', {
      value: vi.fn().mockResolvedValue(undefined),
      writable: true,
      configurable: true,
    })
  })

  it('.ardoise-teams-grid est présent dans zone-answers en phase REVEALED', () => {
    const { container } = renderTV({ phase: 'REVEALED' })

    const zoneAnswers = container.querySelector('.zone-answers')
    expect(zoneAnswers?.querySelector('.ardoise-teams-grid')).not.toBeNull()
  })

  it('chaque équipe VJoueur est rendue comme une .ardoise-team-card', () => {
    const { container } = renderTV({
      phase: 'REVEALED',
      teams: TEAMS_2,
      ardoiseAnswers: {},
    })

    const cards = container.querySelectorAll('.ardoise-team-card')
    expect(cards.length).toBe(2)
  })

  it('.ardoise-team-card-header contient le nom de l\'équipe', () => {
    const { container } = renderTV({
      phase: 'REVEALED',
      teams: TEAMS_2,
      ardoiseAnswers: {},
    })

    const headers = container.querySelectorAll('.ardoise-team-card-header')
    const names = Array.from(headers).map(h => h.textContent)
    expect(names).toContain('Équipe A')
    expect(names).toContain('Équipe B')
  })

  it('.ardoise-team-card-answer contient le texte quand l\'équipe a répondu', () => {
    const { container } = renderTV({
      phase: 'REVEALED',
      teams: TEAMS_2,
      ardoiseAnswers: {
        'Équipe A': { TEXT: 'Le Nil', SUBMITTED_AT: 1000 },
      },
    })

    const answerEls = container.querySelectorAll('.ardoise-team-card-answer')
    const found = Array.from(answerEls).find(el => el.textContent === 'Le Nil')
    expect(found).not.toBeUndefined()
  })

  it('.ardoise-team-card.no-answer quand pas de réponse, .ardoise-team-card.has-answer si réponse', () => {
    const { container } = renderTV({
      phase: 'REVEALED',
      teams: TEAMS_2,
      ardoiseAnswers: {
        'Équipe A': { TEXT: 'Le Nil', SUBMITTED_AT: 1000 },
        // Équipe B : no answer
      },
    })

    expect(container.querySelectorAll('.ardoise-team-card.has-answer').length).toBe(1)
    expect(container.querySelectorAll('.ardoise-team-card.no-answer').length).toBe(1)
  })
})
