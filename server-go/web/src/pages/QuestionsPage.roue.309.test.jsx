/**
 * #309 (v12.1.0, LOT-2) — QuestionsPage, type ROUE : lecture et envoi des 5 champs de composition (vide = suit le global, valeur = personnalisé,
 * 0 compris) — contracts/roue.md §16.5. Un champ non lu serait perdu (leçon #107). Scaffold de QuestionsPage.roue.308.test.jsx.
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
  default: ({ question, onEdit }) => <div data-testid={`qcard-${question.ID}`}><button onClick={() => onEdit && onEdit(question)}>edit-{question.ID}</button></div>,
  CATEGORIES: { ANIMALS: { label: 'Animaux', icon: '🐾', color: '#3b82f6' } },
}))
vi.mock('../components/RafalePoolAlert', () => ({ default: () => null }))
vi.mock('./QuestionsPage.css', () => ({}))
vi.mock('./ConfigPage.css', () => ({}))

import { useGame } from '../hooks/GameContext'
import { useCategories } from '../hooks/useCategories'

const PREVIEW_OK = { COLS: 14, MAX_ROWS: 4, ROWS: ['COMBIEN POUR'], CONSONANTS: 12, LETTERS: 17 }
const GLOBAL = { max_points: 1000, step: 50, bankrupt_count: 2, lose_turn_count: 1, bonus_count: 0 }
const seg = (i, kind, v = 0) => ({ ID: `s${i}`, KIND: kind, VALUE: v })
let wheelResponse

beforeEach(() => {
  vi.clearAllMocks()
  wheelResponse = { ok: true, status: 200, body: { COUNT: 2, step: 50, SEGMENTS: [seg(0, 'value', 1000), seg(1, 'bankrupt')] } }
  global.fetch = vi.fn(async (url) => {
    if (url === '/api/roue/preview') return { ok: true, status: 200, json: async () => PREVIEW_OK }
    if (url === '/questions') return { ok: true, status: 200, json: async () => ({ status: 'ok', warning: null }) }
    if (url === '/game-config.json') return { ok: true, status: 200, json: async () => ({ wheel: GLOBAL }) }
    if (url === '/api/roue/wheel-preview') return { ok: wheelResponse.ok, status: wheelResponse.status, json: async () => wheelResponse.body }
    return { ok: true, status: 200, json: async () => ({}) }
  })
  useGame.mockReturnValue({ questions: {}, fsInfo: { used: 0, total: 100 }, deleteQuestion: vi.fn(), sendMessage: vi.fn(), gameState: { phase: 'STOPPED', question: null }, newGame: vi.fn() })
  useCategories.mockReturnValue({ categories: [{ key: 'ANIMALS', name: 'Animaux', imageURL: '', isCustom: false }], loading: false, error: null, refetch: vi.fn() })
})

const openRoue = () => { render(<QuestionsPage />); fireEvent.click(screen.getByRole('button', { name: 'Roue' })) }
const submitBtn = () => screen.getByRole('button', { name: 'Ajouter' })
const postedForm = () => { const c = global.fetch.mock.calls.find(x => x[0] === '/questions'); return c ? c[1].body : null }
const fill = async () => {
  fireEvent.change(screen.getByLabelText(/Phrase à deviner/), { target: { value: 'Combien pour ce chien dans la vitrine' } })
  await waitFor(() => expect(screen.getByTestId('roue-wparam-max').querySelector('input').value).toBe('1000'), { timeout: 2000 })
  await waitFor(() => expect(submitBtn().disabled).toBe(false), { timeout: 2000 })
}
const send = async () => { fireEvent.click(submitBtn()); await waitFor(() => expect(postedForm()).not.toBeNull(), { timeout: 2000 }); return postedForm() }
const toggle = (id) => screen.getByTestId(`roue-wparam-${id}`).querySelector('button')
const input = (id) => screen.getByTestId(`roue-wparam-${id}`).querySelector('input')
const KEYS = ['roue_wheel_max_points', 'roue_wheel_step', 'roue_wheel_bankrupt_count', 'roue_wheel_lose_turn_count', 'roue_bonus_count']

describe('QuestionsPage — type ROUE : composition de la roue (#309)', () => {
  it('critical — nouvelle question = tout global : les 5 champs sont envoyés VIDES (le serveur les retire)', async () => {
    openRoue(); await fill()
    const fd = await send()
    for (const k of KEYS) expect(fd.get(k), k).toBe('')
  })

  it('un paramètre personnalisé est envoyé, les autres restent vides', async () => {
    openRoue(); await fill()
    fireEvent.click(toggle('B'))
    fireEvent.change(input('B'), { target: { value: '4' } })
    const fd = await send()
    expect(fd.get('roue_wheel_bankrupt_count')).toBe('4')
    for (const k of KEYS.filter(k => k !== 'roue_wheel_bankrupt_count')) expect(fd.get(k), k).toBe('')
  })

  it('0 personnalisé est envoyé comme « 0 », jamais confondu avec vide', async () => {
    openRoue(); await fill()
    fireEvent.click(toggle('X')) // BONUS : global 0 → personnalisé 0
    const fd = await send()
    expect(fd.get('roue_bonus_count')).toBe('0')
    expect(fd.get('roue_wheel_max_points')).toBe('')
  })

  it('personnaliser puis revenir au global : le champ repart vide', async () => {
    openRoue(); await fill()
    fireEvent.click(toggle('max'))
    fireEvent.click(toggle('max'))
    const fd = await send()
    expect(fd.get('roue_wheel_max_points')).toBe('')
  })

  it('les 5 personnalisés sont tous envoyés', async () => {
    openRoue(); await fill()
    for (const [id, v] of [['max', '500'], ['step', '100'], ['B', '1'], ['P', '2'], ['X', '3']]) {
      fireEvent.click(toggle(id)); fireEvent.change(input(id), { target: { value: v } })
    }
    const fd = await send()
    expect(KEYS.map(k => fd.get(k))).toEqual(['500', '100', '1', '2', '3'])
  })

  it('composition refusée par le serveur (aperçu 400) : « Ajouter » reste grisé, rien n\'est envoyé', async () => {
    openRoue(); await fill()
    wheelResponse = { ok: false, status: 400, body: { CODE: 'ROUE_WHEEL_INVALID', MESSAGE: 'la roue aurait 70 cases' } }
    fireEvent.click(toggle('B'))
    fireEvent.change(input('B'), { target: { value: '70' } })
    await waitFor(() => expect(submitBtn().disabled).toBe(true), { timeout: 2000 })
    expect(postedForm()).toBeNull()
  })

  it('un refus ROUE_WHEEL_INVALID de POST /questions est affiché', async () => {
    global.fetch = vi.fn(async (url) => {
      if (url === '/questions') return { ok: false, status: 400, text: async () => 'ROUE_WHEEL_INVALID: la roue aurait 70 cases', json: async () => ({}) }
      if (url === '/api/roue/preview') return { ok: true, status: 200, json: async () => PREVIEW_OK }
      if (url === '/game-config.json') return { ok: true, status: 200, json: async () => ({ wheel: GLOBAL }) }
      return { ok: true, status: 200, json: async () => ({ COUNT: 2, step: 50, SEGMENTS: [seg(0, 'value', 1000), seg(1, 'bankrupt')] }) }
    })
    openRoue(); await fill()
    fireEvent.click(submitBtn())
    await waitFor(() => expect(document.body.textContent).toMatch(/70 cases/), { timeout: 2000 })
  })
})
