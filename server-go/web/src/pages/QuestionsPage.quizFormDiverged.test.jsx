/**
 * Tests — QuestionsPage : logique de divergence `quizFormDiverged` (#137
 * Batch 2b T2.5).
 *
 * Contexte (_work/handoff/task-test-writer-t25-20260807-090407.md, réserve
 * QA `_work/reports/qa-20260806-173500.md` §3.1) : le correctif du bug de
 * fraîcheur (T2.5, plan §5bis) — un écart entre le formulaire local NON
 * enregistré de la section Quiz et `gameState.quiz*` doit être rendu visible
 * dans le popup IA — n'avait aucun test. `quizFormDiverged` n'est pas
 * exportée (variable dérivée privée au composant, comme `arraysEqualUnordered`)
 * : elle est donc exercée ici via son seul effet observable — la prop
 * `hasUnsavedQuizChanges` reçue par le vrai `AIGenerateModal` (non mocké),
 * dont le bandeau est vérifié par ailleurs dans
 * AIGenerateModal.unsavedBanner.test.jsx pour son rendu isolé.
 *
 * `AIGenerateModal` appelle `useNavigate()`/`useLocation()` sans garde — il
 * faut un contexte Router pour le monter réellement (contrairement aux
 * autres tests QuestionsPage.*.test.jsx qui n'ouvrent jamais la modale).
 *
 * Suit le pattern de mocks de QuestionsPage.quizChips.test.jsx.
 */
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
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

// NOTE : `gameState` doit être fusionné en DERNIER — sinon un `...overrides`
// final écraserait le merge BASE_QUIZ_STATE + overrides.gameState par le
// `overrides.gameState` brut (sans les défauts), un piège classique d'ordre
// de spread.
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
    <MemoryRouter initialEntries={['/admin/questions']}>
      <QuestionsPage />
    </MemoryRouter>
  )

  return waitFor(() => {
    const openBtn = screen.getByText('✨ Générer via IA')
    expect(openBtn).not.toBeDisabled()
    fireEvent.click(openBtn)
  })
}

describe('QuestionsPage — quizFormDiverged : bandeau "modifications non enregistrées" (#137 Batch 2b T2.5)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('aucune divergence sur les 5 champs (formulaire local == gameState) : pas de bandeau', async () => {
    await renderAndOpenModal()

    expect(screen.queryByText(BANNER_TEXT)).not.toBeInTheDocument()
    // Sanity : le rappel lecture seule affiche bien les valeurs (la modale
    // s'est correctement ouverte, ce n'est pas un faux négatif).
    expect(screen.getByText('Cinéma français des années 80')).toBeInTheDocument()
  })

  it('divergence sur le thème seul (formulaire modifié, non enregistré) : bandeau affiché', async () => {
    await renderAndOpenModal()
    expect(screen.queryByText(BANNER_TEXT)).not.toBeInTheDocument()

    fireEvent.change(document.getElementById('quiz-theme'), { target: { value: 'Un autre thème' } })

    expect(screen.getByText(BANNER_TEXT)).toBeInTheDocument()
  })

  it('divergence sur la langue seule : bandeau affiché', async () => {
    await renderAndOpenModal()

    fireEvent.change(document.getElementById('quiz-language'), { target: { value: 'Anglais' } })

    expect(screen.getByText(BANNER_TEXT)).toBeInTheDocument()
  })

  it('divergence sur l\'objectif seul : bandeau affiché', async () => {
    await renderAndOpenModal()

    fireEvent.change(document.getElementById('quiz-objectives'), { target: { value: 'Nouvel objectif' } })

    expect(screen.getByText(BANNER_TEXT)).toBeInTheDocument()
  })

  it('divergence sur les publics seuls (ajout d\'un chip, non enregistré) : bandeau affiché', async () => {
    await renderAndOpenModal()

    fireEvent.click(screen.getByRole('button', { name: 'Ado (13-17 ans)' }))

    expect(screen.getByText(BANNER_TEXT)).toBeInTheDocument()
  })

  it('divergence sur les difficultés seules (ajout d\'un chip, non enregistré) : bandeau affiché', async () => {
    await renderAndOpenModal()

    fireEvent.click(screen.getByRole('button', { name: 'Difficile' }))

    expect(screen.getByText(BANNER_TEXT)).toBeInTheDocument()
  })

  it('divergence sur QUIZ_NAME uniquement (même formulaire/bouton Enregistrer, n\'alimente pas la génération) : PAS de bandeau', async () => {
    await renderAndOpenModal()

    fireEvent.change(document.getElementById('quiz-name'), { target: { value: 'Nouveau nom' } })

    expect(screen.queryByText(BANNER_TEXT)).not.toBeInTheDocument()
  })

  it('divergence sur QUIZ_NOTES (texte libre) uniquement : PAS de bandeau', async () => {
    await renderAndOpenModal()

    fireEvent.change(document.getElementById('quiz-notes'), { target: { value: 'Nouvelle note' } })

    expect(screen.queryByText(BANNER_TEXT)).not.toBeInTheDocument()
  })

  it('arraysEqualUnordered : mêmes publics, ordre différent — pas traité comme une divergence', async () => {
    // gameState porte 2 publics dans un ordre ; le formulaire local, initialisé
    // depuis gameState, part du même ordre. On le fait diverger en ORDRE
    // seulement : retirer puis réajouter un chip le déplace en fin de tableau
    // (toggleQuizPopulation ajoute en fin de liste), sans changer l'ensemble.
    await renderAndOpenModal({ quizPopulations: ['Junior (6-12 ans)', 'Ado (13-17 ans)'] })
    expect(screen.queryByText(BANNER_TEXT)).not.toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Junior (6-12 ans)' })) // retire
    expect(screen.getByText(BANNER_TEXT)).toBeInTheDocument() // divergence transitoire (1 seul élément local)

    fireEvent.click(screen.getByRole('button', { name: 'Junior (6-12 ans)' })) // réajoute en fin de liste

    // Même contenu (['Ado (13-17 ans)', 'Junior (6-12 ans)'] vs
    // ['Junior (6-12 ans)', 'Ado (13-17 ans)']), ordre différent — pas une
    // divergence grâce à arraysEqualUnordered.
    expect(screen.queryByText(BANNER_TEXT)).not.toBeInTheDocument()
  })
})
