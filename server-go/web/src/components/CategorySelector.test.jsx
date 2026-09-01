import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import CategorySelector from './CategorySelector'

// ---------------------------------------------------------------------------
// CategorySelector — sélecteur de catégorie unique (icônes + création
// inline), extrait de QuestionsPage.jsx (#95/#97/#100) pour être partagé
// avec RafalePage.jsx (v8.0.0, #16/#197, bugfix cohérence UI, SHA e7d895a1).
//
// Ce fichier teste le COMPOSANT en isolation (son contrat de props :
// value/onChange/customCategories/onRefetchCategories/allowCreate/size),
// PAS son intégration dans une page particulière — QuestionsPage.v571.test.jsx
// couvre déjà cette même logique de création (400/409/réseau) mais
// UNIQUEMENT telle qu'exercée via QuestionsPage (une seule des DEUX
// consommatrices réelles du composant). Sans ce fichier, un futur troisième
// consommateur (ou une régression du contrat de props lui-même, ex.
// onRefetchCategories jamais appelé avant onChange) ne serait détecté par
// AUCUN test — QuestionsPage.v571.test.jsx ne peut structurellement pas le
// voir, il ne connaît que sa propre intégration. Les deux fichiers ne sont
// donc PAS redondants : couche composant ici, couche intégration page
// là-bas — même répartition que AnimMotionActions.test.jsx (composant) vs
// AnimConductPanel.test.jsx (câblage dans le panneau qui le monte).
// ---------------------------------------------------------------------------

vi.mock('./CategorySelector.css', () => ({}))

const CUSTOM_CATEGORIES = [
  { key: 'CUSTOM_1', name: 'Custom Un', isCustom: true, imageURL: '/img/custom1.png' },
]

function renderSelector(overrides = {}) {
  const onChange = overrides.onChange ?? vi.fn()
  const utils = render(
    <CategorySelector
      value={overrides.value ?? ''}
      onChange={onChange}
      customCategories={overrides.customCategories ?? []}
      onRefetchCategories={overrides.onRefetchCategories}
      allowCreate={overrides.allowCreate ?? true}
      size={overrides.size}
    />
  )
  return { ...utils, onChange }
}

beforeEach(() => {
  vi.clearAllMocks()
})

// ---------------------------------------------------------------------------
// Rendu / sélection
// ---------------------------------------------------------------------------

describe('CategorySelector — rendu et sélection', () => {
  it('rend un bouton par catégorie standard (8 catégories codées en dur)', () => {
    const { container } = renderSelector()
    // 8 standard + le bouton "+" = 9 boutons .category-btn
    expect(container.querySelectorAll('.category-btn').length).toBe(9)
  })

  it('rend aussi les catégories custom passées en props', () => {
    const { container } = renderSelector({ customCategories: CUSTOM_CATEGORIES })
    expect(screen.getByTitle('Custom Un')).toBeInTheDocument()
    expect(container.querySelectorAll('.category-btn').length).toBe(10) // 8 + 1 custom + "+"
  })

  it('clic sur une catégorie appelle onChange avec sa clé', () => {
    const { onChange } = renderSelector()
    fireEvent.click(screen.getByTitle('Histoire'))
    expect(onChange).toHaveBeenCalledWith('HISTORY')
  })

  it('re-cliquer sur la catégorie DÉJÀ sélectionnée appelle onChange(\'\') (toggle off)', () => {
    const { onChange } = renderSelector({ value: 'HISTORY' })
    fireEvent.click(screen.getByTitle('Histoire'))
    expect(onChange).toHaveBeenCalledWith('')
  })

  it('la catégorie sélectionnée porte la classe "active"', () => {
    renderSelector({ value: 'SCIENCE' })
    expect(screen.getByTitle('Sciences & Nature').className).toMatch(/\bactive\b/)
  })

  it('affiche le label de la catégorie sélectionnée sous la grille', () => {
    renderSelector({ value: 'SCIENCE' })
    expect(screen.getByText('Sciences & Nature')).toBeInTheDocument()
  })

  it('value="" (aucune sélection) : aucun bouton actif, aucun label affiché', () => {
    const { container } = renderSelector({ value: '' })
    expect(container.querySelector('.category-btn.active')).toBeNull()
    expect(container.querySelector('.category-label')).toBeNull()
  })
})

