import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import CategoryPicker from './CategoryPicker'

vi.mock('./CategoryPicker.css', () => ({}))
vi.mock('./CategoryBadge.css', () => ({}))

// #285 — création enrichie : nom + icône (obligatoire) + couleur (pastille, défaut = 1ʳᵉ teinte libre de la palette).
// Maquette §01. Les tests existants de création (CategoryPicker.create.test.jsx) restent verts.
let posted
beforeEach(() => {
  posted = null
  global.fetch = vi.fn(async (url, opts = {}) => {
    if ((opts.method || 'GET') === 'POST' && url === '/api/categories') {
      posted = opts.body
      return { ok: true, status: 200, json: async () => ({ key: 'MUSIQUE', name: 'Musique', imageURL: '/files/categories/MUSIQUE.png', color: '#14b8a6', isCustom: true }) }
    }
    return { ok: true, status: 200, json: async () => [] }
  })
})

const open = () => {
  render(<CategoryPicker mode="single" value="" onChange={() => {}} customCategories={[]} allowCreate onRefetchCategories={() => {}} />)
  fireEvent.click(screen.getByRole('button', { name: /Créer|\+/ }))
}

describe('CategoryPicker — création avec couleur (#285)', () => {
  it('le formulaire propose une pastille de couleur (bouton aria-haspopup)', () => {
    open()
    expect(screen.getByRole('button', { name: /couleur/i })).toHaveAttribute('aria-haspopup')
  })

  it('valider envoie name + file + color (la teinte proposée par défaut est une couleur #rrggbb)', async () => {
    open()
    fireEvent.change(screen.getByPlaceholderText(/Nom de la catégorie/i), { target: { value: 'Musique' } })
    const input = document.querySelector('input[type="file"]')
    const file = new File([new Uint8Array([137, 80, 78, 71])], 'm.png', { type: 'image/png' })
    fireEvent.change(input, { target: { files: [file] } })
    fireEvent.click(screen.getByRole('button', { name: /Valider|Ajouter|Créer la catégorie/i }))
    await waitFor(() => expect(posted).toBeInstanceOf(FormData))
    expect(posted.get('name')).toBe('Musique')
    expect(posted.get('file')).toBe(file)
    expect(posted.get('color')).toMatch(/^#[0-9a-f]{6}$/i)
  })

  it('« aucune » couleur choisie ⇒ color "" (ou absent)', async () => {
    open()
    fireEvent.change(screen.getByPlaceholderText(/Nom de la catégorie/i), { target: { value: 'Musique' } })
    fireEvent.change(document.querySelector('input[type="file"]'), { target: { files: [new File(['x'], 'm.png', { type: 'image/png' })] } })
    fireEvent.click(screen.getByRole('button', { name: /couleur/i }))
    fireEvent.click(screen.getByRole('button', { name: /aucune/i }))
    fireEvent.click(screen.getByRole('button', { name: /Valider|Ajouter|Créer la catégorie/i }))
    await waitFor(() => expect(posted).toBeInstanceOf(FormData))
    expect(posted.get('color') || '').toBe('')
  })

  it('refus 409 « partie en cours » : message « Impossible pendant une partie » dans le formulaire', async () => {
    global.fetch = vi.fn(async (url, opts = {}) => ((opts.method || 'GET') === 'POST'
      ? { ok: false, status: 409, json: async () => ({ error: 'game in progress' }), text: async () => 'game in progress' }
      : { ok: true, status: 200, json: async () => [] }))
    open()
    fireEvent.change(screen.getByPlaceholderText(/Nom de la catégorie/i), { target: { value: 'Musique' } })
    fireEvent.change(document.querySelector('input[type="file"]'), { target: { files: [new File(['x'], 'm.png', { type: 'image/png' })] } })
    fireEvent.click(screen.getByRole('button', { name: /Valider|Ajouter|Créer la catégorie/i }))
    expect(await screen.findByText(/Impossible pendant une partie/i)).toBeInTheDocument()
  })
})
