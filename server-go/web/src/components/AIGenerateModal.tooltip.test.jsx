/**
 * Tests — AIGenerateModal : tooltip explicatif sur le bouton "✨ Générer"
 * désactivé (bugfix/config-api-key-help, tâche #8, commit adfd576).
 *
 * Contexte (_work/handoff/dev-frontend-20260809-105500.md) : `canSubmit`
 * dépend de 6 conditions indépendantes (thème, publics, difficultés,
 * catégories cibles, au moins un type actif, volume valide) et le bouton
 * n'avait jusqu'ici AUCUNE explication au survol quand désactivé —
 * contrairement au bouton "✨ Générer via IA" de QuestionsPage (une seule
 * cause possible, déjà pourvu d'un `title`, hors scope ici). `submitDisabledTitle`
 * liste précisément la/les condition(s) manquante(s) via l'attribut `title`
 * natif du bouton.
 *
 * AIGenerateModal.test.jsx couvre déjà `disabled`/`not.toBeDisabled()` pour
 * 5 des 6 conditions (thème, publics, difficultés, catégories, type) — ce
 * fichier NE duplique PAS ces assertions `disabled`, il teste uniquement le
 * contenu de l'attribut `title` qui les accompagne désormais, plus la
 * combinaison de plusieurs raisons.
 *
 * Volume (6e condition) : `volumeCount`/`volumeDuration` sont bornés côté UI
 * par `clampInt(raw, 1, 200, ...)` / `clampInt(raw, 5, 240, ...)` — aucune
 * valeur ≤ 0 n'est atteignable via les champs exposés (Number.isNaN d'une
 * chaîne vide retombe même sur le fallback courant, jamais 0). `volumeValid`
 * est donc une garde défensive non atteignable par l'UI ; comme le reste de
 * cette base de tests n'exerce jamais les fonctions internes non exportées
 * (cf. commentaire en tête de AIGenerateModal.test.jsx), ce cas n'est pas
 * testé ici — noté explicitement plutôt que silencieusement omis.
 *
 * Suit le pattern de mocks/helpers de AIGenerateModal.test.jsx /
 * .unsavedBanner.test.jsx.
 */
import { describe, it, expect, afterEach, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import AIGenerateModal from './AIGenerateModal'

const CATEGORIES = [
  { key: 'ENTERTAINMENT', name: 'Divertissement', color: '#ec4899' },
  { key: 'SCIENCE', name: 'Sciences & Nature', color: '#22c55e' },
]

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
  return { props }
}

function selectFirstCategory() {
  fireEvent.click(screen.getByText('Divertissement'))
}

function generateButton() {
  return screen.getByText('✨ Générer').closest('button')
}

describe('AIGenerateModal — tooltip du bouton "✨ Générer" désactivé (bugfix/config-api-key-help #8)', () => {
  afterEach(() => vi.clearAllMocks())

  it('has no title attribute when the form is valid (canSubmit true, nothing to explain)', () => {
    renderModal()
    selectFirstCategory()
    expect(generateButton()).not.toBeDisabled()
    expect(generateButton()).not.toHaveAttribute('title')
  })

  it('lists the theme alone when it is the only missing condition', () => {
    renderModal({ quizTheme: '' })
    selectFirstCategory()
    expect(generateButton()).toHaveAttribute(
      'title',
      'Champ(s) requis manquant(s) : le thème (section Quiz)'
    )
  })

  it('lists the population alone when it is the only missing condition', () => {
    renderModal({ quizPopulations: [] })
    selectFirstCategory()
    expect(generateButton()).toHaveAttribute(
      'title',
      'Champ(s) requis manquant(s) : au moins un public (section Quiz)'
    )
  })

  it('lists the difficulty alone when it is the only missing condition', () => {
    renderModal({ quizDifficulties: [] })
    selectFirstCategory()
    expect(generateButton()).toHaveAttribute(
      'title',
      'Champ(s) requis manquant(s) : au moins une difficulté (section Quiz)'
    )
  })

  it('lists the category alone when no category is selected (everything else valid)', () => {
    renderModal()
    // No selectFirstCategory() — that is exactly the condition under test.
    expect(generateButton()).toHaveAttribute(
      'title',
      'Champ(s) requis manquant(s) : au moins une catégorie cible'
    )
  })

  it('lists the question type alone when every type is disabled', () => {
    renderModal()
    selectFirstCategory()
    fireEvent.click(screen.getByLabelText('Activer Speedy'))
    fireEvent.click(screen.getByLabelText('Activer QCM'))
    fireEvent.click(screen.getByLabelText('Activer Memory'))
    expect(generateButton()).toHaveAttribute(
      'title',
      'Champ(s) requis manquant(s) : au moins un type de question activé'
    )
  })

  it('joins multiple missing reasons, in declaration order (theme, population, difficulty, category)', () => {
    renderModal({ quizTheme: '', quizPopulations: [], quizDifficulties: [] })
    // No selectFirstCategory() — category also missing, matching the "formulaire
    // vide" example from the handoff (type/volume have valid defaults, so absent
    // from the list).
    expect(generateButton()).toHaveAttribute(
      'title',
      'Champ(s) requis manquant(s) : le thème (section Quiz), au moins un public (section Quiz), au moins une difficulté (section Quiz), au moins une catégorie cible'
    )
  })

  it('the reason disappears from the list as soon as its own condition is fixed, others staying reported', () => {
    renderModal({ quizTheme: '', quizPopulations: [] })
    selectFirstCategory()
    expect(generateButton()).toHaveAttribute(
      'title',
      'Champ(s) requis manquant(s) : le thème (section Quiz), au moins un public (section Quiz)'
    )
  })
})
