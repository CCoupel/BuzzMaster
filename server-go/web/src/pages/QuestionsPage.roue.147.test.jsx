/**
 * #147 (v12.1.0, LOT-1.2) — QuestionsPage, type ROUE : les 6 champs voyelle sont envoyés à POST /questions (contracts/roue.md §15.3 ;
 * un champ non envoyé serait perdu, leçon #107). Scaffold calqué sur QuestionsPage.roue.308.test.jsx.
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

const PREVIEW_OK = { COLS: 14, MAX_ROWS: 4, ROWS: ['COMBIEN POUR'], CONSONANTS: 12, LETTERS: 17 }

beforeEach(() => {
  vi.clearAllMocks()
  global.fetch = vi.fn(async (url) => {
    if (url === '/api/roue/preview') return { ok: true, status: 200, json: async () => PREVIEW_OK }
    if (url === '/questions') return { ok: true, status: 200, json: async () => ({ status: 'ok', warning: null }) }
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
  await waitFor(() => expect(submitBtn().disabled).toBe(false), { timeout: 2000 })
}
const send = async () => { fireEvent.click(submitBtn()); await waitFor(() => expect(postedForm()).not.toBeNull(), { timeout: 2000 }); return postedForm() }

describe('QuestionsPage — type ROUE : voyelles (#147)', () => {
  it('critical — question neuve : défauts envoyés (achat activé, BEFORE, MANY, MANY, 250, NONE)', async () => {
    openRoue(); await fill()
    const fd = await send()
    expect(fd.get('roue_buy_vowels')).toBe('true')
    expect(fd.get('roue_vowel_timing')).toBe('BEFORE')
    expect(fd.get('roue_vowels_before')).toBe('MANY')
    expect(fd.get('roue_vowels_after')).toBe('MANY')
    expect(fd.get('roue_vowel_price')).toBe('250')
    expect(fd.get('roue_vowel_gain')).toBe('NONE')
  })

  it('les choix de l\'éditeur sont envoyés tels quels', async () => {
    openRoue(); await fill()
    fireEvent.click(document.querySelector('input[name="roueVowelTiming"][value="BOTH"]'))
    fireEvent.click(document.querySelector('input[name="roueVowelsBefore"][value="ONE"]'))
    fireEvent.click(document.querySelector('input[name="roueVowelsAfter"][value="ONE"]'))
    fireEvent.click(document.querySelector('input[name="roueVowelGain"][value="PER_OCCURRENCE"]'))
    fireEvent.change(document.querySelector('#roue-vowel-price'), { target: { value: '400' } })
    const fd = await send()
    expect(fd.get('roue_vowel_timing')).toBe('BOTH')
    expect(fd.get('roue_vowels_before')).toBe('ONE')
    expect(fd.get('roue_vowels_after')).toBe('ONE')
    expect(fd.get('roue_vowel_gain')).toBe('PER_OCCURRENCE')
    expect(fd.get('roue_vowel_price')).toBe('400')
  })

  it('achat désactivé : roue_buy_vowels = false, les autres réglages restent envoyés', async () => {
    openRoue(); await fill()
    fireEvent.click(document.querySelector('input[name="roueVowelTiming"][value="AFTER"]'))
    fireEvent.click(screen.getByRole('switch', { name: /Autoriser l'achat de voyelles/i }))
    const fd = await send()
    expect(fd.get('roue_buy_vowels')).toBe('false')
    expect(fd.get('roue_vowel_timing')).toBe('AFTER')
    expect(fd.get('roue_vowel_price')).toBe('250')
  })

  it('prix invalide : « Ajouter » reste grisé, rien n\'est envoyé', async () => {
    openRoue(); await fill()
    fireEvent.change(document.querySelector('#roue-vowel-price'), { target: { value: '0' } })
    await waitFor(() => expect(submitBtn().disabled).toBe(true), { timeout: 2000 })
    expect(postedForm()).toBeNull()
  })

  it('un refus ROUE_VOWEL_PRICE_INVALID du serveur est affiché', async () => {
    global.fetch = vi.fn(async (url) => {
      if (url === '/api/roue/preview') return { ok: true, status: 200, json: async () => PREVIEW_OK }
      if (url === '/questions') return { ok: false, status: 400, text: async () => 'ROUE_VOWEL_PRICE_INVALID: prix', json: async () => ({}) }
      return { ok: true, status: 200, json: async () => ({}) }
    })
    openRoue(); await fill()
    fireEvent.click(submitBtn())
    await waitFor(() => expect(document.body.textContent).toMatch(/Prix d'une voyelle invalide/), { timeout: 2000 })
  })
})
