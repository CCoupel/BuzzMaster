import { describe, it, expect, afterEach, vi } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import RafaleAIGenerateModal from './RafaleAIGenerateModal'

// Tests dérivés de contracts/rafale-ai-generation.md (§2bis paliers, §3.1
// catégories annotées, §6ter pas d'état "ajouté") et du plan
// _work/reports/plan-20260901-162105.md (tâche 11) — issue #203, v8.1.0.
//
// Écrit contre l'implémentation réelle (dev-frontend a livré
// RafaleAIGenerateModal.jsx en parallèle, Batch 1) : la modale ne porte que
// le formulaire + buildPayload/canSubmit, tout le reste (machine à états,
// cycle de vie du job, filtrage TARGET) vit dans AIJobModalShell — testé
// séparément dans ai/AIJobModalShell.target.test.jsx. Ce fichier se
// concentre donc sur ce qui est PROPRE à la modale Rafale : paliers,
// comptes par catégorie, matrice existant→après, absence de tout état
// "fraîchement ajouté".
//
// framer-motion est aliasé globalement en mock via vite.config.js
// test.alias (cf. AIGenerateModal.test.jsx) — non utilisé directement par ce
// composant mais par des dépendances transverses de la page hôte ; gardé
// pour cohérence avec le reste de la suite.

const CATEGORIES = [
  { key: 'HISTORY', name: 'Histoire', color: '#f97316' },
  { key: 'SCIENCE', name: 'Sciences & Nature', color: '#22c55e' },
  { key: 'GEOGRAPHY', name: 'Géographie', color: '#3b82f6' },
]

function questionFixture(category, difficulty, n = 1) {
  return Array.from({ length: n }, (_, i) => ({
    ID: `r-${category}-${difficulty}-${i}`,
    QUESTION: `Q${i}`,
    ANSWER: `A${i}`,
    CATEGORY: category,
    DIFFICULTY: difficulty,
    USED: false,
  }))
}

function defaultModalProps(overrides = {}) {
  return {
    onClose: vi.fn(),
    apiKeyConfigured: true,
    provider: 'anthropic',
    aiJob: null,
    onCancelGeneration: vi.fn(),
    interBatchDelayMs: 60000,
    maxConsecutiveFailures: 2,
    categories: CATEGORIES,
    questions: [],
    maxQuestions: 200,
    ...overrides,
  }
}

function modalTree(props, route) {
  return (
    <MemoryRouter initialEntries={[route]}>
      <Routes>
        <Route path="/admin/settings" element={<div data-testid="settings-page">Settings</div>} />
        <Route path="*" element={<RafaleAIGenerateModal {...props} />} />
      </Routes>
    </MemoryRouter>
  )
}

function renderModal(overrides = {}, { route = '/admin/rafale' } = {}) {
  const props = defaultModalProps(overrides)
  const utils = render(modalTree(props, route))
  return { props, ...utils }
}

// presetButton disambiguates a volume-preset button from a matrix cell's
// <strong> total that can coincidentally carry the very same digits (e.g.
// preset "10" vs. an "after" total of 10) — role-scoped, unlike a bare
// getByText which matches any element's text content.
function presetButton(n) {
  return screen.getByRole('button', { name: String(n) })
}

function fillMinimalValidForm() {
  fireEvent.change(screen.getByPlaceholderText('ex. Culture générale — France'), { target: { value: 'Culture générale' } })
  fireEvent.click(screen.getByText('Adulte (18-64 ans)'))
  fireEvent.click(screen.getByText('Histoire'))
  fireEvent.click(screen.getByText('★☆☆'))
}

