import { describe, it, expect, vi } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import PoolPage from './PoolPage'

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
