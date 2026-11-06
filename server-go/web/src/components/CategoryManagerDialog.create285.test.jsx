import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react'
import CategoryManagerDialog from './CategoryManagerDialog'

// ---------------------------------------------------------------------------
// #285 (LOT-4, retour QUALIF) — la création d'une catégorie vit dans CategoryManagerDialog (plus dans CategoryPicker).
// Conventions (test-writer ↔ dev-frontend) : erreurs du formulaire = role="alert" dans le groupe ; formulaire = role="group" nommé /nouvelle catégorie/i ; dedans : champ texte
// (placeholder /Nom de la catégorie/i), pastille de couleur (bouton aria-haspopup /couleur/i), <input type="file">,
// bouton /Ajouter|Créer/i. POST /api/categories en FormData {name, file, color}. Contrat : http-endpoints.md §POST.
// Le dialogue est rendu directement (props categories/onClose/onChanged) : indépendant de son point d'entrée.
// ---------------------------------------------------------------------------

vi.mock('./CategoryManagerDialog.css', () => ({}))
vi.mock('./CategoryColorSwatch.css', () => ({}))

const CATS = [
  { key: 'GEOGRAPHY', name: 'Geographie', imageURL: '', color: '#3b82f6', isCustom: false, questionCount: 0, rafaleCount: 0 },
  { key: 'CINEMA', name: 'Cinéma', imageURL: '/files/categories/CINEMA.png', color: '#0ea5e9', isCustom: true, questionCount: 0, rafaleCount: 0 },
]

let posts
let postResponse
beforeEach(() => {
  posts = []
  postResponse = () => ({ ok: true, status: 200, json: async () => ({ key: 'MUSIQUE', name: 'Musique', color: '#14b8a6', isCustom: true }) })
  global.fetch = vi.fn(async (url, opts = {}) => {
    if ((opts.method || 'GET') === 'POST' && url === '/api/categories') {
      posts.push(opts)
      return postResponse()
    }
    return { ok: true, status: 200, json: async () => [] }
  })
})
afterEach(() => vi.restoreAllMocks())

function setup(extra = {}) {
  const onChanged = vi.fn()
  render(<CategoryManagerDialog categories={CATS} onClose={() => {}} onChanged={onChanged} {...extra} />)
  const form = screen.getByRole('group', { name: /nouvelle catégorie/i })
  return { form, onChanged }
}
const png = () => new File([new Uint8Array([137, 80, 78, 71])], 'm.png', { type: 'image/png' })
const fill = (form, { name = 'Musique', file = png() } = {}) => {
  if (name !== null) fireEvent.change(within(form).getByPlaceholderText(/Nom de la catégorie/i), { target: { value: name } })
  if (file) fireEvent.change(form.querySelector('input[type="file"]'), { target: { files: [file] } })
}
const submit = (form) => fireEvent.click(within(form).getByRole('button', { name: /Ajouter|Créer/i }))

describe('CategoryManagerDialog — création (#285)', () => {
  it('le dialogue propose un formulaire de création : nom, pastille de couleur, image, bouton', () => {
    const { form } = setup()
    expect(within(form).getByPlaceholderText(/Nom de la catégorie/i)).toBeInTheDocument()
    expect(within(form).getByRole('button', { name: /couleur/i })).toHaveAttribute('aria-haspopup')
    expect(form.querySelector('input[type="file"]')).not.toBeNull()
    expect(within(form).getByRole('button', { name: /Ajouter|Créer/i })).toBeInTheDocument()
  })

  it('nom obligatoire : aucun POST, message sur le nom', () => {
    const { form } = setup()
    fill(form, { name: '   ' })
    submit(form)
    expect(posts).toHaveLength(0)
    expect(within(form).getByRole('alert')).toHaveTextContent(/nom/i)
  })

  it('image obligatoire : aucun POST, message sur l\'image', () => {
    const { form } = setup()
    fill(form, { name: 'Musique', file: null })
    submit(form)
    expect(posts).toHaveLength(0)
    expect(within(form).getByRole('alert')).toHaveTextContent(/image/i)
  })

  it('valider envoie en POST /api/categories un FormData {name, file, color #rrggbb}, sans Content-Type posé à la main', async () => {
    const { form } = setup()
    const file = png()
    fill(form, { name: ' Musique ', file })
    submit(form)
    await waitFor(() => expect(posts).toHaveLength(1))
    const body = posts[0].body
    expect(body).toBeInstanceOf(FormData)
    expect(body.get('name')).toBe('Musique')
    expect(body.get('file')).toBe(file)
    expect(body.get('color')).toMatch(/^#[0-9a-f]{6}$/i)
    expect(posts[0].headers?.['Content-Type']).toBeUndefined()
  })

  it('couleur choisie à la pastille : envoyée telle quelle ; « aucune » ⇒ color vide ou absent', async () => {
    const { form } = setup()
    fill(form)
    fireEvent.click(within(form).getByRole('button', { name: /couleur/i }))
    fireEvent.click(screen.getByRole('button', { name: /aucune/i }))
    submit(form)
    await waitFor(() => expect(posts).toHaveLength(1))
    expect(posts[0].body.get('color') || '').toBe('')
  })

  it('succès : le formulaire est vidé, sans refetch de repli', async () => {
    const { form, onChanged } = setup()
    fill(form)
    submit(form)
    await waitFor(() => expect(posts).toHaveLength(1))
    await waitFor(() => expect(within(form).getByPlaceholderText(/Nom de la catégorie/i)).toHaveValue(''))
    expect(onChanged).not.toHaveBeenCalled()
  })

  it('400 reserved name : « nom réservé », formulaire conservé', async () => {
    postResponse = () => ({ ok: false, status: 400, json: async () => ({ error: 'reserved name' }), text: async () => 'reserved name' })
    const { form } = setup()
    fill(form, { name: 'CON' })
    submit(form)
    expect(await screen.findByText(/nom réservé/i)).toBeInTheDocument()
    expect(within(form).getByPlaceholderText(/Nom de la catégorie/i)).toHaveValue('CON')
  })

  it('409 key already exists : « existe déjà »', async () => {
    postResponse = () => ({ ok: false, status: 409, json: async () => ({ error: 'key already exists' }), text: async () => 'key already exists' })
    const { form } = setup()
    fill(form, { name: 'Cinema' })
    submit(form)
    expect(await screen.findByText(/existe déjà/i)).toBeInTheDocument()
  })

  it('409 game in progress : « Impossible pendant une partie » et création désactivée', async () => {
    postResponse = () => ({ ok: false, status: 409, json: async () => ({ error: 'game in progress' }), text: async () => 'game in progress' })
    const { form } = setup()
    fill(form)
    submit(form)
    expect(await screen.findByText(/Impossible pendant une partie/i)).toBeInTheDocument()
    expect(within(form).getByRole('button', { name: /Ajouter|Créer/i })).toBeDisabled()
  })
})