describe('RafaleAIGenerateModal — paliers de volume (contrat §2bis)', () => {
  afterEach(() => vi.clearAllMocks())

  it('renders exactly the 5 presets when maxQuestions allows all of them', () => {
    renderModal({ maxQuestions: 200 })
    for (const p of [10, 20, 50, 100, 200]) {
      expect(presetButton(p)).toBeInTheDocument()
    }
  })

  it('masks presets above the configured maxQuestions ceiling', () => {
    renderModal({ maxQuestions: 50 })
    expect(presetButton(10)).toBeInTheDocument()
    expect(presetButton(20)).toBeInTheDocument()
    expect(presetButton(50)).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '100' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '200' })).not.toBeInTheDocument()
  })

  it('defaults to the 20 preset selected (active)', () => {
    renderModal()
    expect(presetButton(20)).toHaveClass('active')
  })

  it('selecting a different preset marks it active and the previous one inactive', () => {
    renderModal()
    fireEvent.click(presetButton(50))
    expect(presetButton(50)).toHaveClass('active')
    expect(presetButton(20)).not.toHaveClass('active')
  })

  it('falls back to the largest still-visible preset when the previously selected one becomes hidden (maxQuestions lowered)', () => {
    renderModal({ maxQuestions: 10 })
    // Default state selected 20, but maxQuestions=10 only leaves the 10
    // preset visible — the component must not submit an invisible preset.
    expect(presetButton(10)).toHaveClass('active')
    expect(screen.queryByRole('button', { name: '20' })).not.toBeInTheDocument()
  })
})

describe('RafaleAIGenerateModal — comptes par catégorie (contrat §3.1)', () => {
  afterEach(() => vi.clearAllMocks())

  it('annotates each category chip with its existing reservoir count (no difficulty selected = all difficulties)', () => {
    const questions = [...questionFixture('HISTORY', 1, 3), ...questionFixture('HISTORY', 2, 2), ...questionFixture('SCIENCE', 1, 1)]
    renderModal({ questions })
    const historyChip = screen.getByText('Histoire').closest('button')
    const scienceChip = screen.getByText('Sciences & Nature').closest('button')
    expect(historyChip.querySelector('.rafale-ai-cat-count').textContent).toBe('5')
    expect(scienceChip.querySelector('.rafale-ai-cat-count').textContent).toBe('1')
  })

  it('shows 0 for a category absent from the reservoir — no amorçage blockage (contrat §3.1)', () => {
    renderModal({ questions: questionFixture('HISTORY', 1, 2) })
    const geoChip = screen.getByText('Géographie').closest('button')
    expect(geoChip.querySelector('.rafale-ai-cat-count').textContent).toBe('0')
    expect(geoChip).not.toBeDisabled()
  })

  it('recalculates category counts when the selected difficulty changes', () => {
    const questions = [...questionFixture('HISTORY', 1, 4), ...questionFixture('HISTORY', 3, 1)]
    renderModal({ questions })
    const historyChip = screen.getByText('Histoire').closest('button')
    // No difficulty selected yet — aggregates all difficulties (5).
    expect(historyChip.querySelector('.rafale-ai-cat-count').textContent).toBe('5')

    fireEvent.click(screen.getByText('★☆☆')) // select difficulty 1
    expect(historyChip.querySelector('.rafale-ai-cat-count').textContent).toBe('4')

    fireEvent.click(screen.getByText('★★★')) // also select difficulty 3
    expect(historyChip.querySelector('.rafale-ai-cat-count').textContent).toBe('5')

    fireEvent.click(screen.getByText('★☆☆')) // deselect difficulty 1, only 3 remains
    expect(historyChip.querySelector('.rafale-ai-cat-count').textContent).toBe('1')
  })
})