// ---------------------------------------------------------------------------
// Non-régression — doublon de catégories (retour utilisateur QUALIF
// 8.0.0.2, "le pool RAFALE liste 2 lignes identiques de catégorie", SHA
// ba960bdf). Cause racine : GET /api/categories renvoie les 8 catégories
// codées en dur EN MIROIR (isCustom:false, même clé que utils/
// categoryUtils.CATEGORIES) FUSIONNÉES avec les vraies catégories
// personnalisées (isCustom:true) — internal/server/http.go
// handleGetCategories (hardcodedCategories ++ custom, une seule réponse).
// Un appelant passant cette réponse BRUTE (non filtrée) à `customCategories`
// faisait alors doublonner chaque catégorie standard : une fois via
// `Object.keys(CATEGORIES)`, une fois via `customCategories.map(c => c.key)`
// (qui recontenait la même clé, mirror isCustom:false). Le composant filtre
// désormais lui-même (`customCategories.filter(c => c.isCustom)`) — ce bloc
// prouve que la classe de bug ne peut plus se reproduire, quel que soit
// l'appelant.
// ---------------------------------------------------------------------------

// Reproduit fidèlement la forme RÉELLE de GET /api/categories
// (internal/server/http.go, CategoryInfo{Key,Name,ImageURL,IsCustom,Color})
// : les 8 catégories codées en dur en miroir (isCustom:false) + une vraie
// catégorie personnalisée (isCustom:true) — PAS pré-filtrée, exactement ce
// qu'un appelant négligent passerait tel quel.
const RAW_API_CATEGORIES_RESPONSE = [
  { key: 'GEOGRAPHY', name: 'Geographie', imageURL: '', isCustom: false, color: '#3b82f6' },
  { key: 'ENTERTAINMENT', name: 'Divertissement', imageURL: '', isCustom: false, color: '#ec4899' },
  { key: 'HISTORY', name: 'Histoire', imageURL: '', isCustom: false, color: '#eab308' },
  { key: 'ARTS', name: 'Arts & Litterature', imageURL: '', isCustom: false, color: '#a855f7' },
  { key: 'SCIENCE', name: 'Sciences & Nature', imageURL: '', isCustom: false, color: '#22c55e' },
  { key: 'SPORTS', name: 'Sports & Loisirs', imageURL: '', isCustom: false, color: '#f97316' },
  { key: 'FOOD', name: 'Gastronomie', imageURL: '', isCustom: false, color: '#991b1b' },
  { key: 'ANIMALS', name: 'Animaux', imageURL: '', isCustom: false, color: '#78716c' },
  { key: 'CUSTOM_MASCOTS', name: 'Mascottes', imageURL: '/files/categories/mascots.png', isCustom: true, color: '' },
]

describe('CategorySelector — non-régression doublon de catégories (bug QUALIF 8.0.0.2)', () => {
  it('customCategories BRUT (réponse API non filtrée) : chaque catégorie standard apparaît UNE SEULE fois', () => {
    const { container } = renderSelector({ customCategories: RAW_API_CATEGORIES_RESPONSE })

    // LE test du bug lui-même : sans le filtre isCustom, les 8 clés
    // codées en dur apparaîtraient CHACUNE deux fois dans le tableau
    // combiné (une fois via Object.keys(CATEGORIES), une fois via
    // customCategories.map sur la réponse brute qui les remiroite) — total
    // attendu ici : 8 standard + 1 custom + le bouton "+" = 10, jamais plus.
    expect(container.querySelectorAll('.category-btn').length).toBe(10)

    for (const label of ['Geographie', 'Divertissement', 'Histoire', 'Arts & Litterature', 'Sciences & Nature', 'Sports & Loisirs', 'Gastronomie', 'Animaux']) {
      expect(screen.getAllByTitle(label)).toHaveLength(1)
    }
  })

  it('la vraie catégorie personnalisée (isCustom:true) apparaît, une seule fois', () => {
    renderSelector({ customCategories: RAW_API_CATEGORIES_RESPONSE })
    expect(screen.getAllByTitle('Mascottes')).toHaveLength(1)
  })

  it('sélectionner une catégorie standard depuis une réponse API brute fonctionne malgré le doublon potentiel dans les données sources', () => {
    const { onChange } = renderSelector({ customCategories: RAW_API_CATEGORIES_RESPONSE })
    fireEvent.click(screen.getByTitle('Histoire'))
    expect(onChange).toHaveBeenCalledTimes(1)
    expect(onChange).toHaveBeenCalledWith('HISTORY')
  })
})

