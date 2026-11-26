import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import PoolAIGenerateModal from './PoolAIGenerateModal'

// #324 (v13.1.0, LOT-2 T13 générique) — modale IA paramétrée : paliers, titre et libellés par instance.
const props = (o = {}) => ({
  onClose: vi.fn(), apiKeyConfigured: true, provider: 'anthropic', aiJob: null, onCancelGeneration: vi.fn(),
  categories: [{ key: 'HISTORY', name: 'Histoire', color: '#f97316' }], questions: [], maxQuestions: 200, ...o,
})
const renderModal = (p) => render(<MemoryRouter><PoolAIGenerateModal {...p} /></MemoryRouter>)

describe('PoolAIGenerateModal', () => {
  it('instance picto : paliers 10 / 20 / 50 seulement, titre « mots Picto »', () => {
    renderModal(props({ instance: 'picto' }))
    ;[10, 20, 50].forEach((n) => expect(screen.getByRole('button', { name: String(n) })).toBeTruthy())
    ;[100, 200].forEach((n) => expect(screen.queryByRole('button', { name: String(n) })).toBeNull())
    expect(screen.getAllByText(/Générer des mots pour le Pool Picto/).length).toBeGreaterThan(0)
    expect(screen.getByText("Ce qu'on ajoute au Pool Picto")).toBeTruthy()
  })
  it('instance par défaut (rafale) : paliers 10..200 et libellés « réservoir » inchangés', () => {
    renderModal(props())
    ;[10, 20, 50, 100, 200].forEach((n) => expect(screen.getByRole('button', { name: String(n) })).toBeTruthy())
    expect(screen.getByText("Ce qu'on ajoute au réservoir")).toBeTruthy()
    expect(screen.getAllByText(/Générer des questions pour le Pool Speedy/).length).toBeGreaterThan(0)
  })
  it('maxQuestions filtre aussi les paliers Picto', () => {
    renderModal(props({ instance: 'picto', maxQuestions: 20 }))
    expect(screen.queryByRole('button', { name: '50' })).toBeNull()
    expect(screen.getByRole('button', { name: '20' })).toBeTruthy()
  })
})
