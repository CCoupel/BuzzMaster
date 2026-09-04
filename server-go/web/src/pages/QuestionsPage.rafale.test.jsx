/**
 * Tests for QuestionsPage — sélecteur de mode RAFALE (v8.0.0, #16/#199,
 * bugfix cohérence UI, SHA e7d895a1).
 *
 * Le `<select>` texte de mode RAFALE a été remplacé par un pattern
 * data-driven de cartes radio, réutilisant TEL QUEL le patron MEMORY/
 * MEMOTION existant (.memory-mode-selector/-options/-option, aucune
 * nouvelle classe CSS) — 4 modes (contrat rafale.md §3.4) : SOLO,
 * CHACUN_SON_TOUR, TANT_QUE_JE_GAGNE, MAILLON_FAIBLE.
 *
 * Périmètre de ce fichier : le sélecteur de mode UNIQUEMENT (rendu des 4
 * modes, sélection, valeur par défaut). Le sélecteur de CATÉGORIE voisin
 * (CategorySelector.jsx) a son propre test dédié
 * (components/CategorySelector.test.jsx) — pas dupliqué ici.
 */
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import QuestionsPage from './QuestionsPage'

// ---------------------------------------------------------------------------
// Mocks — même patron que QuestionsPage.v571.test.jsx
// ---------------------------------------------------------------------------

vi.mock('../hooks/GameContext', () => ({
  useGame: vi.fn(),
  GameProvider: ({ children }) => children,
}))

// #215 — QuestionsPage appelle désormais useNavigate()/useSearchParams()
// (onglets Questions/Rafale, lien "configurer le quiz" → navigation vers
// Backstage) : ces tests ne montent aucun Router réel, donc mock plutôt que
// de faire porter <MemoryRouter> à chacun des (nombreux) render() ci-dessous.
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

// RafalePoolAlert fait son propre fetch (GET /api/rafale/pool) — déjà
// couvert isolément par components/RafalePoolAlert.rafale.test.jsx.
// Stub léger ici pour ne pas polluer ces tests (centrés sur le sélecteur
// de mode) avec du fetch asynchrone hors-sujet.
vi.mock('../components/RafalePoolAlert', () => ({
  default: () => <div data-testid="rafale-pool-alert-stub" />,
}))

vi.mock('./QuestionsPage.css', () => ({}))
vi.mock('./ConfigPage.css', () => ({}))

import { useGame } from '../hooks/GameContext'
import { useCategories } from '../hooks/useCategories'

// ---------------------------------------------------------------------------
// Helpers — identiques à QuestionsPage.v571.test.jsx
// ---------------------------------------------------------------------------

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
  categories: overrides.categories ?? [
    { key: 'GEOGRAPHY', name: 'Geographie', imageURL: '', isCustom: false },
  ],
  loading: overrides.loading ?? false,
  error: overrides.error ?? null,
  refetch: overrides.refetch ?? vi.fn(),
})

/** Sélectionne le type RAFALE puis retourne les 4 radios de mode. */
function switchToRafaleAndGetModeRadios(container) {
  // getByRole('button', ...) plutôt que getByText().closest('button') : #215
  // ajoute un onglet "Rafale" (role="tab") sur cette même page — même
  // libellé visible que le bouton de type, texte seul ne suffit plus à
  // distinguer les deux.
  fireEvent.click(screen.getByRole('button', { name: 'Rafale' }))
  return {
    solo: container.querySelector('input[name="rafaleMode"][value="SOLO"]'),
    chacunSonTour: container.querySelector('input[name="rafaleMode"][value="CHACUN_SON_TOUR"]'),
    tantQueJeGagne: container.querySelector('input[name="rafaleMode"][value="TANT_QUE_JE_GAGNE"]'),
    maillonFaible: container.querySelector('input[name="rafaleMode"][value="MAILLON_FAIBLE"]'),
  }
}

