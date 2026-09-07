/**
 * Tests — QuestionsPage : bandeau "modifications non enregistrées" de la
 * modale IA (#137 Batch 2b T2.5), ré-adressé pour #215.
 *
 * Contexte AVANT #215 (_work/handoff/task-test-writer-t25-20260807-090407.md,
 * réserve QA `_work/reports/qa-20260806-173500.md` §3.1) : le formulaire de
 * la section Quiz vivait SUR CETTE MÊME PAGE que la modale de génération IA
 * — `quizFormDiverged` comparait l'état local (non enregistré) du formulaire
 * à `gameState.quiz*`, et la divergence pilotait la prop `hasUnsavedQuizChanges`
 * de `AIGenerateModal` (bandeau d'avertissement).
 *
 * #215 (milestone v9.0.0) déplace ce formulaire vers /admin/backstage
 * (BackstagePage.jsx/QuizMetaForm.jsx) — QuestionsPage ne porte plus AUCUN
 * état local de méta-quiz. Il est donc devenu STRUCTURELLEMENT IMPOSSIBLE
 * d'avoir "des modifications non enregistrées de la section Quiz visibles en
 * même temps que" cette modale (elles vivent sur deux pages distinctes) :
 * `hasUnsavedQuizChanges` retombe sur son défaut (false, jamais transmis).
 * Ce fichier verrouille cette nouvelle invariance plutôt que de simuler une
 * divergence qui ne peut plus se produire depuis cette page.
 *
 * Le rendu isolé du bandeau (prop `hasUnsavedQuizChanges={true}` explicite)
 * reste couvert par AIGenerateModal.unsavedBanner.test.jsx, inchangé.
 * La divergence du formulaire lui-même (chips Publics/Difficultés vs
 * gameState) est désormais testée dans BackstagePage.quizChips.test.jsx.
 *
 * `AIGenerateModal` appelle `useNavigate()`/`useLocation()` sans garde — il
 * faut un contexte Router pour le monter réellement (contrairement aux
 * autres tests QuestionsPage.*.test.jsx qui n'ouvrent jamais la modale).
 * Depuis #215, QuestionsPage elle-même appelle aussi `useNavigate()`/
 * `useSearchParams()` (onglets Questions/Rafale, lien "configurer le quiz" →
 * navigation vers Backstage) — le contexte Router est donc requis pour
 * TOUTE la page, pas seulement pour la modale.
 */
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import QuestionsPage from './QuestionsPage'

// ---------------------------------------------------------------------------
// Mocks
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

vi.mock('./QuestionsPage.css', () => ({}))
vi.mock('./ConfigPage.css', () => ({}))
vi.mock('../components/AIGenerateModal.css', () => ({}))

import { useGame } from '../hooks/GameContext'
import { useCategories } from '../hooks/useCategories'

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const BASE_QUIZ_STATE = {
  quizTheme: 'Cinéma français des années 80',
  quizPopulations: ['Adulte (18-64 ans)'],
  quizDifficulties: ['Moyen'],
  quizLanguage: 'Français',
  quizObjectives: 'Réviser le chapitre 3',
  quizName: '',
  quizNotes: '',
  quizHiddenFields: [],
}

const makeQPageMock = ({ gameState: gameStateOverrides, ...overrides } = {}) => ({
  questions: overrides.questions ?? {},
  fsInfo: { used: 0, total: 100 },
  deleteQuestion: vi.fn(),
  sendMessage: vi.fn(),
  newGame: vi.fn(),
  aiJob: null,
  cancelAiGeneration: vi.fn(),
  ...overrides,
  gameState: {
    phase: 'STOPPED',
    question: null,
    ...BASE_QUIZ_STATE,
    ...gameStateOverrides,
  },
})

const makeCategoriesMock = (overrides = {}) => ({
  categories: overrides.categories ?? [
    { key: 'GEOGRAPHY', name: 'Geographie', imageURL: '', isCustom: false },
  ],
  loading: overrides.loading ?? false,
  error: overrides.error ?? null,
  refetch: overrides.refetch ?? vi.fn(),
})

const BANNER_TEXT = 'Des modifications de la section Quiz ne sont pas enregistrées.'

function renderAndOpenModal(gameStateOverrides = {}) {
  useGame.mockReturnValue(makeQPageMock({ gameState: gameStateOverrides }))
  useCategories.mockReturnValue(makeCategoriesMock())
  global.fetch = vi.fn().mockResolvedValue({
    ok: true,
    json: async () => ({ ai: { provider: 'anthropic', api_key_configured: true } }),
  })

  render(
    <MemoryRouter initialEntries={['/admin/quiz']}>
      <Routes>
        <Route path="/admin/quiz" element={<QuestionsPage />} />
        {/* Sentinelle — vérifie une vraie navigation SPA, pas juste que la
            modale se ferme (le bouton déclencheur "✨ Générer via IA" reste
            dans le DOM tant que QuestionsPage est monté, modale ouverte ou
            non : ce n'est PAS un signal de navigation valide). */}
        <Route path="/admin/backstage" element={<div data-testid="backstage-sentinel">Backstage</div>} />
      </Routes>
    </MemoryRouter>
  )

  return waitFor(() => {
    const openBtn = screen.getByText('✨ Générer via IA')
    expect(openBtn).not.toBeDisabled()
    fireEvent.click(openBtn)
  })
}

describe('QuestionsPage — hasUnsavedQuizChanges : structurellement toujours false depuis #215', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('aucun bandeau au chargement (gameState "à jour" par construction — plus de formulaire local sur cette page)', async () => {
    await renderAndOpenModal()

    expect(screen.queryByText(BANNER_TEXT)).not.toBeInTheDocument()
    // Sanity : le rappel lecture seule affiche bien les valeurs (la modale
    // s'est correctement ouverte, ce n'est pas un faux négatif).
    expect(screen.getByText('Cinéma français des années 80')).toBeInTheDocument()
  })

  it('un changement de gameState (ex. UPDATE_QUIZ_META émis depuis Backstage dans un autre onglet) ne fait toujours apparaître aucun bandeau', async () => {
    await renderAndOpenModal({ quizTheme: 'Un autre thème, déjà enregistré ailleurs' })

    // Le rappel lecture seule suit la nouvelle valeur diffusée...
    expect(screen.getByText('Un autre thème, déjà enregistré ailleurs')).toBeInTheDocument()
    // ...mais aucune notion de "non enregistré" ne peut exister depuis cette
    // page : il n'y a plus de formulaire local à comparer.
    expect(screen.queryByText(BANNER_TEXT)).not.toBeInTheDocument()
  })

  it('le lien "modifier" navigue vers /admin/backstage (plus de scroll — la section Quiz n\'est plus sur cette page)', async () => {
    await renderAndOpenModal()

    fireEvent.click(screen.getByText('modifier'))

    // Route réelle /admin/backstage devenue active (QuestionsPage démonté,
    // sentinelle Backstage affichée) — preuve d'une vraie navigation SPA,
    // pas d'un simple scrollIntoView comme avant #215.
    await waitFor(() => {
      expect(screen.getByTestId('backstage-sentinel')).toBeInTheDocument()
    })
    expect(screen.queryByText('✨ Générer via IA')).not.toBeInTheDocument()
  })
})
