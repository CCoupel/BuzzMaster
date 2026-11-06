import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import RafalePoolAlert from './RafalePoolAlert'

// #282 (v11.4.0, LOT-2) — contracts/rafale.md §15.6 : avec l'option active (prop `alphabetical`),
// l'alerte dit « N questions · L lettres couvertes (A–Z sauf …) » ; sinon message inchangé.
const pool = (letters) => Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({ AVAILABLE: 48, USED: 2, TOTAL: 50, LETTERS: letters }) })
const ALL = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ'.split('')
const props = { categories: ['GEOGRAPHY'], difficulties: [1], roundTime: 120, questionTime: 3 }

beforeEach(() => vi.clearAllMocks())

describe('RafalePoolAlert — lettres couvertes (#282)', () => {
  it('option active : « 48 questions · 19 lettres couvertes (A–Z sauf J, K, Q, U, W, X, Y) »', async () => {
    const letters = ALL.filter(l => !['J', 'K', 'Q', 'U', 'W', 'X', 'Y'].includes(l))
    global.fetch = vi.fn(() => pool(letters))
    render(<RafalePoolAlert {...props} alphabetical />)
    await waitFor(() => expect(screen.getByText(/48 questions/)).toBeInTheDocument())
    expect(screen.getByText(/19 lettres couvertes/)).toBeInTheDocument()
    expect(screen.getByText(/sauf J, K, Q, U, W, X, Y/)).toBeInTheDocument()
  })

  it('toutes les lettres couvertes : pas de « sauf … »', async () => {
    global.fetch = vi.fn(() => pool(ALL))
    render(<RafalePoolAlert {...props} alphabetical />)
    await waitFor(() => expect(screen.getByText(/26 lettres couvertes/)).toBeInTheDocument())
    expect(screen.queryByText(/sauf/)).toBeNull()
  })

  it('option inactive (prop absente) : message inchangé, aucune mention de lettres', async () => {
    global.fetch = vi.fn(() => pool(ALL))
    const { container } = render(<RafalePoolAlert {...props} />)
    await waitFor(() => expect(screen.getByText(/48 questions/)).toBeInTheDocument())
    expect(container.textContent).not.toMatch(/lettres couvertes/)
  })

  it('LETTERS absent de la réponse (serveur ancien) : repli sans lettres, pas de plantage', async () => {
    global.fetch = vi.fn(() => Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({ AVAILABLE: 10, USED: 0, TOTAL: 10 }) }))
    const { container } = render(<RafalePoolAlert {...props} alphabetical />)
    await waitFor(() => expect(screen.getByText(/10 questions/)).toBeInTheDocument())
    expect(container.textContent).not.toMatch(/undefined|NaN/)
  })
})
