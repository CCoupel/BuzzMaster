import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, fireEvent, within } from '@testing-library/react'
import QuestionsPage from './QuestionsPage'

// ---------------------------------------------------------------------------
// QuestionsPage — section Entracte (#119, delta C1/C4 du plan
// _work/reports/plan-entracte-119-fixes-20260820-155123.md).
//
// C1-F2 : la configuration du panneau ENTRACTE quitte ConfigPage pour
// s'installer dans la page Quiz (`quiz-meta-section`), sur le modèle de
// `handleSaveQuizMeta` — formulaire propre, propre bouton d'enregistrement,
// action dédiée `UPDATE_ENTRACTE_CONFIG` (payload UPPER_SNAKE : TITLE,
// SUBTITLE, PANEL_SIZE, ANIM_PERIOD, ANIM_INTENSITY, TRANSITION_MS — plan
// C1-B3).
//
// C4 : le formulaire lit TOUJOURS `gameState.entracteConfigSaved`
// (← GameState.ENTRACTE_CONFIG_SAVED, admin-only), JAMAIS
// `gameState.entracteConfig` (le diffusé, gelé pendant une pause) — sans
// quoi un enregistrement fait pendant l'entracte semblerait perdu au retour
// sur cette page. Une mention « Prendra effet au prochain entracte »
// s'affiche quand `gameState.entracte` est actif.
//
// Suit le pattern de mocks de QuestionsPage.quizChips.test.jsx (#137 Batch
// 2b) — même jeu de mocks minimal éprouvé pour monter QuestionsPage.
// ---------------------------------------------------------------------------

