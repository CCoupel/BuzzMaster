/**
 * #282 (v11.4.0, LOT-2) — contracts/rafale.md §15.2/§15.7, maquette admin_question_editor__rafale_alphabetical :
 * éditeur RAFALE — interrupteur « 🔤 Alphabétique » (role="switch"), vignettes « Bandeau / Roue » visibles
 * seulement si actif (strip par défaut). Scaffold calqué sur QuestionsPage.rafale.test.jsx.
 * Le `data.append` des deux champs (rafale_alphabetical / rafale_wheel_layout) est vérifié côté Go
 * (round-trip multipart, internal/server/rafale_alpha_282_test.go) et par la procédure QA.
 */
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
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
  default: ({ children, onClick, disabled, type, ...rest }) => (
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
  CATEGORIES: { GEOGRAPHY: { label: 'Geographie', icon: '🌍', color: '#3b82f6' } },
}))
vi.mock('../components/RafalePoolAlert', () => ({
  default: ({ alphabetical }) => <div data-testid="rafale-pool-alert-stub" data-alphabetical={String(!!alphabetical)} />,
}))
vi.mock('./QuestionsPage.css', () => ({}))
vi.mock('./ConfigPage.css', () => ({}))

import { useGame } from '../hooks/GameContext'
import { useCategories } from '../hooks/useCategories'

beforeEach(() => {
  vi.clearAllMocks()
  useGame.mockReturnValue({ questions: {}, fsInfo: { used: 0, total: 100 }, deleteQuestion: vi.fn(), sendMessage: vi.fn(), gameState: { phase: 'STOPPED', question: null }, newGame: vi.fn() })
  useCategories.mockReturnValue({ categories: [{ key: 'GEOGRAPHY', name: 'Geographie', imageURL: '', isCustom: false }], loading: false, error: null, refetch: vi.fn() })
})

const openRafale = () => {
  const r = render(<QuestionsPage />)
  fireEvent.click(screen.getByRole('button', { name: 'Rafale' }))
  return r
}
const alphaSwitch = () => screen.queryByRole('switch', { name: /Alphabétique/ })

describe('QuestionsPage — éditeur RAFALE : option Alphabétique (#282)', () => {
  it('l\'interrupteur « Alphabétique » est présent, désactivé par défaut', () => {
    openRafale()
    const sw = alphaSwitch()
    expect(sw).not.toBeNull()
    expect(sw.getAttribute('aria-checked')).toBe('false')
  })

  it('vignettes Bandeau / Roue absentes tant que l\'option est désactivée', () => {
    const { container } = openRafale()
    expect(container.querySelector('input[name="rafaleWheelLayout"]')).toBeNull()
  })

  it('activer l\'option affiche les vignettes, « Bandeau » (strip) coché par défaut', () => {
    const { container } = openRafale()
    fireEvent.click(alphaSwitch())
    expect(alphaSwitch().getAttribute('aria-checked')).toBe('true')
    const strip = container.querySelector('input[name="rafaleWheelLayout"][value="strip"]')
    const arc = container.querySelector('input[name="rafaleWheelLayout"][value="arc"]')
    expect(strip).not.toBeNull()
    expect(arc).not.toBeNull()
    expect(strip.checked).toBe(true)
    expect(arc.checked).toBe(false)
  })

  it('choisir « Roue » sélectionne arc', () => {
    const { container } = openRafale()
    fireEvent.click(alphaSwitch())
    fireEvent.click(container.querySelector('input[name="rafaleWheelLayout"][value="arc"]'))
    expect(container.querySelector('input[name="rafaleWheelLayout"][value="arc"]').checked).toBe(true)
  })

  it('désactiver l\'option masque de nouveau les vignettes', () => {
    const { container } = openRafale()
    fireEvent.click(alphaSwitch())
    fireEvent.click(alphaSwitch())
    expect(container.querySelector('input[name="rafaleWheelLayout"]')).toBeNull()
  })

  it('RafalePoolAlert reçoit la valeur de l\'option', () => {
    openRafale()
    expect(screen.getByTestId('rafale-pool-alert-stub').getAttribute('data-alphabetical')).toBe('false')
    fireEvent.click(alphaSwitch())
    expect(screen.getByTestId('rafale-pool-alert-stub').getAttribute('data-alphabetical')).toBe('true')
  })

  it('les autres types de question (ex. QCM) ne montrent pas l\'interrupteur', () => {
    render(<QuestionsPage />)
    expect(alphaSwitch()).toBeNull()
  })
})
