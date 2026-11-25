import { describe, it, expect, vi } from 'vitest'
import { render, screen, waitFor, fireEvent, within } from '@testing-library/react'
import PoolPage from './PoolPage'
import { useOptionalGame } from '../hooks/GameContext'

vi.mock('../hooks/GameContext', () => ({ useOptionalGame: vi.fn(() => null), useGame: vi.fn() }))

// #324 (v13.1.0, LOT-2 T13 générique) — PoolPage paramétrée : instance `picto` (colonne « Mot », pas de réponse,
// chemins /api/picto/words*) ; l'instance `rafale` (défaut) est couverte par RafalePage.*.test.jsx, inchangés.

const WORDS = [
  { ID: 'p-001', QUESTION: 'chat', ANSWER: '', CATEGORY: 'GEOGRAPHY', DIFFICULTY: 1, USED: false },
  { ID: 'p-002', QUESTION: 'maison', ANSWER: '', CATEGORY: 'HISTORY', DIFFICULTY: 2, USED: true },
]
const json = (body, ok = true, status = 200) => Promise.resolve({ ok, status, json: () => Promise.resolve(body), text: () => Promise.resolve(JSON.stringify(body)) })

function mockFetch() {
  return vi.fn((url, options = {}) => {
    const m = options.method || 'GET'
    if (url === '/api/picto/words' && m === 'GET') return json({ QUESTIONS: WORDS, TOTAL: 2 })
    if (url === '/api/picto/words' && m === 'POST') return json({ ID: 'p-003' })
    if (url === '/api/picto/words/reset-all') return json({ RESET: 1 })
    if (String(url).startsWith('/api/picto/words/') && url.endsWith('/reset')) return json({ AVAILABLE: true })
    if (String(url).startsWith('/api/picto/words/') && m === 'DELETE') return json({ DELETED: 'x' })
    if (url === '/api/categories') return json([])
    return json({})
  })
}

describe('PoolPage — instance picto', () => {
  it('titres « Pool Picto », colonne « Mot », aucune colonne « Reponse », aucun champ réponse', async () => {
    global.fetch = mockFetch()
    render(<PoolPage instance="picto" />)
    await waitFor(() => expect(screen.getByText('chat')).toBeTruthy())
    expect(screen.getAllByText('Pool Picto').length).toBeGreaterThan(0)
    expect(screen.getByRole('columnheader', { name: 'Mot' })).toBeTruthy()
    expect(screen.queryByRole('columnheader', { name: 'Reponse' })).toBeNull()
    expect(screen.queryByText('Reponse')).toBeNull()
    expect(global.fetch).toHaveBeenCalledWith('/api/picto/words')
    expect(global.fetch).not.toHaveBeenCalledWith('/api/rafale/questions')
  })

  it('compteur au singulier/pluriel du nom « mot »', async () => {
    global.fetch = mockFetch()
    render(<PoolPage instance="picto" />)
    await waitFor(() => expect(screen.getByText(/2 mots · 1 utilisee · 1 disponible/)).toBeTruthy())
  })

  it('remise à disponible : chemins /api/picto/words/{id}/reset et /reset-all', async () => {
    global.fetch = mockFetch()
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    render(<PoolPage instance="picto" />)
    await waitFor(() => expect(screen.getByText('maison')).toBeTruthy())
    fireEvent.click(screen.getByTitle('Remettre disponible'))
    await waitFor(() => expect(global.fetch).toHaveBeenCalledWith('/api/picto/words/p-002/reset', { method: 'POST' }))
    fireEvent.click(screen.getByText(/Remettre tout disponible/))
    await waitFor(() => expect(global.fetch).toHaveBeenCalledWith('/api/picto/words/reset-all', { method: 'POST' }))
  })

  it('suppression : DELETE /api/picto/words/{id}, confirmation « Pool Picto »', async () => {
    global.fetch = mockFetch()
    const confirm = vi.spyOn(window, 'confirm').mockReturnValue(true)
    render(<PoolPage instance="picto" />)
    await waitFor(() => expect(screen.getByText('chat')).toBeTruthy())
    fireEvent.click(screen.getAllByTitle('Supprimer')[0])
    await waitFor(() => expect(global.fetch).toHaveBeenCalledWith('/api/picto/words/p-001', { method: 'DELETE' }))
    expect(confirm).toHaveBeenCalledWith('Supprimer ce mot du Pool Picto ?')
  })
})

describe('PoolPage — instance par défaut', () => {
  it('sans prop : instance rafale, chemin /api/rafale/questions et colonne « Reponse »', async () => {
    global.fetch = vi.fn((url) => json(url === '/api/rafale/questions' ? { QUESTIONS: [{ ID: 'r-1', QUESTION: 'Q', ANSWER: 'A', CATEGORY: 'HISTORY', DIFFICULTY: 1, USED: false }], TOTAL: 1 } : []))
    render(<PoolPage />)
    await waitFor(() => expect(screen.getByRole('columnheader', { name: 'Reponse' })).toBeTruthy())
    expect(global.fetch).toHaveBeenCalledWith('/api/rafale/questions')
  })
})

