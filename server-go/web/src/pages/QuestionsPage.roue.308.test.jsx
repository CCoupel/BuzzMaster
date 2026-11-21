/**
 * #308 (v12.1.0, LOT-1.1) — QuestionsPage, type ROUE : le champ `roue_bonus_count` est envoyé à POST /questions
 * (contracts/roue.md §15.3 ; un champ non envoyé serait perdu, leçon #107). Scaffold calqué sur QuestionsPage.roue.145.test.jsx.
 */
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import QuestionsPage from './QuestionsPage'

vi.mock('../hooks/GameContext', () => ({ useGame: vi.fn(), GameProvider: ({ children }) => children }))
vi.mock('react-router-dom', () => ({ useNavigate: () => vi.fn(), useSearchParams: () => [new URLSearchParams(), vi.fn()] }))
vi.mock('../hooks/useCategoryFilter', () => ({
  useCategoryFilter: vi.fn((questions) => ({
    selectedCategories: new Set(), availableCategories: [], filteredQuestions: questions || [],
    toggleCategoryFilter: vi.fn(), clearCategoryFilters: vi.fn(),
  })),
}))
vi.mock('../hooks/useCategories', () => ({ useCategories: vi.fn() }))
vi.mock('../components/Button', () => ({
  default: ({ children, onClick, disabled, type, loading, ...rest }) => (
    <button onClick={onClick} disabled={disabled} type={type || 'button'} {...rest}>{children}</button>
  ),
}))
vi.mock('../components/Card', () => ({
  default: ({ children, className }) => <div className={className}>{children}</div>,
  CardHeader: ({ children }) => <div>{children}</div>,
  CardBody: ({ children }) => <div>{children}</div>,
}))
vi.mock('../components/CategoryBalance', () => ({ default: () => null }))
vi.mock('../components/QuestionCard', () => ({
  default: ({ question }) => <div data-testid={`qcard-${question.ID}`} />,
  CATEGORIES: { ANIMALS: { label: 'Animaux', icon: '🐾', color: '#3b82f6' } },
}))
vi.mock('../components/RafalePoolAlert', () => ({ default: () => null }))
vi.mock('./QuestionsPage.css', () => ({}))
vi.mock('./ConfigPage.css', () => ({}))

import { useGame } from '../hooks/GameContext'
import { useCategories } from '../hooks/useCategories'

const PREVIEW_OK = { COLS: 14, MAX_ROWS: 4, ROWS: ['COMBIEN POUR', 'CE CHIEN DANS', 'LA VITRINE'], CONSONANTS: 12, LETTERS: 17 }

beforeEach(() => {
  vi.clearAllMocks()
  global.fetch = vi.fn(async (url) => {
    if (url === '/api/roue/preview') return { ok: true, status: 200, json: async () => PREVIEW_OK }
    if (url === '/questions') return { ok: true, status: 200, json: async () => ({ status: 'ok', warning: null }) }
    if (url === '/game-config.json') return { ok: true, status: 200, json: async () => ({ wheel: { max_points: 100, step: 10, bankrupt_count: 1, lose_turn_count: 1 } }) }
    if (url === '/api/roue/wheel-preview') return { ok: true, status: 200, json: async () => ({ COUNT: 12, SEGMENTS: [] }) }
    return { ok: true, status: 200, json: async () => ({}) }
  })
  useGame.mockReturnValue({ questions: {}, fsInfo: { used: 0, total: 100 }, deleteQuestion: vi.fn(), sendMessage: vi.fn(), gameState: { phase: 'STOPPED', question: null }, newGame: vi.fn() })
  useCategories.mockReturnValue({ categories: [{ key: 'ANIMALS', name: 'Animaux', imageURL: '', isCustom: false }], loading: false, error: null, refetch: vi.fn() })
})

const openRoue = () => {
  const r = render(<QuestionsPage />)
  fireEvent.click(screen.getByRole('button', { name: 'Roue' }))
  return r
}
const submitBtn = () => screen.getByRole('button', { name: 'Ajouter' })
const postedForm = () => {
  const call = global.fetch.mock.calls.find(c => c[0] === '/questions')
  return call ? call[1].body : null
}
const fill = async () => {
  fireEvent.change(screen.getByLabelText(/Phrase à deviner/), { target: { value: 'Combien pour ce chien dans la vitrine' } })
  await waitFor(() => expect(submitBtn().disabled).toBe(false), { timeout: 2000 })
}

describe('QuestionsPage — type ROUE : cases BONUS (#308)', () => {
  it('critical — question neuve : roue_bonus_count = 0 envoyé (absent côté disque, jamais perdu)', async () => {
    openRoue()
    await fill()
    fireEvent.click(submitBtn())
    await waitFor(() => expect(postedForm()).not.toBeNull(), { timeout: 2000 })
    expect(postedForm().get('roue_bonus_count')).toBe('0')
  })

  it('le compteur choisi (« plus » ×3) est envoyé tel quel', async () => {
    openRoue()
    await fill()
    const plus = screen.getByRole('button', { name: /Plus de cases BONUS/ })
    fireEvent.click(plus); fireEvent.click(plus); fireEvent.click(plus)
    fireEvent.click(submitBtn())
    await waitFor(() => expect(postedForm()).not.toBeNull(), { timeout: 2000 })
    expect(postedForm().get('roue_bonus_count')).toBe('3')
  })

  it('le compteur est plafonné à 4 dans le formulaire', async () => {
    openRoue()
    await fill()
    const plus = screen.getByRole('button', { name: /Plus de cases BONUS/ })
    for (let i = 0; i < 7; i++) fireEvent.click(plus)
    fireEvent.click(submitBtn())
    await waitFor(() => expect(postedForm()).not.toBeNull(), { timeout: 2000 })
    expect(postedForm().get('roue_bonus_count')).toBe('4')
  })

  it('un refus ROUE_BONUS_COUNT_INVALID du serveur n\'est pas avalé (message affiché)', async () => {
    global.fetch = vi.fn(async (url) => {
      if (url === '/api/roue/preview') return { ok: true, status: 200, json: async () => PREVIEW_OK }
      if (url === '/questions') return { ok: false, status: 400, text: async () => 'ROUE_BONUS_COUNT_INVALID: trop de cases', json: async () => ({}) }
      return { ok: true, status: 200, json: async () => ({ wheel: { max_points: 63, step: 1, bankrupt_count: 0, lose_turn_count: 0 }, COUNT: 0, SEGMENTS: [] }) }
    })
    openRoue()
    await fill()
    fireEvent.click(screen.getByRole('button', { name: /Plus de cases BONUS/ }))
    fireEvent.click(submitBtn())
    await waitFor(() => expect(document.body.textContent).toMatch(/invalide|trop de cases/i), { timeout: 2000 })
  })
})
