import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import CategoryPicker from './CategoryPicker'

// ---------------------------------------------------------------------------
// CategoryPicker — création inline de catégorie (allowCreate, mode single).
// #221 (LOT-4, v11.2.2) : transposition des 12 cas `allowCreate` de l'ancien
// CategorySelector.test.jsx (#97/#100), supprimé dans le même commit. La logique
// handleCreate est reprise À L'IDENTIQUE dans CategoryPicker : mêmes libellés,
// mêmes erreurs, même ordre onRefetchCategories → onChange(created.key).
// Fichier scindé de CategoryPicker.test.jsx pour rester < 50 cas par fichier.
// Cible : title / placeholder / texte, jamais de classe CSS.
// ---------------------------------------------------------------------------

vi.mock('./CategoryPicker.css', () => ({}))

function renderSingle(overrides = {}) {
  const onChange = overrides.onChange ?? vi.fn()
  const utils = render(
    <CategoryPicker
      mode="single"
      value=""
      onChange={onChange}
      customCategories={[]}
      onRefetchCategories={overrides.onRefetchCategories}
      allowCreate={overrides.allowCreate}
    />
  )
  return { ...utils, onChange }
}

beforeEach(() => {
  vi.clearAllMocks()
})

// ---------------------------------------------------------------------------
// Création inline (#97/#100)
// ---------------------------------------------------------------------------

function fillAndSubmit(name = 'X', { withFile = true } = {}) {
  fireEvent.click(screen.getByTitle('Créer une catégorie'))
  fireEvent.change(screen.getByPlaceholderText('Nom de la catégorie...'), { target: { value: name } })
  if (withFile) {
    const file = new File(['x'], 'photo.png', { type: 'image/png' })
    fireEvent.change(document.querySelector('input[type="file"]'), { target: { files: [file] } })
  }
  fireEvent.click(screen.getByText('Valider'))
}