describe('PoolPage — Pool Picto : collage d\'une liste (M1 A)', () => {
  function mockBatch(created, refused) {
    return vi.fn((url, options = {}) => {
      if (url === '/api/picto/words/batch') return json({ CREATED: created, REFUSED: refused })
      if (url === '/api/picto/words') return json({ QUESTIONS: WORDS, TOTAL: 2 })
      return json([])
    })
  }
  const openList = async () => {
    await waitFor(() => expect(screen.getByText('chat')).toBeTruthy())
    fireEvent.click(screen.getByRole('button', { name: 'Une liste' }))
    return screen.getByTestId('pool-list-form')
  }

  it('« Un mot » / « Une liste » proposés pour Picto, absents pour le Pool Speedy', async () => {
    global.fetch = mockBatch([], [])
    const { unmount } = render(<PoolPage instance="picto" />)
    await waitFor(() => expect(screen.getByRole('button', { name: 'Une liste' })).toBeTruthy())
    unmount()
    global.fetch = vi.fn((u) => json(u === '/api/categories' ? [] : { QUESTIONS: [], TOTAL: 0 }))
    render(<PoolPage instance="rafale" />)
    await waitFor(() => expect(screen.queryByRole('button', { name: 'Une liste' })).toBeNull())
  })

  it('collage : POST /words/batch {CATEGORY, DIFFICULTY, WORDS}, compte rendu des ajoutés et des refus listés', async () => {
    global.fetch = mockBatch([{ ID: 'p-3', QUESTION: 'Tortue' }, { ID: 'p-4', QUESTION: 'Lion' }], [{ QUESTION: 'chat', CODE: 'DUPLICATE' }, { QUESTION: 'a/b', CODE: 'WORD_INVALID' }])
    render(<PoolPage instance="picto" />)
    const form = await openList()
    fireEvent.change(within(form).getByRole('textbox'), { target: { value: 'Tortue\n  Lion  \n\nchat\na/b' } })
    expect(within(form).getByRole('button', { name: 'Ajouter les 4 lignes' })).toBeTruthy()
    fireEvent.click(within(form).getByTitle('Geographie'))
    fireEvent.click(within(form).getByRole('button', { name: 'Ajouter les 4 lignes' }))
    await waitFor(() => expect(screen.getByText(/2 ajoutés/)).toBeTruthy())
    const call = global.fetch.mock.calls.find(c => c[0] === '/api/picto/words/batch')
    expect(JSON.parse(call[1].body)).toEqual({ CATEGORY: 'GEOGRAPHY', DIFFICULTY: 1, WORDS: ['Tortue', 'Lion', 'chat', 'a/b'] })
    expect(screen.getByText('2 refusés')).toBeTruthy()
    expect(screen.getByText('« chat » : déjà dans la catégorie')).toBeTruthy()
    expect(screen.getByText('« a/b » : mot non valide')).toBeTruthy()
    // Les refusés restent dans la zone pour correction
    expect(within(form).getByRole('textbox').value).toBe('chat\na/b')
  })

  it('catégorie manquante : aucun envoi', async () => {
    global.fetch = mockBatch([], [])
    render(<PoolPage instance="picto" />)
    const form = await openList()
    fireEvent.change(within(form).getByRole('textbox'), { target: { value: 'Tortue' } })
    fireEvent.click(within(form).getByRole('button', { name: 'Ajouter les 1 ligne' }))
    expect(await screen.findByText('Selectionnez une categorie.')).toBeTruthy()
    expect(global.fetch.mock.calls.some(c => c[0] === '/api/picto/words/batch')).toBe(false)
  })
})

describe('PoolPage — Pool Picto : manche Picto en cours', () => {
  it('écritures, remises à zéro et IA grisées avec message explicite', async () => {
    useOptionalGame.mockReturnValue({ gameState: { phase: 'STARTED', question: { TYPE: 'PICTO' } }, aiJob: null })
    global.fetch = mockFetch()
    render(<PoolPage instance="picto" />)
    await waitFor(() => expect(screen.getByText('chat')).toBeTruthy())
    expect(screen.getByText(/Manche Picto en cours : le pool ne peut pas être modifié/)).toBeTruthy()
    screen.getAllByTitle('Supprimer').forEach(b => expect(b.disabled).toBe(true))
    expect(screen.getByTitle('Remettre disponible').disabled).toBe(true)
    expect(screen.getByText('Ajouter').closest('button').disabled).toBe(true)
    useOptionalGame.mockReturnValue(null)
  })
})
