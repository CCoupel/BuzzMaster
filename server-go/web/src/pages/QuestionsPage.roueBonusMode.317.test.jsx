/**
 * #317 (v12.2.0, LOT-1) — QuestionsPage, type ROUE : le champ `roue_bonus_mode` est envoyé à POST /questions
 * (contracts/roue.md §17.5 : « absent ou vide = global, champ retiré » ; un champ non envoyé serait perdu, leçon #107).
 * Scaffold calqué sur QuestionsPage.roue.308.test.jsx (fichier NEUF).
 */
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react'
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

const GLOBAL_WHEEL = { max_points: 1000, step: 50, bankrupt_count: 2, lose_turn_count: 1, bonus_count: 2, bonus_mode: 'EPHEMERAL' }
const WHEEL_PREVIEW = { COUNT: 4, step: 50, SEGMENTS: [{ ID: 's0', KIND: 'value', VALUE: 1000, BONUS: false }, { ID: 's1', KIND: 'bankrupt', VALUE: 0, BONUS: false }, { ID: 's2', KIND: 'value', VALUE: 800, BONUS: true }, { ID: 's3', KIND: 'lose_turn', VALUE: 0, BONUS: false }] }

beforeEach(() => {
  vi.clearAllMocks()
  useGame.mockReturnValue({ questions: {}, fsInfo: { used: 0, total: 100 }, deleteQuestion: vi.fn(), sendMessage: vi.fn(), gameState: { phase: 'STOPPED', question: null }, newGame: vi.fn() })
  useCategories.mockReturnValue({ categories: [{ key: 'ANIMALS', name: 'Animaux', imageURL: '', isCustom: false }], loading: false, error: null, refetch: vi.fn() })
  global.fetch = vi.fn(async (url) => {
    if (url === '/api/roue/preview') return { ok: true, status: 200, json: async () => PREVIEW_OK }
    if (url === '/questions') return { ok: true, status: 200, json: async () => ({ status: 'ok', warning: null }) }
    if (url === '/game-config.json') return { ok: true, status: 200, json: async () => ({ wheel: GLOBAL_WHEEL }) }
    if (url === '/api/roue/wheel-preview') return { ok: true, status: 200, json: async () => WHEEL_PREVIEW }
    return { ok: true, status: 200, json: async () => ({}) }
  })
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
const modeButton = () => within(screen.getByTestId('roue-wparam-X')).getByRole('button', { name: /mode du BONUS/i })
const modeSwitch = () => within(screen.getByTestId('roue-wparam-X')).getByRole('switch', { name: /une seule fois par manche/i })
const fill = async () => {
  fireEvent.change(screen.getByLabelText(/Phrase à deviner/), { target: { value: 'Combien pour ce chien dans la vitrine' } })
  await waitFor(() => expect(screen.getByTestId('roue-wparam-X').querySelector('input').value).toBe('2'), { timeout: 2000 })
  await waitFor(() => expect(submitBtn().disabled).toBe(false), { timeout: 2000 })
}
const submit = async () => {
  fireEvent.click(submitBtn())
  await waitFor(() => expect(postedForm()).not.toBeNull(), { timeout: 2000 })
  return postedForm()
}

describe('QuestionsPage — type ROUE : mode du BONUS (#317)', () => {
  it('critical — question neuve : roue_bonus_mode envoyé VIDE (suit le global, champ retiré côté disque)', async () => {
    openRoue()
    await fill()
    expect((await submit()).get('roue_bonus_mode')).toBe('')
  })

  it('✏️ sans toucher à l\'interrupteur : le mode global (éphémère) est enregistré tel quel pour la question', async () => {
    openRoue()
    await fill()
    fireEvent.click(modeButton())
    expect(modeSwitch().checked).toBe(true)
    expect((await submit()).get('roue_bonus_mode')).toBe('EPHEMERAL')
  })

  it('interrupteur décoché : roue_bonus_mode = PERMANENT', async () => {
    openRoue()
    await fill()
    fireEvent.click(modeButton())
    fireEvent.click(modeSwitch())
    expect(modeSwitch().checked).toBe(false)
    expect((await submit()).get('roue_bonus_mode')).toBe('PERMANENT')
  })

  it('🔗 après une personnalisation : le champ repart vide (retiré)', async () => {
    openRoue()
    await fill()
    fireEvent.click(modeButton())
    fireEvent.click(modeSwitch())
    fireEvent.click(modeButton()) // 🔗
    expect((await submit()).get('roue_bonus_mode')).toBe('')
  })

  it('le mode personnalisé ne perturbe pas le compteur : roue_bonus_count reste vide (suit le global)', async () => {
    openRoue()
    await fill()
    fireEvent.click(modeButton())
    fireEvent.click(modeSwitch())
    const form = await submit()
    expect(form.get('roue_bonus_mode')).toBe('PERMANENT')
    expect(form.get('roue_bonus_count')).toBe('')
  })

  it('un refus 400 ROUE_BONUS_MODE_INVALID du serveur n\'est pas avalé', async () => {
    const ok = global.fetch
    global.fetch = vi.fn(async (url, opts) => (url === '/questions'
      ? { ok: false, status: 400, text: async () => 'ROUE_BONUS_MODE_INVALID: roue_bonus_mode doit valoir EPHEMERAL ou PERMANENT', json: async () => ({}) }
      : ok(url, opts)))
    openRoue()
    await fill()
    fireEvent.click(submitBtn())
    await waitFor(() => expect(document.body.textContent).toMatch(/ROUE_BONUS_MODE_INVALID|mode du BONUS|EPHEMERAL/i), { timeout: 2000 })
  })
})
