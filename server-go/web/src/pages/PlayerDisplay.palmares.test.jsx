/**
 * Tests for PlayerDisplay — PALMARES view (v5.7.10 #107)
 *
 * v5.7.10 : PALMARES utilise GET /palmares (endpoint unique pré-assemblé backend).
 * Plus de /history ni useCategories pour PALMARES — les données arrivant
 * directement dans PalmaresEntry : category, name, imageURL, color, teams[], players[].
 *
 * Cas couverts :
 *   1. Catégorie avec name → label affiché directement
 *   2. Catégorie custom avec imageURL → <img> affiché
 *   3. Entry sans imageURL → icône emoji (CATEGORIES[key]?.icon ?? '📷')
 *   4. Entry avec couleur → backgroundColor correct
 *   5. Entry name vide + key UNKNOWN → fallback "Inconnue"
 *   6. Entry name vide + key générique → fallback title case
 *   7. Classement équipes (ranks) avec ex aequo
 *   8. Palmares vide → "Aucun evenement enregistre"
 *   9. imageURL présent dans réponse /palmares → <img> affiché (test d'intégration)
 *  10. useCategories toujours actif (autres vues) — pas d'erreur rendu
 */
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import PlayerDisplay from './PlayerDisplay'

// ---------------------------------------------------------------------------
// Mocks — même pattern que PlayerDisplay.ardoise.test.jsx
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