describe('RafaleAIGenerateModal — matrice existant → après (contrat §2bis)', () => {
  afterEach(() => vi.clearAllMocks())

  it('is absent until at least one category AND one difficulty are selected', () => {
    renderModal()
    expect(screen.queryByText(/existant/)).not.toBeInTheDocument()
    fireEvent.click(screen.getByText('Histoire'))
    expect(screen.queryByText(/existant/)).not.toBeInTheDocument()
  })

  it('shows existing -> after values for a single couple, uniform split', () => {
    renderModal({ questions: questionFixture('HISTORY', 1, 3) })
    fireEvent.click(screen.getByText('Histoire'))
    fireEvent.click(screen.getByText('★☆☆'))
    fireEvent.click(presetButton(20)) // preset 20, 1 couple -> +20

    const cell = document.querySelector('.rafale-ai-matrix-cell')
    expect(cell.textContent).toContain('3')
    expect(cell.textContent).toContain('23') // 3 existing + 20 generated
  })

  it('splits the remainder to the first couples in request (category) order', () => {
    renderModal({ questions: [] })
    fireEvent.click(screen.getByText('Histoire')) // selected first
    fireEvent.click(screen.getByText('Sciences & Nature')) // selected second
    fireEvent.click(screen.getByText('★☆☆'))
    fireEvent.click(presetButton(10)) // 10 / 2 couples = 5 + 5, no remainder — use a case with remainder instead

    // 10 across 2 couples splits evenly (5/5) — verify no remainder case
    // first, then re-render for the actual remainder assertion below via a
    // 3rd category to produce 10 / 3 = 3,3,4.
    const cells = document.querySelectorAll('.rafale-ai-matrix-cell')
    expect(cells).toHaveLength(2)
    expect(cells[0].textContent).toContain('5')
    expect(cells[1].textContent).toContain('5')
  })

  it('remainder goes to the first couple when the total does not divide evenly', () => {
    renderModal({ questions: [] })
    fireEvent.click(screen.getByText('Histoire'))
    fireEvent.click(screen.getByText('Sciences & Nature'))
    fireEvent.click(screen.getByText('Géographie'))
    fireEvent.click(screen.getByText('★☆☆'))
    fireEvent.click(presetButton(10)) // 10 / 3 couples = 3,3,3 + 1 remainder -> first gets 4

    const rows = document.querySelectorAll('.rafale-ai-matrix tbody tr')
    expect(rows).toHaveLength(3)
    // First row (Histoire, request order) absorbs the remainder: 0 -> 4.
    expect(rows[0].textContent).toContain('4')
  })
})

describe('RafaleAIGenerateModal — aucun état "ajouté" (contrat §6ter, GATE 2)', () => {
  afterEach(() => vi.clearAllMocks())

  it('never renders a "nouveau" badge, regardless of job state', () => {
    renderModal({
      aiJob: { jobId: 'gen-1', state: 'DONE', target: 'RAFALE', createdCount: 5, skippedCount: 0, batchesDone: 1, batchesTotal: 1 },
    })
    expect(screen.queryByText(/nouveau/i)).not.toBeInTheDocument()
  })

  it('the DoneBody shows no per-category breakdown list (breakdown is always [])', () => {
    renderModal({
      aiJob: { jobId: 'gen-1', state: 'DONE', target: 'RAFALE', createdCount: 5, skippedCount: 0, batchesDone: 1, batchesTotal: 1 },
    })
    expect(document.querySelector('.ai-success-list')).not.toBeInTheDocument()
  })
})

describe('RafaleAIGenerateModal — canSubmit et payload', () => {
  afterEach(() => vi.clearAllMocks())

  it('"✨ Générer" is disabled with no field filled', () => {
    renderModal()
    expect(screen.getByText('✨ Générer').closest('button')).toBeDisabled()
  })

  it('"✨ Générer" becomes enabled once theme, a population, a category and a difficulty are all set', () => {
    renderModal()
    fillMinimalValidForm()
    expect(screen.getByText('✨ Générer').closest('button')).not.toBeDisabled()
  })

  it('lists every missing field in the disabled tooltip', () => {
    renderModal()
    const btn = screen.getByText('✨ Générer').closest('button')
    expect(btn.getAttribute('title')).toContain('thème')
    expect(btn.getAttribute('title')).toContain('public')
    expect(btn.getAttribute('title')).toContain('catégorie')
    expect(btn.getAttribute('title')).toContain('difficulté')
  })

  it('POSTs to /api/rafale/generate-questions with the built payload on submit', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      status: 202,
      json: async () => ({ status: 'accepted', job_id: 'gen-1', batches_total: 1 }),
    })
    vi.stubGlobal('fetch', fetchMock)

    renderModal()
    fillMinimalValidForm()
    fireEvent.click(screen.getByText('✨ Générer'))
    await waitFor(() => expect(fetchMock).toHaveBeenCalled())

    expect(fetchMock).toHaveBeenCalledWith('/api/rafale/generate-questions', expect.objectContaining({ method: 'POST' }))
    const body = JSON.parse(fetchMock.mock.calls[0][1].body)
    expect(body).toEqual({
      theme: 'Culture générale',
      populations: ['Adulte (18-64 ans)'],
      language: 'Français',
      instructions: '',
      categories: ['HISTORY'],
      difficulties: [1],
      count: 20,
    })
    vi.unstubAllGlobals()
  })
})