describe('CategoryPicker — allowCreate (single)', () => {
  it('allowCreate=true : le bouton « + » est présent', () => {
    renderSingle({ allowCreate: true })
    expect(screen.getByTitle('Créer une catégorie')).toBeInTheDocument()
  })

  it('le bouton « + » n\'est pas un radio de catégorie (compte des radios inchangé)', () => {
    renderSingle({ allowCreate: true })
    expect(screen.getAllByRole('radio')).toHaveLength(8)
    expect(screen.getByTitle('Créer une catégorie')).not.toHaveAttribute('role', 'radio')
  })

  it('allowCreate=false : AUCUN bouton « + » rendu', () => {
    renderSingle({ allowCreate: false })
    expect(screen.queryByTitle('Créer une catégorie')).not.toBeInTheDocument()
  })

  it('clic sur « + » affiche le formulaire (input texte, input file, Valider/Annuler)', () => {
    renderSingle({ allowCreate: true })
    fireEvent.click(screen.getByTitle('Créer une catégorie'))

    expect(screen.getByPlaceholderText('Nom de la catégorie...')).toBeInTheDocument()
    expect(screen.getByText(/Choisir une image/)).toBeInTheDocument()
    expect(screen.getByText('Valider')).toBeInTheDocument()
    expect(screen.getByText('Annuler')).toBeInTheDocument()
  })

  it('« Annuler » masque le formulaire et réinitialise le nom saisi', () => {
    renderSingle({ allowCreate: true })
    fireEvent.click(screen.getByTitle('Créer une catégorie'))
    fireEvent.change(screen.getByPlaceholderText('Nom de la catégorie...'), { target: { value: 'Ma categorie' } })
    fireEvent.click(screen.getByText('Annuler'))

    expect(screen.queryByPlaceholderText('Nom de la catégorie...')).not.toBeInTheDocument()
    // Ré-ouvrir prouve que le nom a bien été vidé, pas juste masqué.
    fireEvent.click(screen.getByTitle('Créer une catégorie'))
    expect(screen.getByPlaceholderText('Nom de la catégorie...').value).toBe('')
  })

  it('touche Échap dans le champ nom masque le formulaire', () => {
    renderSingle({ allowCreate: true })
    fireEvent.click(screen.getByTitle('Créer une catégorie'))
    fireEvent.keyDown(screen.getByPlaceholderText('Nom de la catégorie...'), { key: 'Escape' })

    expect(screen.queryByPlaceholderText('Nom de la catégorie...')).not.toBeInTheDocument()
  })

  it('Valider sans nom : « Nom invalide », aucun fetch envoyé', async () => {
    global.fetch = vi.fn()
    renderSingle({ allowCreate: true })
    fireEvent.click(screen.getByTitle('Créer une catégorie'))
    fireEvent.click(screen.getByText('Valider'))

    expect(await screen.findByText('Nom invalide')).toBeInTheDocument()
    expect(global.fetch).not.toHaveBeenCalled()
  })

  it('Valider avec un nom mais sans image : « Image requise », aucun fetch envoyé', async () => {
    global.fetch = vi.fn()
    renderSingle({ allowCreate: true })
    fillAndSubmit('Ma categorie', { withFile: false })

    expect(await screen.findByText('Image requise')).toBeInTheDocument()
    expect(global.fetch).not.toHaveBeenCalled()
  })

  it('création réussie : POST /api/categories, puis onRefetchCategories AVANT onChange(created.key), formulaire refermé', async () => {
    const callOrder = []
    const onRefetchCategories = vi.fn(() => { callOrder.push('refetch') })
    const onChange = vi.fn((key) => { callOrder.push(`onChange:${key}`) })
    global.fetch = vi.fn(() => Promise.resolve({
      ok: true,
      status: 200,
      json: () => Promise.resolve({ key: 'NEW_CAT', name: 'Ma categorie' }),
    }))

    renderSingle({ allowCreate: true, onChange, onRefetchCategories })
    fillAndSubmit('Ma categorie')

    await waitFor(() => {
      expect(onChange).toHaveBeenCalledWith('NEW_CAT')
    })

    expect(global.fetch).toHaveBeenCalledWith('/api/categories', expect.objectContaining({ method: 'POST' }))
    const [, options] = global.fetch.mock.calls[0]
    expect(options.body).toBeInstanceOf(FormData)
    expect(callOrder).toEqual(['refetch', 'onChange:NEW_CAT'])
    expect(screen.queryByPlaceholderText('Nom de la catégorie...')).not.toBeInTheDocument()
  })

  it('erreur 409 (conflit) : « Cette catégorie existe déjà », formulaire reste ouvert', async () => {
    global.fetch = vi.fn(() => Promise.resolve({ ok: false, status: 409 }))
    renderSingle({ allowCreate: true })
    fillAndSubmit('Existe')

    expect(await screen.findByText('Cette catégorie existe déjà')).toBeInTheDocument()
    expect(screen.getByPlaceholderText('Nom de la catégorie...')).toBeInTheDocument()
  })

  it('erreur 400 (autre) : message générique « Nom invalide ou image non supportée »', async () => {
    global.fetch = vi.fn(() => Promise.resolve({ ok: false, status: 400 }))
    renderSingle({ allowCreate: true })
    fillAndSubmit('X')

    expect(await screen.findByText('Nom invalide ou image non supportée')).toBeInTheDocument()
  })

  it('rejet réseau (fetch throw) : « Erreur réseau »', async () => {
    global.fetch = vi.fn(() => Promise.reject(new Error('offline')))
    renderSingle({ allowCreate: true })
    fillAndSubmit('X')

    expect(await screen.findByText('Erreur réseau')).toBeInTheDocument()
  })

  it('sans onRefetchCategories fourni (prop optionnelle) : la création réussie n\'explose pas', async () => {
    const onChange = vi.fn()
    global.fetch = vi.fn(() => Promise.resolve({
      ok: true, status: 200, json: () => Promise.resolve({ key: 'NEW_CAT' }),
    }))
    renderSingle({ allowCreate: true, onChange, onRefetchCategories: undefined })
    fillAndSubmit('X')

    await waitFor(() => expect(onChange).toHaveBeenCalledWith('NEW_CAT'))
  })
})