// ---------------------------------------------------------------------------
// allowCreate
// ---------------------------------------------------------------------------

describe('CategorySelector — allowCreate', () => {
  it('allowCreate=true (défaut) : le bouton "+" est présent', () => {
    renderSelector()
    expect(screen.getByTitle('Créer une catégorie')).toBeInTheDocument()
  })

  it('allowCreate=false : AUCUN bouton "+" rendu, jamais de formulaire inline possible', () => {
    renderSelector({ allowCreate: false })
    expect(screen.queryByTitle('Créer une catégorie')).not.toBeInTheDocument()
  })
})

// ---------------------------------------------------------------------------
// Création inline (#97/#100)
// ---------------------------------------------------------------------------

describe('CategorySelector — création inline', () => {
  it('clic sur "+" affiche le formulaire (input texte, input file, Valider/Annuler)', () => {
    renderSelector()
    fireEvent.click(screen.getByTitle('Créer une catégorie'))

    expect(screen.getByPlaceholderText('Nom de la catégorie...')).toBeInTheDocument()
    expect(screen.getByText(/Choisir une image/)).toBeInTheDocument()
    expect(screen.getByText('Valider')).toBeInTheDocument()
    expect(screen.getByText('Annuler')).toBeInTheDocument()
  })

  it('"Annuler" masque le formulaire et réinitialise le nom saisi', () => {
    renderSelector()
    fireEvent.click(screen.getByTitle('Créer une catégorie'))
    fireEvent.change(screen.getByPlaceholderText('Nom de la catégorie...'), { target: { value: 'Ma categorie' } })
    fireEvent.click(screen.getByText('Annuler'))

    expect(screen.queryByPlaceholderText('Nom de la catégorie...')).not.toBeInTheDocument()
    // Ré-ouvrir prouve que le nom a bien été vidé, pas juste masqué.
    fireEvent.click(screen.getByTitle('Créer une catégorie'))
    expect(screen.getByPlaceholderText('Nom de la catégorie...').value).toBe('')
  })

  it('touche Échap dans le champ nom masque le formulaire', () => {
    renderSelector()
    fireEvent.click(screen.getByTitle('Créer une catégorie'))
    fireEvent.keyDown(screen.getByPlaceholderText('Nom de la catégorie...'), { key: 'Escape' })

    expect(screen.queryByPlaceholderText('Nom de la catégorie...')).not.toBeInTheDocument()
  })

  it('Valider sans nom : "Nom invalide", aucun fetch envoyé', async () => {
    global.fetch = vi.fn()
    renderSelector()
    fireEvent.click(screen.getByTitle('Créer une catégorie'))
    fireEvent.click(screen.getByText('Valider'))

    expect(await screen.findByText('Nom invalide')).toBeInTheDocument()
    expect(global.fetch).not.toHaveBeenCalled()
  })

  it('Valider avec un nom mais sans image : "Image requise", aucun fetch envoyé', async () => {
    global.fetch = vi.fn()
    renderSelector()
    fireEvent.click(screen.getByTitle('Créer une catégorie'))
    fireEvent.change(screen.getByPlaceholderText('Nom de la catégorie...'), { target: { value: 'Ma categorie' } })
    fireEvent.click(screen.getByText('Valider'))

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

    renderSelector({ onChange, onRefetchCategories })
    fireEvent.click(screen.getByTitle('Créer une catégorie'))
    fireEvent.change(screen.getByPlaceholderText('Nom de la catégorie...'), { target: { value: 'Ma categorie' } })

    const file = new File(['x'], 'photo.png', { type: 'image/png' })
    const fileInput = document.querySelector('input[type="file"]')
    fireEvent.change(fileInput, { target: { files: [file] } })

    fireEvent.click(screen.getByText('Valider'))

    await waitFor(() => {
      expect(onChange).toHaveBeenCalledWith('NEW_CAT')
    })

    expect(global.fetch).toHaveBeenCalledWith('/api/categories', expect.objectContaining({ method: 'POST' }))
    const [, options] = global.fetch.mock.calls[0]
    expect(options.body).toBeInstanceOf(FormData)
    expect(callOrder).toEqual(['refetch', 'onChange:NEW_CAT'])
    expect(screen.queryByPlaceholderText('Nom de la catégorie...')).not.toBeInTheDocument()
  })

  it('erreur 409 (conflit) : "Cette catégorie existe déjà", formulaire reste ouvert', async () => {
    global.fetch = vi.fn(() => Promise.resolve({ ok: false, status: 409 }))
    renderSelector()
    fireEvent.click(screen.getByTitle('Créer une catégorie'))
    fireEvent.change(screen.getByPlaceholderText('Nom de la catégorie...'), { target: { value: 'Existe' } })
    const file = new File(['x'], 'photo.png', { type: 'image/png' })
    fireEvent.change(document.querySelector('input[type="file"]'), { target: { files: [file] } })
    fireEvent.click(screen.getByText('Valider'))

    expect(await screen.findByText('Cette catégorie existe déjà')).toBeInTheDocument()
    expect(screen.getByPlaceholderText('Nom de la catégorie...')).toBeInTheDocument()
  })

  it('erreur 400 (autre) : message générique "Nom invalide ou image non supportée"', async () => {
    global.fetch = vi.fn(() => Promise.resolve({ ok: false, status: 400 }))
    renderSelector()
    fireEvent.click(screen.getByTitle('Créer une catégorie'))
    fireEvent.change(screen.getByPlaceholderText('Nom de la catégorie...'), { target: { value: 'X' } })
    fireEvent.change(document.querySelector('input[type="file"]'), { target: { files: [new File(['x'], 'p.png', { type: 'image/png' })] } })
    fireEvent.click(screen.getByText('Valider'))

    expect(await screen.findByText('Nom invalide ou image non supportée')).toBeInTheDocument()
  })

  it('rejet réseau (fetch throw) : "Erreur réseau"', async () => {
    global.fetch = vi.fn(() => Promise.reject(new Error('offline')))
    renderSelector()
    fireEvent.click(screen.getByTitle('Créer une catégorie'))
    fireEvent.change(screen.getByPlaceholderText('Nom de la catégorie...'), { target: { value: 'X' } })
    fireEvent.change(document.querySelector('input[type="file"]'), { target: { files: [new File(['x'], 'p.png', { type: 'image/png' })] } })
    fireEvent.click(screen.getByText('Valider'))

    expect(await screen.findByText('Erreur réseau')).toBeInTheDocument()
  })

  it('sans onRefetchCategories fourni (prop optionnelle) : la création réussie n\'explose pas', async () => {
    const onChange = vi.fn()
    global.fetch = vi.fn(() => Promise.resolve({
      ok: true, status: 200, json: () => Promise.resolve({ key: 'NEW_CAT' }),
    }))
    renderSelector({ onChange, onRefetchCategories: undefined })
    fireEvent.click(screen.getByTitle('Créer une catégorie'))
    fireEvent.change(screen.getByPlaceholderText('Nom de la catégorie...'), { target: { value: 'X' } })
    fireEvent.change(document.querySelector('input[type="file"]'), { target: { files: [new File(['x'], 'p.png', { type: 'image/png' })] } })
    fireEvent.click(screen.getByText('Valider'))

    await waitFor(() => expect(onChange).toHaveBeenCalledWith('NEW_CAT'))
  })
})
