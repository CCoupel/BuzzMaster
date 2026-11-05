import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react'
import CategoryPicker from './CategoryPicker'

// ---------------------------------------------------------------------------
// #285 (v11.4.0, LOT-4) — CategoryPicker « ⚙ Gérer… » + CategoryManagerDialog
// (contracts/http-endpoints.md §Catégories « Frontend », maquette admin_category_manager__rename_color_delete §01/§02).
// Le dialogue est atteint par le VRAI parcours utilisateur (clic sur « ⚙ Gérer… » du sélecteur) : ses props internes
// ne sont pas figées par le contrat. Conventions testées (à respecter par dev-frontend) :
//   - bouton « Gérer… » (name /Gérer/) à côté de « + Créer » ; dialogue role="dialog" ;
//   - pastille de couleur = bouton `aria-haspopup`, nom /couleur/i ; palette : 12 boutons dont le nom accessible
//     (aria-label ou title) est la teinte `#rrggbb`, plus un bouton « aucune » ;
//   - ✎ = bouton de nom /renommer/i ; 🗑 = bouton de nom /supprimer/i (désactivé sur les intégrées, 🔒) ;
//   - icône = <input type="file" accept="…png…jpg…webp"> présent UNIQUEMENT sur les personnalisées.
// ---------------------------------------------------------------------------

vi.mock('./CategoryPicker.css', () => ({}))
vi.mock('./CategoryBadge.css', () => ({}))

const API = () => [
  { key: 'GEOGRAPHY', name: 'Geographie', imageURL: '', color: '#3b82f6', isCustom: false, questionCount: 5, rafaleCount: 2 },
  { key: 'HISTORY', name: 'Histoire', imageURL: '', color: '#eab308', isCustom: false, questionCount: 0, rafaleCount: 0 },
  { key: 'CINEMA', name: 'Cinéma', imageURL: '/files/categories/CINEMA.png', color: '#0ea5e9', isCustom: true, questionCount: 12, rafaleCount: 30 },
  { key: 'VIDE', name: 'Vide', imageURL: '/files/categories/VIDE.png', color: '', isCustom: true, questionCount: 0, rafaleCount: 0 },
]

let calls
function installFetch({ onWrite } = {}) {
  calls = []
  global.fetch = vi.fn(async (url, opts = {}) => {
    const method = opts.method || 'GET'
    let body = null
    if (typeof opts.body === 'string') { try { body = JSON.parse(opts.body) } catch { body = null } }
    calls.push({ method, url, body, raw: opts.body })
    const res = (status, json = {}) => ({ ok: status < 400, status, json: async () => json, text: async () => JSON.stringify(json) })
    if (method === 'GET' && url === '/api/categories') return res(200, API())
    const custom = onWrite?.({ method, url, body })
    if (custom) return custom(res)
    if (method === 'PUT' || method === 'DELETE' || method === 'POST') return res(200, { ok: true })
    return res(404, {})
  })
}
const writes = (m) => calls.filter((c) => c.method === m && c.url.startsWith('/api/categories/'))

async function openDialog() {
  render(<CategoryPicker mode="single" value="" onChange={() => {}} customCategories={API()} allowCreate onRefetchCategories={() => {}} />)
  fireEvent.click(screen.getByRole('button', { name: /Gérer/ }))
  const dlg = await screen.findByRole('dialog')
  return dlg
}
const rowOf = (dlg, label) => within(dlg).getByText(label).closest('tr')

beforeEach(() => installFetch())
afterEach(() => vi.restoreAllMocks())

describe('CategoryPicker — « ⚙ Gérer… » (#285)', () => {
  it('le bouton « Gérer… » est présent à côté de « + Créer » et ouvre le dialogue', async () => {
    const dlg = await openDialog()
    expect(dlg).toBeInTheDocument()
    expect(within(dlg).getByText(/Gérer les catégories/i)).toBeInTheDocument()
  })

  it('sans allowCreate (sélecteur de lecture) : pas de « Gérer… »', () => {
    render(<CategoryPicker mode="single" value="" onChange={() => {}} customCategories={API()} />)
    expect(screen.queryByRole('button', { name: /Gérer/ })).not.toBeInTheDocument()
  })

  it('mode filtre de la page Questions : « Gérer… » disponible', () => {
    render(<CategoryPicker variant="filter" value={new Set()} onToggle={() => {}} customCategories={API()} availableCategories={['CINEMA']} allowCreate onRefetchCategories={() => {}} />)
    expect(screen.getByRole('button', { name: /Gérer/ })).toBeInTheDocument()
  })

  it('fermer le dialogue (✕ « Fermer ») le retire', async () => {
    const dlg = await openDialog()
    fireEvent.click(within(dlg).getByRole('button', { name: /Fermer/ }))
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
  })
})