describe('QuestionsPage — sélecteur de mode RAFALE (v8.0.0, #199, bugfix cohérence UI)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    useGame.mockReturnValue(makeQPageMock())
    useCategories.mockReturnValue(makeCategoriesMock())
    global.fetch = vi.fn()
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  it('le bouton de type "Rafale" est présent dans le sélecteur de type', () => {
    render(<QuestionsPage />)
    // getByRole('button', ...) : #215 ajoute un onglet "Rafale" (role="tab")
    // sur cette même page — distinct du bouton de type ciblé ici.
    expect(screen.getByRole('button', { name: 'Rafale' })).toBeInTheDocument()
  })

  it('le sélecteur de mode RAFALE (4 radios) n\'est PAS rendu tant que le type RAFALE n\'est pas sélectionné', () => {
    const { container } = render(<QuestionsPage />)
    expect(container.querySelectorAll('input[name="rafaleMode"]').length).toBe(0)
  })

  it('après sélection du type Rafale : les 4 modes du contrat §3.4 sont rendus, exactement 4 radios', () => {
    const { container } = render(<QuestionsPage />)
    const radios = switchToRafaleAndGetModeRadios(container)

    expect(container.querySelectorAll('input[name="rafaleMode"]').length).toBe(4)
    expect(radios.solo).not.toBeNull()
    expect(radios.chacunSonTour).not.toBeNull()
    expect(radios.tantQueJeGagne).not.toBeNull()
    expect(radios.maillonFaible).not.toBeNull()
  })

  it('SOLO est sélectionné par défaut (formData.rafaleMode initial)', () => {
    const { container } = render(<QuestionsPage />)
    const radios = switchToRafaleAndGetModeRadios(container)

    expect(radios.solo.checked).toBe(true)
    expect(radios.chacunSonTour.checked).toBe(false)
    expect(radios.tantQueJeGagne.checked).toBe(false)
    expect(radios.maillonFaible.checked).toBe(false)
  })

  it('cliquer sur CHACUN_SON_TOUR le sélectionne et désélectionne SOLO', () => {
    const { container } = render(<QuestionsPage />)
    const radios = switchToRafaleAndGetModeRadios(container)

    fireEvent.click(radios.chacunSonTour)

    expect(radios.chacunSonTour.checked).toBe(true)
    expect(radios.solo.checked).toBe(false)
  })

  it('cliquer sur TANT_QUE_JE_GAGNE le sélectionne (un seul mode actif à la fois)', () => {
    const { container } = render(<QuestionsPage />)
    const radios = switchToRafaleAndGetModeRadios(container)

    fireEvent.click(radios.tantQueJeGagne)

    expect(radios.tantQueJeGagne.checked).toBe(true)
    expect(radios.solo.checked).toBe(false)
    expect(radios.chacunSonTour.checked).toBe(false)
    expect(radios.maillonFaible.checked).toBe(false)
  })

  it('cliquer sur MAILLON_FAIBLE le sélectionne', () => {
    const { container } = render(<QuestionsPage />)
    const radios = switchToRafaleAndGetModeRadios(container)

    fireEvent.click(radios.maillonFaible)

    expect(radios.maillonFaible.checked).toBe(true)
  })

  it('changer de mode puis revenir sur SOLO le re-sélectionne', () => {
    const { container } = render(<QuestionsPage />)
    const radios = switchToRafaleAndGetModeRadios(container)

    fireEvent.click(radios.maillonFaible)
    expect(radios.solo.checked).toBe(false)

    fireEvent.click(radios.solo)
    expect(radios.solo.checked).toBe(true)
    expect(radios.maillonFaible.checked).toBe(false)
  })

  it('chaque option affiche un libellé et une description distincts (pas de doublon de texte)', () => {
    const { container } = render(<QuestionsPage />)
    switchToRafaleAndGetModeRadios(container)

    const labels = Array.from(container.querySelectorAll('.memory-mode-option .memory-mode-label strong')).map(el => el.textContent)
    expect(labels).toEqual(['SOLO', 'CHACUN SON TOUR', 'TANT QUE JE GAGNE', 'MAILLON FAIBLE'])

    const descriptions = Array.from(container.querySelectorAll('.memory-mode-option .memory-mode-label small')).map(el => el.textContent)
    expect(new Set(descriptions).size).toBe(4) // 4 descriptions toutes différentes
  })

  it('le sélecteur de mode réutilise les classes .memory-mode-* existantes (MEMORY/MEMOTION), aucune classe nouvelle', () => {
    const { container } = render(<QuestionsPage />)
    switchToRafaleAndGetModeRadios(container)

    expect(container.querySelector('.memory-mode-selector')).not.toBeNull()
    expect(container.querySelectorAll('.memory-mode-option').length).toBe(4)
  })

  it('changer de type après avoir choisi un mode RAFALE puis revenir sur Rafale : le mode choisi est conservé (état du formulaire, pas remis à SOLO)', () => {
    const { container } = render(<QuestionsPage />)
    let radios = switchToRafaleAndGetModeRadios(container)
    fireEvent.click(radios.maillonFaible)

    // Bascule sur un autre type puis retour sur Rafale.
    fireEvent.click(screen.getByText(/qcm/i).closest('button'))
    fireEvent.click(screen.getByRole('button', { name: 'Rafale' }))

    radios = {
      solo: container.querySelector('input[name="rafaleMode"][value="SOLO"]'),
      maillonFaible: container.querySelector('input[name="rafaleMode"][value="MAILLON_FAIBLE"]'),
    }
    expect(radios.maillonFaible.checked).toBe(true)
    expect(radios.solo.checked).toBe(false)
  })
})