vi.mock('../hooks/GameContext', () => ({
  useGame: vi.fn(),
  GameProvider: ({ children }) => children,
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
  default: ({ children, onClick, disabled, type, loading, ...rest }) => (
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

vi.mock('../components/AIGenerateModal', () => ({
  default: () => null,
}))

vi.mock('./QuestionsPage.css', () => ({}))
vi.mock('./ConfigPage.css', () => ({}))

import { useGame } from '../hooks/GameContext'
import { useCategories } from '../hooks/useCategories'

const DEFAULT_ENTRACTE_SAVED = {
  TITLE: 'ENTRACTE',
  SUBTITLE: 'Retour dans 20mn',
  IMAGE_IS_CUSTOM: false,
  PANEL_SIZE: 65,
  ANIM_PERIOD: 10,
  ANIM_INTENSITY: 20,
  TRANSITION_MS: 2000,
}

const makeQPageMock = (overrides = {}) => ({
  questions: overrides.questions ?? {},
  fsInfo: { used: 0, total: 100 },
  deleteQuestion: vi.fn(),
  sendMessage: vi.fn(),
  gameState: {
    phase: 'STOPPED',
    question: null,
    quizPopulations: [],
    quizDifficulties: [],
    quizHiddenFields: [],
    entracte: false,
    entracteConfigSaved: DEFAULT_ENTRACTE_SAVED,
    // Le diffusé délibérément DIFFÉRENT du enregistré dans les tests ci-
    // dessous qui l'exercent — s'il fuitait dans le formulaire, ces tests le
    // détecteraient (contrat C4 : le formulaire ne doit JAMAIS s'y alimenter).
    entracteConfig: { ...DEFAULT_ENTRACTE_SAVED, TITLE: 'NE DOIT JAMAIS APPARAÎTRE DANS LE FORMULAIRE' },
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

function getEntracteSection() {
  const heading = screen.getByText(/entracte/i, { selector: '.config-section-title, h3, h4' })
  return heading.closest('.config-section, .new-game-bg-section') || heading.closest('div')
}

beforeEach(() => {
  vi.clearAllMocks()
  useCategories.mockReturnValue(makeCategoriesMock())
  global.fetch = vi.fn().mockResolvedValue({ ok: false })
})

describe('QuestionsPage — section Entracte : alimentation depuis ENTRACTE_CONFIG_SAVED (C4)', () => {
  it("le formulaire lit gameState.entracteConfigSaved, JAMAIS gameState.entracteConfig (le diffusé)", () => {
    useGame.mockReturnValue(makeQPageMock({
      gameState: {
        entracteConfigSaved: { ...DEFAULT_ENTRACTE_SAVED, TITLE: 'Valeur enregistrée' },
        entracteConfig: { ...DEFAULT_ENTRACTE_SAVED, TITLE: 'Valeur diffusée (gelée)' },
      },
    }))
    render(<QuestionsPage />)
    const section = getEntracteSection()

    expect(within(section).getByDisplayValue('Valeur enregistrée')).toBeInTheDocument()
    expect(within(section).queryByDisplayValue('Valeur diffusée (gelée)')).toBeNull()
  })
})

describe('QuestionsPage — section Entracte : sauvegarde émet UPDATE_ENTRACTE_CONFIG (C1-B3)', () => {
  it('Enregistrer envoie UPDATE_ENTRACTE_CONFIG avec les champs UPPER_SNAKE du contrat', () => {
    const sendMessage = vi.fn()
    useGame.mockReturnValue(makeQPageMock({ sendMessage }))
    render(<QuestionsPage />)
    const section = getEntracteSection()

    fireEvent.change(within(section).getByDisplayValue('ENTRACTE'), { target: { value: 'Pause déjeuner' } })
    fireEvent.click(within(section).getByText('Enregistrer'))

    expect(sendMessage).toHaveBeenCalledWith('UPDATE_ENTRACTE_CONFIG', expect.objectContaining({
      TITLE: 'Pause déjeuner',
    }))
    const call = sendMessage.mock.calls.find(([action]) => action === 'UPDATE_ENTRACTE_CONFIG')
    const payload = call[1]
    // Les 6 champs du contrat doivent être présents dans le payload envoyé.
    for (const key of ['TITLE', 'SUBTITLE', 'PANEL_SIZE', 'ANIM_PERIOD', 'ANIM_INTENSITY', 'TRANSITION_MS']) {
      expect(payload).toHaveProperty(key)
    }
  })

  it("l'enregistrement réussit et n'est PAS bloqué même si un entracte est actif (accepté à l'enregistrement, C4)", () => {
    const sendMessage = vi.fn()
    useGame.mockReturnValue(makeQPageMock({ sendMessage, gameState: { entracte: true } }))
    render(<QuestionsPage />)
    const section = getEntracteSection()

    fireEvent.click(within(section).getByText('Enregistrer'))

    expect(sendMessage).toHaveBeenCalledWith('UPDATE_ENTRACTE_CONFIG', expect.anything())
  })
})

describe('QuestionsPage — section Entracte : mention "Prendra effet au prochain entracte" (C4-F2)', () => {
  it("n'affiche AUCUNE mention hors entracte", () => {
    useGame.mockReturnValue(makeQPageMock({ gameState: { entracte: false } }))
    render(<QuestionsPage />)
    expect(screen.queryByText(/prendra effet au prochain entracte/i)).toBeNull()
  })

  it('affiche la mention "Prendra effet au prochain entracte" pendant qu\'un entracte est actif', () => {
    useGame.mockReturnValue(makeQPageMock({ gameState: { entracte: true } }))
    render(<QuestionsPage />)
    expect(screen.getByText(/prendra effet au prochain entracte/i)).toBeInTheDocument()
  })
})

describe('QuestionsPage — section Entracte : animation ANIM_INTENSITY=0 (non-régression)', () => {
  it('affiche "animation desactivee" à intensité 0 (repris tel quel de la section retirée de ConfigPage)', () => {
    useGame.mockReturnValue(makeQPageMock({
      gameState: { entracteConfigSaved: { ...DEFAULT_ENTRACTE_SAVED, ANIM_INTENSITY: 0 } },
    }))
    render(<QuestionsPage />)
    const section = getEntracteSection()
    expect(within(section).getByText(/animation d.sactiv.e/i)).toBeInTheDocument()
  })
})

describe('QuestionsPage — section Entracte : image de fond (endpoint renommé, C1-B6)', () => {
  it("l'upload envoie un POST multipart vers /api/game/entracte-image (PAS l'ancien /api/config/entracte-image)", () => {
    useGame.mockReturnValue(makeQPageMock())
    global.fetch = vi.fn().mockResolvedValue({ ok: true, json: async () => ({ image_is_custom: true }) })
    render(<QuestionsPage />)
    const section = getEntracteSection()

    const fileInput = section.querySelector('input[type="file"]')
    expect(fileInput).not.toBeNull()
    const file = new File(['fake-image'], 'fond.jpg', { type: 'image/jpeg' })
    fireEvent.change(fileInput, { target: { files: [file] } })
    fireEvent.click(within(section).getByText(/enregistrer l.image/i))

    const uploadCall = global.fetch.mock.calls.find(([url]) => typeof url === 'string' && url.includes('entracte-image'))
    expect(uploadCall).toBeDefined()
    expect(uploadCall[0]).toBe('/api/game/entracte-image')
    expect(uploadCall[0]).not.toContain('/api/config/')
  })
})
