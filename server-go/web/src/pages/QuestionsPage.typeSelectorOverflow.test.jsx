/**
 * Tests for QuestionsPage — Lot B / C4 (retour QUALIF v9.0.0.4, plan-
 * v900-correctifs-qualif-20260906-104500.md §5) : débordement du sélecteur
 * de type de question.
 *
 * #214 a ajouté ENTRACTE (7e type) sans toucher au sélecteur, écrit pour
 * 5 types (regroupement figé `[QUESTION_TYPES.slice(0, 3),
 * QUESTION_TYPES.slice(3)]`, `QuestionsPage.jsx:1663`) — la seconde rangée
 * porte désormais 4 boutons `flex: 1` sans `flex-wrap`, débordant en
 * largeur réduite (Lot B). Le sélecteur de type de CARTE MEMOTION
 * (`:2246`, types nestable uniquement) n'a lui jamais été scindé en rangées
 * figées, mais partage la même règle CSS `.type-filter-row`/`.type-btn` —
 * RAFALE devenu nestable (#217) l'a fait passer de 3 à 4 boutons, même
 * symptôme (C4). Les deux se corrigent par la MÊME règle CSS
 * (`flex-wrap: wrap` + `min-width: 0`), à poser une seule fois.
 *
 * Ce fichier couvre :
 * - Lot B (JS) : le sélecteur de type de QUESTION rend tous les types dans
 *   UNE SEULE rangée (plus de découpage figé par tranche) — construit
 *   contre `QUESTION_TYPES` (utils/questionTypeMeta.js, source unique), pas
 *   contre un compte codé en dur : un 8e type futur ne doit jamais faire
 *   récidiver ce défaut silencieusement.
 * - C4 (CSS) : `.type-filter-row` accepte le retour à la ligne et
 *   `.type-btn` ne force plus une largeur minimale qui l'en empêche —
 *   vérifié par lecture directe du texte source de QuestionsPage.css (même
 *   technique que GamePage.ardoise-grid.test.jsx), complémentaire à la
 *   vérification visuelle prévue en procédure manuelle (largeur réduite,
 *   7 types, aucune coupure).
 */
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import QuestionsPage from './QuestionsPage'
import { QUESTION_TYPES } from '../utils/questionTypeMeta'

// ---------------------------------------------------------------------------
// Mocks — même patron que QuestionsPage.rafale.test.jsx / .v571.test.jsx
// ---------------------------------------------------------------------------

vi.mock('../hooks/GameContext', () => ({
  useGame: vi.fn(),
  GameProvider: ({ children }) => children,
}))

vi.mock('react-router-dom', () => ({
  useNavigate: () => vi.fn(),
  useSearchParams: () => [new URLSearchParams(), vi.fn()],
}))

vi.mock('../hooks/useCategoryFilter', () => ({
  useCategoryFilter: vi.fn((questions) => ({
    selectedCategories: new Set(),
    availableCategories: [],
    filteredQuestions: questions || [],
    toggleCategoryFilter: vi.fn(),
    clearCategoryFilters: vi.fn(),
  })),
}))

vi.mock('../hooks/useCategories', () => ({
  useCategories: vi.fn(),
}))

vi.mock('../components/Button', () => ({
  default: ({ children, onClick, disabled, type, ...rest }) => (
    <button onClick={onClick} disabled={disabled} type={type || 'button'} {...rest}>
      {children}
    </button>
  ),
}))

vi.mock('../components/Card', () => ({
  default: ({ children, className, padding, variant, ...rest }) => (
    <div className={className} {...rest}>{children}</div>
  ),
  CardHeader: ({ children }) => <div className="card-header">{children}</div>,
  CardBody: ({ children }) => <div className="card-body">{children}</div>,
}))

vi.mock('../components/CategoryBalance', () => ({
  default: () => null,
}))

vi.mock('../components/QuestionCard', () => ({
  default: ({ question, onClick }) => (
    <div data-testid={`qcard-${question.ID}`} onClick={() => onClick && onClick(question)} />
  ),
  CATEGORIES: {
    GEOGRAPHY: { label: 'Geographie', icon: '🌍', color: '#3b82f6' },
  },
}))

vi.mock('../components/RafalePoolAlert', () => ({
  default: () => <div data-testid="rafale-pool-alert-stub" />,
}))

vi.mock('./QuestionsPage.css', () => ({}))
vi.mock('./ConfigPage.css', () => ({}))

import { useGame } from '../hooks/GameContext'
import { useCategories } from '../hooks/useCategories'