// ---------------------------------------------------------------------------
// ⚠️ [CHANGED] #216 (milestone v9.0.0, Lot 2, réouverture ASSUMÉE de #107,
// décision utilisateur explicite — contracts/rafale.md §3.3) — RAFALE est
// retiré du CategorySelector générique (guard `formData.type !== 'RAFALE'`
// dans QuestionsPage.jsx) : il a de nouveau besoin de PLUSIEURS catégories
// ET plusieurs difficultés (non exclusives), plus un barème par étoile en
// repli sur POINTS générique. Le bloc precedent ("sélecteur de catégorie
// unique, bugfix 2026-08-29") affirmait exactement le comportement
// opposé — réécrit ci-dessous, pas supprimé, avec le nouveau sélecteur à
// chips (motif éprouvé RafaleAIGenerateModal.jsx, classe .rafale-multi-chip,
// pas .category-btn/.category-selector).
// ---------------------------------------------------------------------------

describe('QuestionsPage — RAFALE : sélecteur multi-chips catégories/difficultés (#216)', () => {
  const TWO_CATEGORIES = [
    { key: 'HISTORY', name: 'Histoire', imageURL: '', isCustom: false },
    { key: 'GEOGRAPHY', name: 'Geographie', imageURL: '', isCustom: false },
  ]

  beforeEach(() => {
    vi.clearAllMocks()
    useGame.mockReturnValue(makeQPageMock())
    useCategories.mockReturnValue(makeCategoriesMock({ categories: TWO_CATEGORIES }))
    global.fetch = vi.fn()
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  it('le CategorySelector générique n\'est PLUS rendu pour RAFALE (retiré au profit du sélecteur multi-chips dédié)', () => {
    const { container } = render(<QuestionsPage />)
    switchToRafaleAndGetModeRadios(container)

    expect(container.querySelector('.category-selector')).toBeNull()
    expect(container.querySelectorAll('.rafale-multi-chip').length).toBeGreaterThan(0)
  })

  it('plusieurs catégories peuvent être sélectionnées simultanément (non exclusives)', () => {
    render(<QuestionsPage />)
    switchToRafaleAndGetModeRadios(document.body)

    const histoire = screen.getByRole('button', { name: /Histoire/ })
    const geographie = screen.getByRole('button', { name: /Geographie/ })
    fireEvent.click(histoire)
    fireEvent.click(geographie)

    expect(histoire.className).toMatch(/\bactive\b/)
    expect(geographie.className).toMatch(/\bactive\b/)
  })

  it('re-cliquer sur une catégorie déjà active la désélectionne, sans affecter l\'autre (toggle indépendant)', () => {
    render(<QuestionsPage />)
    switchToRafaleAndGetModeRadios(document.body)

    const histoire = screen.getByRole('button', { name: /Histoire/ })
    const geographie = screen.getByRole('button', { name: /Geographie/ })
    fireEvent.click(histoire)
    fireEvent.click(geographie)
    fireEvent.click(histoire)

    expect(histoire.className).not.toMatch(/\bactive\b/)
    expect(geographie.className).toMatch(/\bactive\b/)
  })

  it('plusieurs difficultés peuvent être sélectionnées simultanément (non exclusives, contrairement à MEMOTION)', () => {
    const { container } = render(<QuestionsPage />)
    switchToRafaleAndGetModeRadios(container)

    const diffChips = container.querySelectorAll('.rafale-multi-chip-row')[1].querySelectorAll('.rafale-multi-chip')
    fireEvent.click(diffChips[0]) // ★
    fireEvent.click(diffChips[2]) // ★★★

    expect(diffChips[0].className).toMatch(/\bactive\b/)
    expect(diffChips[1].className).not.toMatch(/\bactive\b/)
    expect(diffChips[2].className).toMatch(/\bactive\b/)
  })

  it('sélection persistée après un aller-retour vers un autre type (RAFALE -> QCM -> RAFALE)', () => {
    render(<QuestionsPage />)
    switchToRafaleAndGetModeRadios(document.body)
    fireEvent.click(screen.getByRole('button', { name: /Histoire/ }))

    fireEvent.click(screen.getByText(/qcm/i).closest('button'))
    fireEvent.click(screen.getByRole('button', { name: 'Rafale' }))

    expect(screen.getByRole('button', { name: /Histoire/ }).className).toMatch(/\bactive\b/)
  })

  it('éditeur de barème par difficulté : n\'apparaît qu\'après sélection d\'au moins une difficulté, un champ par difficulté sélectionnée', () => {
    const { container } = render(<QuestionsPage />)
    switchToRafaleAndGetModeRadios(container)

    expect(container.querySelector('.rafale-points-by-difficulty-row')).toBeNull()

    const diffChips = container.querySelectorAll('.rafale-multi-chip-row')[1].querySelectorAll('.rafale-multi-chip')
    fireEvent.click(diffChips[0]) // ★
    fireEvent.click(diffChips[1]) // ★★

    const baremeRow = container.querySelector('.rafale-points-by-difficulty-row')
    expect(baremeRow).not.toBeNull()
    expect(baremeRow.querySelectorAll('input[type="number"]').length).toBe(2)
  })

  it('saisie libre du barème : la valeur saisie pour une difficulté est conservée dans le champ correspondant', () => {
    const { container } = render(<QuestionsPage />)
    switchToRafaleAndGetModeRadios(container)

    const diffChips = container.querySelectorAll('.rafale-multi-chip-row')[1].querySelectorAll('.rafale-multi-chip')
    fireEvent.click(diffChips[1]) // ★★

    const input = container.querySelector('#rafale-points-diff-2')
    expect(input).not.toBeNull()
    fireEvent.change(input, { target: { value: '15' } })
    expect(input.value).toBe('15')
  })

  it('validation au submit : au moins une catégorie ET une difficulté requises, sinon aucun POST envoyé (défense en profondeur, réouverture code-review-20260829-163049.md)', () => {
    render(<QuestionsPage />)
    switchToRafaleAndGetModeRadios(document.body)
    // Ni catégorie ni difficulté sélectionnée — tente d'enregistrer.
    const saveBtn = screen.queryByText(/enregistrer|ajouter/i)
    if (saveBtn) fireEvent.click(saveBtn)

    expect(global.fetch).not.toHaveBeenCalledWith('/questions', expect.anything())
  })
})
