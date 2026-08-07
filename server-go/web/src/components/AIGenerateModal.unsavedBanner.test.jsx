/**
 * Tests — AIGenerateModal : bandeau "modifications non enregistrées" piloté
 * par `hasUnsavedQuizChanges` (#137 Batch 2b T2.5).
 *
 * Contexte (_work/handoff/task-test-writer-t25-20260807-090407.md) :
 * `hasUnsavedQuizChanges` n'apparaissait que comme valeur de fixture par
 * défaut (`false`) dans AIGenerateModal.test.jsx — jamais comme assertion
 * sur le rendu du bandeau lui-même. Ce fichier teste la prop en isolation :
 * rendu conditionnel du bandeau, et non-blocage du bouton "✨ Générer" (T2.5
 * rend l'écart VISIBLE, il ne l'INTERDIT jamais — contrairement au cas
 * "publics/difficultés vides" qui, lui, désactive le bouton).
 *
 * La logique de calcul de `hasUnsavedQuizChanges` (quizFormDiverged,
 * QuestionsPage.jsx) est testée séparément dans
 * QuestionsPage.quizFormDiverged.test.jsx — ce fichier-ci ne teste que l'effet
 * de la prop sur le composant qui la reçoit.
 *
 * Suit le pattern de mocks/helpers de AIGenerateModal.test.jsx.
 */
import { describe, it, expect, afterEach, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import AIGenerateModal from './AIGenerateModal'

const CATEGORIES = [
  { key: 'ENTERTAINMENT', name: 'Divertissement', color: '#ec4899' },
]

const BANNER_TEXT = 'Des modifications de la section Quiz ne sont pas enregistrées.'
const BANNER_DETAIL = 'La génération utilisera les valeurs ci-dessous.'

function defaultModalProps(overrides = {}) {
  return {
    onClose: vi.fn(),
    apiKeyConfigured: true,
    provider: 'anthropic',
    categories: CATEGORIES,
    quizTheme: 'Cinéma français des années 80',
    quizPopulations: ['Adulte (18-64 ans)'],
    quizDifficulties: ['Moyen'],
    quizLanguage: 'Français',
    quizObjectives: '',
    hasUnsavedQuizChanges: false,
    questions: {},
    aiJob: null,
    onCancelGeneration: vi.fn(),
    onGenerated: vi.fn(),
    onNavigateToQuizSettings: vi.fn(),
    ...overrides,
  }
}

function renderModal(overrides = {}) {
  const props = defaultModalProps(overrides)
  render(
    <MemoryRouter initialEntries={['/admin/questions']}>
      <Routes>
        <Route path="*" element={<AIGenerateModal {...props} />} />
      </Routes>
    </MemoryRouter>
  )
  return props
}

function selectFirstCategory() {
  fireEvent.click(screen.getByText('Divertissement'))
}

describe('AIGenerateModal — bandeau "modifications non enregistrées" (#137 Batch 2b T2.5)', () => {
  afterEach(() => {
    vi.clearAllMocks()
    vi.unstubAllGlobals()
  })

  it('hasUnsavedQuizChanges=true : le bandeau est affiché avec le texte d\'avertissement attendu', () => {
    renderModal({ hasUnsavedQuizChanges: true })

    const banner = screen.getByText(BANNER_TEXT)
    expect(banner).toBeInTheDocument()
    expect(screen.getByText(BANNER_DETAIL)).toBeInTheDocument()
    // role="alert" — le bandeau doit être annoncé (accessibilité), pas un
    // simple texte perdu dans le reste du rappel.
    expect(banner.closest('[role="alert"]')).not.toBeNull()
  })

  it('hasUnsavedQuizChanges=false (défaut) : le bandeau est absent', () => {
    renderModal()

    expect(screen.queryByText(BANNER_TEXT)).not.toBeInTheDocument()
  })

  it('le bouton "✨ Générer" reste cliquable (non désactivé) même bandeau affiché, dès que le reste du formulaire est valide', () => {
    renderModal({ hasUnsavedQuizChanges: true })
    selectFirstCategory()

    expect(screen.getByText(BANNER_TEXT)).toBeInTheDocument()
    expect(screen.getByText('✨ Générer').closest('button')).not.toBeDisabled()
  })

  it('contrairement au bandeau, des publics/difficultés vides désactivent bien le bouton — le non-blocage de T2.5 ne masque pas les autres règles', () => {
    renderModal({ hasUnsavedQuizChanges: true, quizPopulations: [] })
    selectFirstCategory()

    expect(screen.getByText(BANNER_TEXT)).toBeInTheDocument()
    expect(screen.getByText('✨ Générer').closest('button')).toBeDisabled()
  })

  it('cliquer "✨ Générer" avec le bandeau affiché déclenche réellement la génération (pas seulement "non disabled" en apparence)', () => {
    const fetchMock = vi.fn(() => new Promise(() => {})) // never resolves — on inspecte seulement l'appel
    vi.stubGlobal('fetch', fetchMock)
    renderModal({ hasUnsavedQuizChanges: true })
    selectFirstCategory()

    fireEvent.click(screen.getByText('✨ Générer'))

    expect(fetchMock).toHaveBeenCalledWith('/api/generate-questions', expect.any(Object))
  })
})
