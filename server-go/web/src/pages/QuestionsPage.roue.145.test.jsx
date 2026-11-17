/**
 * #145 (v12.0.0, LOT-1.1) — QuestionsPage : type ROUE (contracts/roue.md §3, §11.1, maquette M3).
 * Scaffold calqué sur QuestionsPage.rafale-alpha.282.test.jsx. Vérifie le montage de l'éditeur, les champs
 * masqués (Réponse / Points / Temps), le verrou « Ajouter » tant que le serveur refuse la phrase, et le FormData
 * envoyé à POST /questions (les 6 champs ROUE_* — un champ non envoyé serait perdu, leçon #107).
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
let previewResponse

beforeEach(() => {
  vi.clearAllMocks()
  previewResponse = { ok: true, status: 200, body: PREVIEW_OK }
  global.fetch = vi.fn(async (url) => {
    if (url === '/api/roue/preview') return { ok: previewResponse.ok, status: previewResponse.status, json: async () => previewResponse.body }
    if (url === '/questions') return { ok: true, status: 200, json: async () => ({ status: 'ok', warning: null }) }
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
const phraseInput = () => screen.getByLabelText(/Phrase à deviner/)
const submitBtn = () => screen.getByRole('button', { name: 'Ajouter' })
const typePhrase = (v) => fireEvent.change(phraseInput(), { target: { value: v } })
const postedForm = () => {
  const call = global.fetch.mock.calls.find(c => c[0] === '/questions')
  return call ? call[1].body : null
}

describe('QuestionsPage — type ROUE (#145)', () => {
  it('le sélecteur de type propose « Roue » en dernier', () => {
    render(<QuestionsPage />)
    const labels = Array.from(document.querySelectorAll('.type-filter-row')[0].querySelectorAll('.type-btn')).map(b => b.textContent)
    expect(labels).toContain('Roue')
    expect(labels[labels.length - 1]).toBe('Roue')
  })

  it('choisir Roue monte l\'éditeur ROUE', () => {
    openRoue()
    expect(phraseInput()).toBeTruthy()
    expect(document.querySelector('input[name="roueMode"]')).not.toBeNull()
  })

  it('masque Réponse, Points et Temps pour ce type', () => {
    openRoue()
    expect(screen.queryByLabelText(/^Réponse/)).toBeNull()
    expect(screen.queryByLabelText(/^Points/)).toBeNull()
    expect(screen.queryByLabelText(/^Temps/)).toBeNull()
  })

  it('l\'éditeur ROUE n\'apparaît pas pour un autre type', () => {
    render(<QuestionsPage />)
    expect(screen.queryByLabelText(/Phrase à deviner/)).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: 'Rafale' }))
    expect(screen.queryByLabelText(/Phrase à deviner/)).toBeNull()
  })

  it('« Ajouter » est désactivé tant qu\'aucune phrase n\'est validée par le serveur', async () => {
    openRoue()
    expect(submitBtn().disabled).toBe(true)
    typePhrase('Combien pour ce chien dans la vitrine')
    await waitFor(() => expect(submitBtn().disabled).toBe(false), { timeout: 2000 })
  })

  it('« Ajouter » reste désactivé quand le serveur refuse la phrase', async () => {
    previewResponse = { ok: false, status: 400, body: { CODE: 'ROUE_PHRASE_TOO_LONG', MESSAGE: 'Phrase trop longue' } }
    openRoue()
    typePhrase('x'.repeat(80))
    await screen.findByRole('alert', {}, { timeout: 2000 })
    expect(submitBtn().disabled).toBe(true)
  })

  it('« Ajouter » se verrouille à nouveau si la phrase devient invalide après avoir été valide', async () => {
    openRoue()
    typePhrase('Bonjour')
    await waitFor(() => expect(submitBtn().disabled).toBe(false), { timeout: 2000 })
    previewResponse = { ok: false, status: 400, body: { CODE: 'ROUE_PHRASE_INVALID_CHAR', MESSAGE: 'Caractère non admis' } }
    typePhrase('mon_chien')
    await waitFor(() => expect(submitBtn().disabled).toBe(true), { timeout: 2000 })
  })

  it('smoke — enregistre une question ROUE : POST /questions avec les 6 champs ROUE_* et type=ROUE', async () => {
    openRoue()
    typePhrase('Combien pour ce chien dans la vitrine')
    fireEvent.change(screen.getByLabelText(/Phrase de présentation/), { target: { value: 'Chanson de Line Renaud' } })
    fireEvent.click(document.querySelector('input[name="roueMode"][value="CHACUN_SON_TOUR"]'))
    fireEvent.click(document.querySelector('input[name="roueGainsRule"][value="ALL_TEAMS"]'))
    fireEvent.click(document.querySelector('input[name="roueWheelLayout"][value="arc"]'))
    fireEvent.click(screen.getByRole('switch', { name: /lettres déjà proposées/i }))
    await waitFor(() => expect(submitBtn().disabled).toBe(false), { timeout: 2000 })
    fireEvent.click(submitBtn())
    await waitFor(() => expect(postedForm()).not.toBeNull(), { timeout: 2000 })
    const fd = postedForm()
    expect(fd.get('type')).toBe('ROUE')
    expect(fd.get('roue_phrase')).toBe('Combien pour ce chien dans la vitrine')
    expect(fd.get('roue_presentation')).toBe('Chanson de Line Renaud')
    expect(fd.get('ROUE_MODE')).toBe('CHACUN_SON_TOUR')
    expect(fd.get('roue_gains_rule')).toBe('ALL_TEAMS')
    expect(fd.get('roue_wheel_layout')).toBe('arc')
    expect(fd.get('roue_show_letters')).toBe('false')
  })

  it('valeurs par défaut envoyées : TANT_QUE_JE_GAGNE, WINNER_ONLY, strip, lettres affichées', async () => {
    openRoue()
    typePhrase('Bonjour')
    await waitFor(() => expect(submitBtn().disabled).toBe(false), { timeout: 2000 })
    fireEvent.click(submitBtn())
    await waitFor(() => expect(postedForm()).not.toBeNull(), { timeout: 2000 })
    const fd = postedForm()
    expect(fd.get('ROUE_MODE')).toBe('TANT_QUE_JE_GAGNE')
    expect(fd.get('roue_gains_rule')).toBe('WINNER_ONLY')
    expect(fd.get('roue_wheel_layout')).toBe('strip')
    expect(fd.get('roue_show_letters')).toBe('true')
  })

  // Contrat §9 : la phrase est secrète — elle ne transite jamais par le champ générique « question »
  // (texte visible sur la TV), ni par « answer ».
  it('critical — la phrase n\'est envoyée ni dans « question » ni dans « answer »', async () => {
    openRoue()
    typePhrase('Combien pour ce chien dans la vitrine')
    await waitFor(() => expect(submitBtn().disabled).toBe(false), { timeout: 2000 })
    fireEvent.click(submitBtn())
    await waitFor(() => expect(postedForm()).not.toBeNull(), { timeout: 2000 })
    const fd = postedForm()
    expect(String(fd.get('question') || '')).not.toMatch(/vitrine/i)
    expect(String(fd.get('answer') || '')).not.toMatch(/vitrine/i)
  })
})
