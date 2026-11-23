/**
 * #191 (v13.0.0, LOT-1) — QuestionsPage + PictoEditor : éditeur de question PICTO
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

const openPicto = () => {
  const r = render(<QuestionsPage />)
  fireEvent.click(screen.getByRole('button', { name: 'Picto' }))
  return r
}
const wordInput = () => screen.getByLabelText(/Mots de la question/)
const timeInput = () => screen.getByLabelText(/Durée de dessin/)
const submitBtn = () => screen.getByRole('button', { name: 'Ajouter' })
const chips = () => Array.from(document.querySelectorAll('.picto-words .picto-chip')).map(c => c.firstChild.textContent)
const addByEnter = (w) => { fireEvent.change(wordInput(), { target: { value: w } }); fireEvent.keyDown(wordInput(), { key: 'Enter' }) }
const paste = (text) => fireEvent.paste(wordInput(), { clipboardData: { getData: () => text } })
const postedForm = () => {
  const call = global.fetch.mock.calls.find(c => c[0] === '/questions')
  return call ? call[1].body : null
}
const fillTitle = () => fireEvent.change(document.getElementById('question-input'), { target: { value: 'Dessinez !' } })

describe('QuestionsPage — type PICTO (#191)', () => {
  it('le sélecteur propose « Picto » en dernier et monte l\'éditeur PICTO', () => {
    render(<QuestionsPage />)
    const labels = Array.from(document.querySelectorAll('.type-filter-row')[0].querySelectorAll('.type-btn')).map(b => b.textContent)
    expect(labels[labels.length - 1]).toBe('Picto')
    expect(screen.queryByLabelText(/Mots de la question/)).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: 'Picto' }))
    expect(wordInput()).toBeTruthy()
  })

  it('la durée est pré-remplie à 90 s à l\'entrée du type', () => {
    openPicto()
    expect(timeInput().value).toBe('90')
  })

  it('l\'éditeur PICTO n\'apparaît pas pour un autre type', () => {
    render(<QuestionsPage />)
    fireEvent.click(screen.getByRole('button', { name: 'Rafale' }))
    expect(screen.queryByLabelText(/Mots de la question/)).toBeNull()
  })

  it('ajout d\'un mot par Entrée, par virgule et par point-virgule', () => {
    openPicto()
    addByEnter('chat')
    fireEvent.change(wordInput(), { target: { value: 'chien' } })
    fireEvent.keyDown(wordInput(), { key: ',' })
    fireEvent.change(wordInput(), { target: { value: 'souris' } })
    fireEvent.keyDown(wordInput(), { key: ';' })
    expect(chips()).toEqual(['chat', 'chien', 'souris'])
    expect(wordInput().value).toBe('')
  })

  it('les espaces de début/fin sont retirés et les espaces internes compactés', () => {
    openPicto()
    addByEnter('  grand    chat  ')
    expect(chips()).toEqual(['grand chat'])
  })

  it('un collage multi-lignes (ou à virgules) ajoute tous les mots, lignes vides ignorées', () => {
    openPicto()
    paste('chat\n\nchien\r\nsouris, lapin')
    expect(chips()).toEqual(['chat', 'chien', 'souris', 'lapin'])
  })

  it('un mot est retiré par sa croix, et le dernier par Retour arrière sur champ vide', () => {
    openPicto()
    paste('chat\nchien\nsouris')
    fireEvent.click(screen.getByRole('button', { name: 'Retirer « chien »' }))
    expect(chips()).toEqual(['chat', 'souris'])
    fireEvent.keyDown(wordInput(), { key: 'Backspace' })
    expect(chips()).toEqual(['chat'])
  })

  it('la saisie est bloquée à 20 mots', () => {
    openPicto()
    paste(Array.from({ length: 20 }, (_, i) => `mot${i}`).join('\n'))
    expect(chips()).toHaveLength(20)
    expect(wordInput().disabled).toBe(true)
  })

  it('le résumé reflète la durée et le nombre de mots', () => {
    openPicto()
    paste('chat\nchien')
    const summary = screen.getByLabelText('Résumé de la question').textContent
    expect(summary).toContain('90 s')
    expect(summary).toContain('2 mots')
  })

  it('« Ajouter » est désactivé sans mot, puis activé avec 1 mot et une durée valide', () => {
    openPicto()
    fillTitle()
    expect(submitBtn().disabled).toBe(true)
    expect(screen.getByText('Saisissez au moins un mot.')).toBeTruthy()
    addByEnter('chat')
    expect(submitBtn().disabled).toBe(false)
  })

  it.each(['29', '301', ''])('« Ajouter » est désactivé pour une durée invalide (%s)', (t) => {
    openPicto()
    fillTitle()
    addByEnter('chat')
    fireEvent.change(timeInput(), { target: { value: t } })
    expect(submitBtn().disabled).toBe(true)
  })

  it.each(['30', '300'])('la durée limite %s est acceptée', (t) => {
    openPicto()
    fillTitle()
    addByEnter('chat')
    fireEvent.change(timeInput(), { target: { value: t } })
    expect(submitBtn().disabled).toBe(false)
  })

  it('smoke — enregistre : POST /questions type=PICTO, picto_words un mot par ligne, time=90', async () => {
    openPicto()
    fillTitle()
    paste('Éléphant\nJean-Pierre\nL\'arbre')
    fireEvent.click(submitBtn())
    await waitFor(() => expect(postedForm()).not.toBeNull(), { timeout: 2000 })
    const fd = postedForm()
    expect(fd.get('type')).toBe('PICTO')
    expect(fd.get('picto_words')).toBe('Éléphant\nJean-Pierre\nL\'arbre')
    expect(fd.get('time')).toBe('90')
  })

  it('un refus serveur PICTO_WORDS_INVALID est affiché dans l\'éditeur, mots conservés', async () => {
    postStatus = 400
    postBody = 'PICTO_WORDS_INVALID: doublon « elephant »'
    openPicto()
    fillTitle()
    paste('Éléphant\nelephant')
    fireEvent.click(submitBtn())
    const alert = await screen.findByRole('alert', {}, { timeout: 2000 })
    expect(alert.textContent).toContain('doublon « elephant »')
    expect(alert.textContent).not.toContain('PICTO_WORDS_INVALID')
    expect(chips()).toEqual(['Éléphant', 'elephant'])
  })

  it('un refus PICTO_TIME_INVALID sans détail affiche un libellé lisible', async () => {
    postStatus = 400
    postBody = 'PICTO_TIME_INVALID'
    openPicto()
    fillTitle()
    addByEnter('chat')
    fireEvent.click(submitBtn())
    const alert = await screen.findByRole('alert', {}, { timeout: 2000 })
    expect(alert.textContent).toMatch(/30 à 300/)
  })

  it('aucune règle de mot n\'est appliquée côté client (le serveur tranche) : doublon et caractère spécial acceptés à la saisie', () => {
    openPicto()
    addByEnter('chat')
    addByEnter('CHAT')
    addByEnter('mot!')
    expect(chips()).toEqual(['chat', 'CHAT', 'mot!'])
  })

  it('aucun libellé de l\'éditeur ne contient « Pictionary » ni « PICSONARY »', () => {
    openPicto()
    expect(document.querySelector('.picto-editor').textContent).not.toMatch(/pictionary|picsonary/i)
  })

  // M3 (revue LOT-1) : au-delà de 20 mots, les mots en trop ne sont pas ajoutés et un message le dit.
  it('M3 — un collage de 23 mots garde 20 mots et annonce « 3 mots ignorés »', () => {
    openPicto()
    paste(Array.from({ length: 23 }, (_, i) => `mot${i}`).join('\n'))
    expect(chips()).toHaveLength(20)
    expect(chips()[19]).toBe('mot19')
    const alert = screen.getByRole('alert')
    expect(alert.textContent).toBe('3 mots ignorés : 20 mots au maximum par question.')
  })

  it('M3 — « mot ignoré » au singulier pour un seul mot en trop', () => {
    openPicto()
    paste(Array.from({ length: 21 }, (_, i) => `mot${i}`).join('\n'))
    expect(screen.getByRole('alert').textContent).toBe('1 mot ignoré : 20 mots au maximum par question.')
  })

  it('M3 — le message disparaît au retrait d\'un mot ; aucun message sous la limite', () => {
    openPicto()
    paste('a\nb\nc')
    expect(screen.queryByRole('alert')).toBeNull()
    paste(Array.from({ length: 20 }, (_, i) => `x${i}`).join('\n'))
    expect(screen.getByRole('alert')).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: 'Retirer « a »' }))
    expect(screen.queryByRole('alert')).toBeNull()
  })

  it('M2 — le champ générique devient « Titre (liste des questions) », sur une ligne', () => {
    openPicto()
    expect(screen.getByLabelText(/Titre \(liste des questions\)/)).toBe(document.getElementById('question-input'))
    expect(document.getElementById('question-input').tagName).toBe('INPUT')
  })
})