const makeQPageMock = (overrides = {}) => ({
  questions: overrides.questions ?? {},
  fsInfo: { used: 0, total: 100 },
  deleteQuestion: vi.fn(),
  sendMessage: vi.fn(),
  gameState: {
    phase: 'STOPPED',
    question: null,
    ...overrides.gameState,
  },
  newGame: vi.fn(),
  ...overrides,
})

const makeCategoriesMock = (overrides = {}) => ({
  categories: overrides.categories ?? [],
  loading: overrides.loading ?? false,
  error: overrides.error ?? null,
  refetch: overrides.refetch ?? vi.fn(),
})

describe('QuestionsPage — sélecteur de type de QUESTION, une seule rangée pour les 7 types (Lot B, retour QUALIF v9.0.0.4)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    useGame.mockReturnValue(makeQPageMock())
    useCategories.mockReturnValue(makeCategoriesMock())
    global.fetch = vi.fn()
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  it('QUESTION_TYPES compte bien 7 entrées à ce jour (garde de contexte — si ce nombre change, les assertions ci-dessous restent valables sans modification)', () => {
    expect(QUESTION_TYPES.length).toBe(7)
  })

  it('rend un bouton .type-btn pour CHAQUE entrée de QUESTION_TYPES, dans l\'ordre, sans aucun être omis ni dupliqué', () => {
    const { container } = render(<QuestionsPage />)
    const grid = container.querySelector('.type-filter-grid')
    expect(grid).not.toBeNull()
    const buttons = Array.from(grid.querySelectorAll('.type-btn'))
    expect(buttons.map(b => b.textContent.trim())).toEqual(QUESTION_TYPES.map(t => t.label))
  })

  it('les boutons de type de question vivent tous dans une SEULE .type-filter-row — plus de découpage figé par tranche (#214 ajoutait ENTRACTE à un sélecteur écrit pour 5 types)', () => {
    const { container } = render(<QuestionsPage />)
    const grid = container.querySelector('.type-filter-grid')
    const rows = grid.querySelectorAll('.type-filter-row')
    expect(rows.length).toBe(1)
    expect(rows[0].querySelectorAll('.type-btn').length).toBe(QUESTION_TYPES.length)
  })

  it('cliquer sur un type en fin de liste (ENTRACTE) le sélectionne — non-régression : rester sur une rangée unique n\'empêche pas la sélection', () => {
    const { container } = render(<QuestionsPage />)
    const entracteBtn = Array.from(container.querySelectorAll('.type-filter-grid .type-btn'))
      .find(b => b.textContent.trim() === 'Entracte')
    expect(entracteBtn).not.toBeUndefined()
    fireEvent.click(entracteBtn)
    expect(entracteBtn.className).toContain('active')
  })
})

describe('QuestionsPage.css — .type-filter-row / .type-btn acceptent le retour à la ligne (C4, retour QUALIF v9.0.0.4)', () => {
  const cssPath = path.join(path.dirname(fileURLToPath(import.meta.url)), 'QuestionsPage.css')
  const cssSource = fs.readFileSync(cssPath, 'utf-8')

  // Extraction par sélecteur EXACT (pas de section à isoler ici :
  // `.type-filter-row`/`.type-btn` n'ont chacun qu'une seule déclaration de
  // base dans QuestionsPage.css — vérifié par grep avant écriture de ce
  // test — les nombreuses règles `.type-btn.<variant>`/`.type-btn:hover`
  // etc. ne matchent pas grâce à l'exigence d'espace/accolade immédiate).
  function ruleBody(selector) {
    const escaped = selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
    const match = cssSource.match(new RegExp(escaped + '\\s*\\{([^}]*)\\}'))
    return match ? match[1] : null
  }

  it('.type-filter-row : flex-wrap: wrap (le sélecteur de type — question ET carte MEMOTION — peut revenir à la ligne)', () => {
    const rule = ruleBody('.type-filter-row')
    expect(rule).not.toBeNull()
    expect(rule).toMatch(/flex-wrap:\s*wrap/)
  })

  it('.type-btn : min-width: 0 (un flex-basis figé empêcherait le wrap de .type-filter-row de reprendre de la place)', () => {
    const rule = ruleBody('.type-btn')
    expect(rule).not.toBeNull()
    expect(rule).toMatch(/min-width:\s*0/)
  })

  it('.type-btn : reste flexible (flex: 1 ... auto), ne revient pas à une largeur fixe qui romprait la grille existante', () => {
    const rule = ruleBody('.type-btn')
    expect(rule).toMatch(/flex:\s*1/)
  })
})