// useCategories reste mocké : il est utilisé par les autres vues (READY, MEMOTION).
// PALMARES ne l'utilise plus depuis v5.7.10 — mais le composant l'appelle encore.
vi.mock('../hooks/useCategories', () => ({
  useCategories: vi.fn(),
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
// Import après les mocks
// ---------------------------------------------------------------------------
import { useGame } from '../hooks/GameContext'
import { useCategories } from '../hooks/useCategories'

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const PALMARES_GAME_STATE = {
  phase: 'STOPPED',
  remote: 'PALMARES',
  question: null,
  timer: 0,
  totalTime: 0,
  virtualPlayerCount: 0,
  virtualPlayerLimit: 20,
  backgrounds: [],
  currentBackgroundIndex: 0,
  memoryMatchedPairs: [],
  neonEffect: { enabled: false },
}

/**
 * Crée une entrée PalmaresEntry minimale pour les tests.
 * Correspond à la structure retournée par GET /palmares (v5.7.10).
 */
function makePalmaresEntry(category, overrides = {}) {
  return {
    category,
    name: '',           // résolu côté backend (vide si catégorie inconnue)
    imageURL: '',       // vide si pas d'image
    color: '',          // vide pour catégories custom sans couleur définie
    totalPoints: 10,
    teams: [{ name: 'Équipe Test', color: [99, 102, 241], points: 10 }],
    players: [],
    ...overrides,
  }
}

/**
 * Configure les mocks pour un rendu PALMARES v5.7.10.
 * fetch('/palmares') retourne les entrées spécifiées.
 * useCategories retourne vide par défaut (PALMARES ne l'utilise plus).
 */
function setupMocks({ palmaresEntries = [] } = {}) {
  useGame.mockReturnValue({
    gameState: PALMARES_GAME_STATE,
    teams: {},
    bumpers: {},
    flipMemoryCard: vi.fn(),
    showQRCode: vi.fn(),
    selectMotionCard: vi.fn(),
  })

  // useCategories encore nécessaire pour les autres vues — vide pour les tests PALMARES
  useCategories.mockReturnValue({
    categories: [],
    loading: false,
    error: null,
    refetch: vi.fn(),
  })

  // v5.7.10 : fetch('/palmares') est le seul endpoint utilisé par PALMARES
  global.fetch = vi.fn().mockResolvedValue({
    ok: true,
    json: () => Promise.resolve(palmaresEntries),
  })
}

// ---------------------------------------------------------------------------
// Tests — section 1 : labels et métadonnées
// ---------------------------------------------------------------------------

describe('PlayerDisplay — PALMARES v5.7.10 (GET /palmares)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  it('catégorie SCIENCE avec name résolu → label "Sciences & Nature" affiché', async () => {
    // Backend résout le name côté serveur → frontend affiche directement entry.name
    setupMocks({
      palmaresEntries: [makePalmaresEntry('SCIENCE', { name: 'Sciences & Nature', color: '#22c55e' })],
    })
    render(<PlayerDisplay />)
    await waitFor(() => {
      expect(screen.getByText('Sciences & Nature')).toBeInTheDocument()
    })
  })

  it('catégorie custom avec name → label affiché directement', async () => {
    // Catégorie custom résolue côté backend — pas de lookup apiCategories nécessaire
    setupMocks({
      palmaresEntries: [makePalmaresEntry('SPORT_EXTREME', {
        name: 'Sport Extrême',
        imageURL: '/files/categories/SPORT_EXTREME.png',
      })],
    })
    render(<PlayerDisplay />)
    await waitFor(() => {
      expect(screen.getByText('Sport Extrême')).toBeInTheDocument()
    })
  })

  it('entry name vide + category UNKNOWN → fallback "Inconnue"', async () => {
    // Backend retourne name="" pour les catégories non résolues
    // Frontend applique le fallback : category === 'UNKNOWN' → 'Inconnue'
    setupMocks({
      palmaresEntries: [makePalmaresEntry('UNKNOWN', { name: '' })],
    })
    render(<PlayerDisplay />)
    await waitFor(() => {
      expect(screen.getByText('Inconnue')).toBeInTheDocument()
    })
  })

  it('entry name vide + category générique → fallback title case "Machin Truc"', async () => {
    // Backend retourne name="" pour une clé sans résolution connue
    // Frontend : 'MACHIN_TRUC' → replace(/_/g,' ').lower().capitalize() → 'Machin Truc'
    setupMocks({
      palmaresEntries: [makePalmaresEntry('MACHIN_TRUC', { name: '' })],
    })
    render(<PlayerDisplay />)
    await waitFor(() => {
      expect(screen.getByText('Machin Truc')).toBeInTheDocument()
    })
  })

  // -------------------------------------------------------------------------
  // imageURL et icônes
  // -------------------------------------------------------------------------

  it('entry avec imageURL → <img> rendu dans le header PALMARES', async () => {
    // v5.7.10 : imageURL vient directement de l'entry — plus de lookup apiCategories
    setupMocks({
      palmaresEntries: [makePalmaresEntry('MON_JEU', {
        name: 'Mon Jeu',
        imageURL: '/files/categories/MON_JEU.png',
      })],
    })
    const { container } = render(<PlayerDisplay />)
    await waitFor(() => {
      const img = container.querySelector('.palmares-category-header img')
      expect(img).not.toBeNull()
      expect(img).toHaveAttribute('src', '/files/categories/MON_JEU.png')
    })
  })

  it('entry sans imageURL → pas de <img>, icône emoji affiché', async () => {
    // entry.imageURL = '' → catImageURL = null → <span class="palmares-category-icon">
    setupMocks({
      palmaresEntries: [makePalmaresEntry('SCIENCE', {
        name: 'Sciences & Nature',
        imageURL: '',
        color: '#22c55e',
      })],
    })
    const { container } = render(<PlayerDisplay />)
    await waitFor(() => {
      expect(screen.getByText('Sciences & Nature')).toBeInTheDocument()
    })
    const img = container.querySelector('.palmares-category-header img')
    expect(img).toBeNull()
    const icon = container.querySelector('.palmares-category-icon')
    expect(icon).not.toBeNull()
  })

  it('entry avec color → backgroundColor du header correct', async () => {
    // entry.color = '#22c55e' → header style backgroundColor = 'rgb(34, 197, 94)'
    setupMocks({
      palmaresEntries: [makePalmaresEntry('SCIENCE', {
        name: 'Sciences & Nature',
        color: '#22c55e',
      })],
    })
    const { container } = render(<PlayerDisplay />)
    await waitFor(() => {
      const header = container.querySelector('.palmares-category-header')
      expect(header).not.toBeNull()
      expect(header.style.backgroundColor).toBe('rgb(34, 197, 94)')
    })
  })

  it('entry color vide → fallback gris #6b7280 dans le header', async () => {
    // Catégories custom sans color définie → entry.color = '' → fallback '#6b7280'
    setupMocks({
      palmaresEntries: [makePalmaresEntry('SPORT_EXTREME', {
        name: 'Sport Extrême',
        color: '',
      })],
    })
    const { container } = render(<PlayerDisplay />)
    await waitFor(() => {
      const header = container.querySelector('.palmares-category-header')
      expect(header).not.toBeNull()
      // jsdom normalise '#6b7280' → 'rgb(107, 114, 128)'
      expect(header.style.backgroundColor).toBe('rgb(107, 114, 128)')
    })
  })

  // -------------------------------------------------------------------------
  // Classement équipes et médailles
  // -------------------------------------------------------------------------

  it('équipes triées desc → médailles 🥇🥈🥉 affichées dans le bon ordre', async () => {
    setupMocks({
      palmaresEntries: [makePalmaresEntry('SCIENCE', {
        name: 'Sciences & Nature',
        color: '#22c55e',
        teams: [
          { name: 'Rouges', color: [239, 68, 68], points: 30 },
          { name: 'Verts',  color: [34, 197, 94], points: 20 },
          { name: 'Bleus',  color: [59, 130, 246], points: 10 },
        ],
      })],
    })
    render(<PlayerDisplay />)
    await waitFor(() => {
      expect(screen.getByText('Rouges')).toBeInTheDocument()
    })
    // Les médailles doivent apparaître dans le DOM dans l'ordre
    const medals = screen.getAllByText(/🥇|🥈|🥉/)
    expect(medals[0].textContent).toBe('🥇')
    expect(medals[1].textContent).toBe('🥈')
    expect(medals[2].textContent).toBe('🥉')
  })

  it('ex aequo équipes → même médaille pour points identiques', async () => {
    // teams[0].points === teams[1].points → rank 1 pour les deux → 🥇🥇🥉
    setupMocks({
      palmaresEntries: [makePalmaresEntry('SCIENCE', {
        name: 'Sciences & Nature',
        color: '#22c55e',
        teams: [
          { name: 'Rouges', color: [239, 68, 68], points: 20 },
          { name: 'Verts',  color: [34, 197, 94], points: 20 },
          { name: 'Bleus',  color: [59, 130, 246], points: 10 },
        ],
      })],
    })
    render(<PlayerDisplay />)
    await waitFor(() => {
      expect(screen.getByText('Rouges')).toBeInTheDocument()
    })
    const medals = screen.getAllByText(/🥇|🥈|🥉/)
    // Rouges et Verts ex aequo → 🥇🥇, puis Bleus → 🥉
    expect(medals[0].textContent).toBe('🥇')
    expect(medals[1].textContent).toBe('🥇')
    expect(medals[2].textContent).toBe('🥉')
  })

  // -------------------------------------------------------------------------
  // Palmares vide
  // -------------------------------------------------------------------------

  it('palmares vide → affiche "Aucun evenement enregistre"', async () => {
    setupMocks({ palmaresEntries: [] })
    render(<PlayerDisplay />)
    await waitFor(() => {
      expect(screen.getByText('Aucun evenement enregistre')).toBeInTheDocument()
    })
  })

  // -------------------------------------------------------------------------
  // Fetch cible /palmares (et non /history)
  // -------------------------------------------------------------------------

  it('fetch cible /palmares — pas /history (v5.7.10)', async () => {
    setupMocks({
      palmaresEntries: [makePalmaresEntry('SCIENCE', { name: 'Sciences & Nature' })],
    })
    render(<PlayerDisplay />)
    await waitFor(() => {
      expect(screen.getByText('Sciences & Nature')).toBeInTheDocument()
    })
    // Le seul fetch déclenché doit cibler /palmares
    const calls = global.fetch.mock.calls.map(([url]) => url)
    expect(calls.some(url => url === '/palmares')).toBe(true)
    expect(calls.some(url => url === '/history')).toBe(false)
  })

  // -------------------------------------------------------------------------
  // totalPoints et points équipes affichés (checklist items 5 et 6)
  // -------------------------------------------------------------------------

  it('totalPoints → "👥 N pts" affiché dans le header catégorie', async () => {
    // totalTeamPoints = sum(teams[].points) → rendu "👥 N pts" dans .palmares-category-stats
    // entry par défaut : teams[0].points = 10 → totalTeamPoints = 10
    setupMocks({
      palmaresEntries: [makePalmaresEntry('SCIENCE', {
        name: 'Sciences & Nature',
        color: '#22c55e',
        // teams hérités du défaut makePalmaresEntry → [{ name:'Équipe Test', points:10 }]
      })],
    })
    render(<PlayerDisplay />)
    await waitFor(() => {
      expect(screen.getByText('Sciences & Nature')).toBeInTheDocument()
    })
    // totalTeamPoints = 10 → rendu "👥 10 pts"
    expect(screen.getByText('👥 10 pts')).toBeInTheDocument()
  })

  it('team points → "{N} pts" affiché pour chaque équipe dans le classement', async () => {
    // team.points rendu comme "{points} pts" dans .palmares-rank-points (ligne 1253 PlayerDisplay.jsx)
    setupMocks({
      palmaresEntries: [makePalmaresEntry('SCIENCE', {
        name: 'Sciences & Nature',
        color: '#22c55e',
        teams: [
          { name: 'Rouges', color: [239, 68, 68], points: 30 },
          { name: 'Verts',  color: [34, 197, 94], points: 20 },
        ],
        totalPoints: 50,
      })],
    })
    render(<PlayerDisplay />)
    await waitFor(() => {
      expect(screen.getByText('Rouges')).toBeInTheDocument()
    })
    // Points individuels des équipes
    expect(screen.getByText('30 pts')).toBeInTheDocument()
    expect(screen.getByText('20 pts')).toBeInTheDocument()
    // Header stats : totalTeamPoints = 30+20 = 50
    expect(screen.getByText('👥 50 pts')).toBeInTheDocument()
  })

})
