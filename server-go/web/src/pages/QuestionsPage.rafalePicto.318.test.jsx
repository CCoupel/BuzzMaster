/**
 * #318 (v13.1.0, LOT-3) — QuestionsPage + PictoEditor : éditeur de question RAFALE_PICTO (Rafale Picto)
 * (contrat picto.md §3.1, §4, §11 ; maquette M5 C `admin_question_editor__picto_settings.html`).
 * Scaffold calqué sur QuestionsPage.roue.145.test.jsx. L'éditeur n'implémente aucune règle de mot : les
 * refus (doublon, caractère, longueur) sont ceux du serveur, affichés tels quels.
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

let postStatus
let postBody

beforeEach(() => {
  vi.clearAllMocks()
  postStatus = 200
  postBody = ''
  global.fetch = vi.fn(async (url) => {
    if (url === '/questions') {
      return postStatus === 200
        ? { ok: true, status: 200, json: async () => ({ status: 'ok', warning: null }), text: async () => '' }
        : { ok: false, status: postStatus, json: async () => ({}), text: async () => postBody }
    }
    return { ok: true, status: 200, json: async () => ({}) }
  })
  useGame.mockReturnValue({ questions: {}, fsInfo: { used: 0, total: 100 }, deleteQuestion: vi.fn(), sendMessage: vi.fn(), gameState: { phase: 'STOPPED', question: null }, newGame: vi.fn() })
  useCategories.mockReturnValue({ categories: [{ key: 'ANIMALS', name: 'Animaux', imageURL: '', isCustom: false }], loading: false, error: null, refetch: vi.fn() })
})

const openRafalePicto = () => {
  const r = render(<QuestionsPage />)
  fireEvent.click(screen.getByRole('button', { name: 'Rafale Picto' }))
  return r
}
const submitBtn = () => screen.getByRole('button', { name: 'Ajouter' })
const fillTitle = () => fireEvent.change(document.getElementById('question-input'), { target: { value: 'Rafale !' } })
const postedForm = () => {
  const call = global.fetch.mock.calls.find(c => c[0] === '/questions')
  return call ? call[1].body : null
}
const pressed = (name) => screen.getByRole('button', { name }).getAttribute('aria-pressed') === 'true'

// #318 (v13.1.0, LOT-3 T23) — éditeur « Rafale Picto » : sous-éditeur partagé (préfixe RAFALE_PICTO_) + bloc Passes.
describe('QuestionsPage — type RAFALE_PICTO (#318)', () => {
  it('le type « Rafale Picto » est proposé en dernier et monte l\'éditeur avec le bloc Passes', () => {
    render(<QuestionsPage />)
    const labels = Array.from(document.querySelectorAll('.type-filter-row')[0].querySelectorAll('.type-btn')).map(b => b.textContent)
    expect(labels[labels.length - 1]).toBe('Rafale Picto')
    expect(screen.queryByTestId('picto-passes')).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: 'Rafale Picto' }))
    expect(screen.getByTestId('picto-passes')).toBeTruthy()
    expect(screen.getByTestId('picto-words-source')).toBeTruthy()
  })

  it('le type « Picto » (un mot) n\'a pas le bloc Passes', () => {
    render(<QuestionsPage />)
    fireEvent.click(screen.getByRole('button', { name: 'Picto' }))
    expect(screen.queryByTestId('picto-passes')).toBeNull()
  })

  it('défauts : durée 90 s, 10 pts par mot, Pool Picto, passes illimitées et gratuites, rebouclage', () => {
    openRafalePicto()
    expect(screen.getByLabelText(/Durée de la manche/).value).toBe('90')
    expect(screen.getByLabelText(/Points par mot trouvé/).value).toBe('10')
    expect(pressed('🗂 Pool Picto')).toBe(true)
    expect(screen.getByLabelText(/Coût d'une passe/).value).toBe('0')
    expect(pressed('Illimitées')).toBe(true)
    expect(pressed('🔁 Reboucler sur les passés')).toBe(true)
  })

  it('l\'interrupteur de passes : « 1 passe » / « Illimitées » ; fin de pool : reboucler / finir', () => {
    openRafalePicto()
    fireEvent.click(screen.getByRole('button', { name: '1 passe' }))
    expect(pressed('1 passe')).toBe(true)
    expect(pressed('Illimitées')).toBe(false)
    fireEvent.click(screen.getByRole('button', { name: '⏹ Finir la manche' }))
    expect(pressed('⏹ Finir la manche')).toBe(true)
    expect(screen.getByTestId('picto-pass-summary').textContent).toContain('1 passe')
  })

  it('le coût de passe doit être 0 à 100 : hors bornes, « Ajouter » est désactivé', () => {
    openRafalePicto()
    fillTitle()
    fireEvent.click(screen.getAllByTitle('Geographie')[0])
    expect(submitBtn().disabled).toBe(false)
    fireEvent.change(screen.getByLabelText(/Coût d'une passe/), { target: { value: '101' } })
    expect(submitBtn().disabled).toBe(true)
    fireEvent.change(screen.getByLabelText(/Coût d'une passe/), { target: { value: '100' } })
    expect(submitBtn().disabled).toBe(false)
  })

  it('POST pool : type=RAFALE_PICTO, champs rafale_picto_*, aucun champ picto_* ni mots', async () => {
    openRafalePicto()
    fillTitle()
    fireEvent.click(screen.getAllByTitle('Geographie')[0])
    fireEvent.change(screen.getByLabelText(/Coût d'une passe/), { target: { value: '5' } })
    fireEvent.click(screen.getByRole('button', { name: '1 passe' }))
    fireEvent.click(screen.getByRole('button', { name: '⏹ Finir la manche' }))
    fireEvent.click(submitBtn())
    await waitFor(() => expect(postedForm()).not.toBeNull(), { timeout: 2000 })
    const fd = postedForm()
    expect(fd.get('type')).toBe('RAFALE_PICTO')
    expect(fd.get('rafale_picto_source')).toBe('RESERVOIR')
    expect(JSON.parse(fd.get('rafale_picto_categories'))).toEqual(['GEOGRAPHY'])
    expect(JSON.parse(fd.get('rafale_picto_difficulties'))).toEqual([1, 2, 3])
    expect(fd.get('rafale_picto_words')).toBe('')
    expect(fd.get('rafale_picto_pass_cost')).toBe('5')
    expect(fd.get('rafale_picto_pass_limit')).toBe('1')
    expect(fd.get('rafale_picto_pool_end')).toBe('END')
    expect(fd.get('time')).toBe('90')
    expect(fd.get('picto_source')).toBeNull()
    expect(fd.get('picto_words')).toBeNull()
  })

  it('POST mots saisis : rafale_picto_words un par ligne, source WORDS, pas de filtre', async () => {
    openRafalePicto()
    fillTitle()
    fireEvent.click(screen.getByRole('button', { name: '✍️ Saisis dans la question' }))
    const input = screen.getByLabelText(/Mots de la question/)
    fireEvent.change(input, { target: { value: 'chat' } }); fireEvent.keyDown(input, { key: 'Enter' })
    fireEvent.change(input, { target: { value: 'chien' } }); fireEvent.keyDown(input, { key: 'Enter' })
    fireEvent.click(submitBtn())
    await waitFor(() => expect(postedForm()).not.toBeNull(), { timeout: 2000 })
    const fd = postedForm()
    expect(fd.get('rafale_picto_source')).toBe('WORDS')
    expect(fd.get('rafale_picto_words')).toBe('chat\nchien')
    expect(fd.get('rafale_picto_categories')).toBeNull()
    expect(fd.get('rafale_picto_pass_limit')).toBe('0')
    expect(fd.get('rafale_picto_pool_end')).toBe('LOOP')
  })

  it('un refus PICTO_PASS_INVALID du serveur s\'affiche dans l\'éditeur', async () => {
    postStatus = 400
    postBody = 'PICTO_PASS_INVALID: passes 0 ou 1'
    openRafalePicto()
    fillTitle()
    fireEvent.click(screen.getAllByTitle('Geographie')[0])
    fireEvent.click(submitBtn())
    await waitFor(() => expect(screen.getByRole('alert').textContent).toMatch(/PICTO_PASS_INVALID/), { timeout: 2000 })
  })
})
