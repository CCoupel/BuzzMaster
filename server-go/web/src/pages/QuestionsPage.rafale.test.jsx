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
  fireEvent.click(screen.getByText('Rafale').closest('button'))
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
    expect(screen.getByText('Rafale')).toBeInTheDocument()
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
    fireEvent.click(screen.getByText('Rafale').closest('button'))

    radios = {
      solo: container.querySelector('input[name="rafaleMode"][value="SOLO"]'),
      maillonFaible: container.querySelector('input[name="rafaleMode"][value="MAILLON_FAIBLE"]'),
    }
    expect(radios.maillonFaible.checked).toBe(true)
    expect(radios.solo.checked).toBe(false)
  })
})