describe('CategoryManagerDialog — lignes (#285)', () => {
  it('une ligne par catégorie : 8 intégrées + personnalisées, nom et compteur de questions', async () => {
    const dlg = await openDialog()
    const row = rowOf(dlg, 'Cinéma')
    expect(within(row).getByText('12')).toBeInTheDocument()
    expect(rowOf(dlg, 'Geographie')).toBeTruthy()
  })

  it('intégrées : 🗑 désactivé (🔒), aucun champ d\'envoi d\'icône ; personnalisées : icône modifiable', async () => {
    const dlg = await openDialog()
    const builtin = rowOf(dlg, 'Histoire')
    expect(within(builtin).getByRole('button', { name: /Supprimer/i })).toBeDisabled()
    expect(builtin.querySelector('input[type="file"]')).toBeNull()
    const custom = rowOf(dlg, 'Cinéma')
    const input = custom.querySelector('input[type="file"]')
    expect(input).not.toBeNull()
    expect(input.getAttribute('accept')).toMatch(/png/i)
    expect(input.getAttribute('accept')).toMatch(/webp/i)
    expect(within(custom).getByRole('button', { name: /Supprimer/i })).not.toBeDisabled()
  })
})

describe('CategoryManagerDialog — couleur (#285)', () => {
  it('la pastille est un bouton aria-haspopup ; la palette propose 12 teintes + « aucune »', async () => {
    const dlg = await openDialog()
    const swatch = within(rowOf(dlg, 'Cinéma')).getByRole('button', { name: /couleur/i })
    expect(swatch).toHaveAttribute('aria-haspopup')
    fireEvent.click(swatch)
    const hexes = screen.getAllByRole('button').filter((b) => /^#[0-9a-f]{6}$/i.test(b.getAttribute('aria-label') || b.getAttribute('title') || ''))
    expect(hexes.length).toBeGreaterThanOrEqual(12)
    expect(screen.getByRole('button', { name: /aucune/i })).toBeInTheDocument()
  })

  it('choisir une teinte ⇒ PUT /api/categories/CINEMA {color}', async () => {
    const dlg = await openDialog()
    fireEvent.click(within(rowOf(dlg, 'Cinéma')).getByRole('button', { name: /couleur/i }))
    const target = screen.getAllByRole('button').find((b) => /^#[0-9a-f]{6}$/i.test(b.getAttribute('aria-label') || b.getAttribute('title') || ''))
    const hex = (target.getAttribute('aria-label') || target.getAttribute('title')).toLowerCase()
    fireEvent.click(target)
    await waitFor(() => expect(writes('PUT')).toHaveLength(1))
    expect(writes('PUT')[0].url).toBe('/api/categories/CINEMA')
    expect(writes('PUT')[0].body).toEqual({ color: hex })
  })

  it('« aucune » ⇒ PUT {color:""}', async () => {
    const dlg = await openDialog()
    fireEvent.click(within(rowOf(dlg, 'Cinéma')).getByRole('button', { name: /couleur/i }))
    fireEvent.click(screen.getByRole('button', { name: /aucune/i }))
    await waitFor(() => expect(writes('PUT')).toHaveLength(1))
    expect(writes('PUT')[0].body).toEqual({ color: '' })
  })

  it('les catégories intégrées sont recolorables aussi (PUT sur la clé intégrée)', async () => {
    const dlg = await openDialog()
    fireEvent.click(within(rowOf(dlg, 'Histoire')).getByRole('button', { name: /couleur/i }))
    fireEvent.click(screen.getByRole('button', { name: /aucune/i }))
    await waitFor(() => expect(writes('PUT')).toHaveLength(1))
    expect(writes('PUT')[0].url).toBe('/api/categories/HISTORY')
  })
})

describe('CategoryManagerDialog — renommage et icône (#285)', () => {
  it('✎ renommage en ligne : Entrée ⇒ PUT {name}', async () => {
    const dlg = await openDialog()
    fireEvent.click(within(rowOf(dlg, 'Cinéma')).getByRole('button', { name: /renommer/i }))
    const input = within(dlg).getByDisplayValue('Cinéma')
    fireEvent.change(input, { target: { value: 'Septième art' } })
    fireEvent.keyDown(input, { key: 'Enter', code: 'Enter' })
    await waitFor(() => expect(writes('PUT')).toHaveLength(1))
    expect(writes('PUT')[0].url).toBe('/api/categories/CINEMA')
    expect(writes('PUT')[0].body).toEqual({ name: 'Septième art' })
  })

  it('Échap annule le renommage : aucun PUT', async () => {
    const dlg = await openDialog()
    fireEvent.click(within(rowOf(dlg, 'Cinéma')).getByRole('button', { name: /renommer/i }))
    const input = within(dlg).getByDisplayValue('Cinéma')
    fireEvent.change(input, { target: { value: 'Autre' } })
    fireEvent.keyDown(input, { key: 'Escape', code: 'Escape' })
    expect(writes('PUT')).toHaveLength(0)
  })

  it('envoyer une icône ⇒ POST /api/categories/CINEMA/icon (multipart, champ « file »)', async () => {
    const dlg = await openDialog()
    const input = rowOf(dlg, 'Cinéma').querySelector('input[type="file"]')
    const file = new File([new Uint8Array([137, 80, 78, 71])], 'new.png', { type: 'image/png' })
    fireEvent.change(input, { target: { files: [file] } })
    await waitFor(() => expect(writes('POST')).toHaveLength(1))
    expect(writes('POST')[0].url).toBe('/api/categories/CINEMA/icon')
    expect(writes('POST')[0].raw).toBeInstanceOf(FormData)
    expect(writes('POST')[0].raw.get('file')).toBe(file)
  })
})

describe('CategoryManagerDialog — suppression (#285)', () => {
  const inUse = (res) => res(409, { error: 'category in use', questions: 12, rafale: 30, history: { events: 3, points: 1240 } })

  it('catégorie inutilisée : DELETE sans mode ⇒ supprimée directement (pas de dialogue de confirmation)', async () => {
    installFetch()
    const dlg = await openDialog()
    fireEvent.click(within(rowOf(dlg, 'Vide')).getByRole('button', { name: /Supprimer/i }))
    await waitFor(() => expect(writes('DELETE')).toHaveLength(1))
    expect(writes('DELETE')[0].url).toBe('/api/categories/VIDE')
    expect(writes('DELETE')[0].body?.mode).toBeUndefined()
    expect(screen.queryByText(/Supprimer la catégorie/)).not.toBeInTheDocument()
  })

  it('409 category in use ⇒ dialogue : « Garder les questions » présélectionné, encart 📊 de l\'historique, bouton désactivé tant que la case n\'est pas cochée', async () => {
    installFetch({ onWrite: ({ method, body }) => (method === 'DELETE' && !body?.mode ? inUse : null) })
    const dlg = await openDialog()
    fireEvent.click(within(rowOf(dlg, 'Cinéma')).getByRole('button', { name: /Supprimer/i }))
    await screen.findByText(/Supprimer « Cinéma »/)
    expect(screen.getByRole('radio', { name: /Garder les questions, sans catégorie/i })).toBeChecked()
    expect(screen.getByRole('radio', { name: /Supprimer aussi les questions/i })).not.toBeChecked()
    expect(screen.getByText(/1.?240/)).toBeInTheDocument() // points d'historique regroupés
    const confirm = screen.getByRole('button', { name: /Supprimer la catégorie/i })
    expect(confirm).toBeDisabled()
    fireEvent.click(screen.getByRole('checkbox'))
    expect(confirm).not.toBeDisabled()
  })

  it('confirmer en mode reassign ⇒ DELETE {mode:"reassign"}', async () => {
    installFetch({ onWrite: ({ method, body }) => (method === 'DELETE' && !body?.mode ? inUse : null) })
    const dlg = await openDialog()
    fireEvent.click(within(rowOf(dlg, 'Cinéma')).getByRole('button', { name: /Supprimer/i }))
    await screen.findByText(/Supprimer « Cinéma »/)
    fireEvent.click(screen.getByRole('checkbox'))
    fireEvent.click(screen.getByRole('button', { name: /Supprimer la catégorie/i }))
    await waitFor(() => expect(writes('DELETE')).toHaveLength(2))
    expect(writes('DELETE')[1].body).toEqual({ mode: 'reassign' })
  })

  it('mode « Supprimer aussi les questions » ⇒ DELETE {mode:"delete"} ; case de confirmation obligatoire même sans historique', async () => {
    installFetch({ onWrite: ({ method, body }) => (method === 'DELETE' && !body?.mode
      ? (res) => res(409, { error: 'category in use', questions: 12, rafale: 30, history: { events: 0, points: 0 } })
      : null) })
    const dlg = await openDialog()
    fireEvent.click(within(rowOf(dlg, 'Cinéma')).getByRole('button', { name: /Supprimer/i }))
    await screen.findByText(/Supprimer « Cinéma »/)
    fireEvent.click(screen.getByRole('radio', { name: /Supprimer aussi les questions/i }))
    const confirm = screen.getByRole('button', { name: /Supprimer la catégorie/i })
    expect(confirm).toBeDisabled()
    fireEvent.click(screen.getByRole('checkbox'))
    fireEvent.click(confirm)
    await waitFor(() => expect(writes('DELETE')).toHaveLength(2))
    expect(writes('DELETE')[1].body).toEqual({ mode: 'delete' })
  })

  it('mode reassign sans historique : aucune case obligatoire', async () => {
    installFetch({ onWrite: ({ method, body }) => (method === 'DELETE' && !body?.mode
      ? (res) => res(409, { error: 'category in use', questions: 2, rafale: 0, history: { events: 0, points: 0 } })
      : null) })
    const dlg = await openDialog()
    fireEvent.click(within(rowOf(dlg, 'Cinéma')).getByRole('button', { name: /Supprimer/i }))
    await screen.findByText(/Supprimer « Cinéma »/)
    expect(screen.getByRole('button', { name: /Supprimer la catégorie/i })).not.toBeDisabled()
  })

  it('Annuler ferme la confirmation sans rien supprimer', async () => {
    installFetch({ onWrite: ({ method, body }) => (method === 'DELETE' && !body?.mode ? inUse : null) })
    const dlg = await openDialog()
    fireEvent.click(within(rowOf(dlg, 'Cinéma')).getByRole('button', { name: /Supprimer/i }))
    await screen.findByText(/Supprimer « Cinéma »/)
    fireEvent.click(screen.getByRole('button', { name: /^Annuler$/ }))
    await waitFor(() => expect(screen.queryByText(/Supprimer « Cinéma »/)).not.toBeInTheDocument())
    expect(writes('DELETE')).toHaveLength(1)
  })
})

describe('CategoryManagerDialog — 409 « partie en cours » sur toute action (#285)', () => {
  const locked = () => (res) => res(409, { error: 'game in progress' })

  it.each([
    ['couleur', async (dlg) => {
      fireEvent.click(within(rowOf(dlg, 'Cinéma')).getByRole('button', { name: /couleur/i }))
      fireEvent.click(screen.getByRole('button', { name: /aucune/i }))
    }],
    ['renommage', async (dlg) => {
      fireEvent.click(within(rowOf(dlg, 'Cinéma')).getByRole('button', { name: /renommer/i }))
      const input = within(dlg).getByDisplayValue('Cinéma')
      fireEvent.change(input, { target: { value: 'X' } })
      fireEvent.keyDown(input, { key: 'Enter', code: 'Enter' })
    }],
    ['icône', async (dlg) => {
      const input = rowOf(dlg, 'Cinéma').querySelector('input[type="file"]')
      fireEvent.change(input, { target: { files: [new File(['x'], 'a.png', { type: 'image/png' })] } })
    }],
    ['suppression', async (dlg) => {
      fireEvent.click(within(rowOf(dlg, 'Vide')).getByRole('button', { name: /Supprimer/i }))
    }],
  ])('%s refusée en partie ⇒ « Impossible pendant une partie », dialogue ouvert', async (_n, act) => {
    installFetch({ onWrite: ({ method }) => (method !== 'GET' ? locked() : null) })
    const dlg = await openDialog()
    await act(dlg)
    expect(await screen.findByText(/Impossible pendant une partie/i)).toBeInTheDocument()
    expect(screen.getByRole('dialog')).toBeInTheDocument()
  })
})
