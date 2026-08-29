import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, waitFor, within, fireEvent } from '@testing-library/react'
import RafalePage from './RafalePage'

// ---------------------------------------------------------------------------
// RafalePage — éditeur du réservoir RAFALE `/admin/rafale` (milestone
// v8.0.0 #16/#197, contrat contracts/rafale.md §9, maquette
// docs/mockups/rafale-v8.html §7).
//
// CRUD via JSON pur (pas de multipart, §2.4/§9) :
//   GET    /api/rafale/questions            -> {QUESTIONS:[{ID,QUESTION,
//                                               ANSWER,CATEGORY,DIFFICULTY,
//                                               USED}], TOTAL}
//   POST   /api/rafale/questions             -> create (sans ID) / update
//                                               (avec ID)
//   DELETE /api/rafale/questions/{id}
//
// USED est DÉRIVÉ côté serveur à la lecture (§3.2/§9) — jamais édité ici,
// seulement affiché (colonne État + filtre "Non utilisees"). Filtres
// catégorie/difficulté/état sont appliqués CÔTÉ CLIENT (useCategoryFilter +
// état local) sur la liste déjà chargée — aucun paramètre de requête n'est
// envoyé par RafalePage (contrairement à GET /api/rafale/pool, hors
// périmètre de cette page — alertes de pool avec estimation du besoin =
// panneau de configuration de manche, GamePage.jsx, tâche 26, Batch 2).
//
// Composant déjà livré par dev-frontend au moment où ce fichier est écrit
// (Batch 1) — tests écrits contre son comportement réel, vérifié conforme
// au contrat.
// ---------------------------------------------------------------------------

const QUESTIONS_FIXTURE = [
  { ID: 'r-001', QUESTION: 'Capitale de l\'Italie ?', ANSWER: 'Rome', CATEGORY: 'GEOGRAPHY', DIFFICULTY: 1, USED: false },
  { ID: 'r-002', QUESTION: 'Annee de la Revolution francaise ?', ANSWER: '1789', CATEGORY: 'HISTORY', DIFFICULTY: 2, USED: true },
  { ID: 'r-003', QUESTION: 'Symbole chimique de l\'or ?', ANSWER: 'Au', CATEGORY: 'SCIENCE', DIFFICULTY: 3, USED: false },
]

function jsonResponse(body, ok = true, status = 200) {
  return Promise.resolve({
    ok,
    status,
    json: () => Promise.resolve(body),
    text: () => Promise.resolve(typeof body === 'string' ? body : JSON.stringify(body)),
  })
}

/**
 * Routes global.fetch by URL/method — RafalePage calls both
 * GET /api/rafale/questions (its own data) and GET /api/categories (shared
 * useCategories hook, custom categories catalogue).
 */
function mockFetchRouter({ questions = QUESTIONS_FIXTURE, categories = [] } = {}) {
  return vi.fn((url, options = {}) => {
    const method = options.method || 'GET'

    if (typeof url === 'string' && url.startsWith('/api/rafale/questions/') && method === 'DELETE') {
      return jsonResponse({ DELETED: url.split('/').pop() })
    }
    if (url === '/api/rafale/questions' && method === 'GET') {
      return jsonResponse({ QUESTIONS: questions, TOTAL: questions.length })
    }
    if (url === '/api/rafale/questions' && method === 'POST') {
      const body = JSON.parse(options.body)
      return jsonResponse({ ID: body.ID || 'r-new' })
    }
    if (url === '/api/categories') {
      return jsonResponse(categories)
    }
    return jsonResponse({}, false, 404)
  })
}

function renderRafalePage(fetchOpts) {
  global.fetch = mockFetchRouter(fetchOpts)
  global.confirm = vi.fn(() => true)
  return render(<RafalePage />)
}

beforeEach(() => {
  vi.clearAllMocks()
})

// ---------------------------------------------------------------------------
// Chargement initial — GET /api/rafale/questions (§9)
// ---------------------------------------------------------------------------

describe('RafalePage — chargement initial', () => {
  it('appelle GET /api/rafale/questions au montage', async () => {
    renderRafalePage()
    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledWith('/api/rafale/questions')
    })
  })

  it('affiche les 3 questions du réservoir une fois chargées', async () => {
    renderRafalePage()
    expect(await screen.findByText('Capitale de l\'Italie ?')).toBeInTheDocument()
    expect(screen.getByText('Annee de la Revolution francaise ?')).toBeInTheDocument()
    expect(screen.getByText('Symbole chimique de l\'or ?')).toBeInTheDocument()
  })

  it('affiche l\'état dérivé (utilisee/disponible) par question, sans jamais l\'éditer ici', async () => {
    renderRafalePage()
    await screen.findByText('Capitale de l\'Italie ?')
    const usedRow = screen.getByText('Annee de la Revolution francaise ?').closest('tr')
    const unusedRow = screen.getByText('Capitale de l\'Italie ?').closest('tr')
    expect(within(usedRow).getByText('utilisee')).toBeInTheDocument()
    expect(within(unusedRow).getByText('disponible')).toBeInTheDocument()
  })

  it('affiche le décompte total/utilisées/disponibles', async () => {
    renderRafalePage()
    await screen.findByText('Capitale de l\'Italie ?')
    // 3 questions, 1 utilisee (r-002), 2 disponibles (r-001, r-003)
    expect(screen.getByText(/3 questions/)).toBeInTheDocument()
    expect(screen.getByText(/1 utilisee/)).toBeInTheDocument()
    expect(screen.getByText(/2 disponibles/)).toBeInTheDocument()
  })

  it('réservoir vide : message "Aucune question pour ce filtre."', async () => {
    renderRafalePage({ questions: [] })
    expect(await screen.findByText('Aucune question pour ce filtre.')).toBeInTheDocument()
  })

  it('erreur réseau : affiche un message d\'erreur, ne plante pas', async () => {
    global.fetch = vi.fn(() => Promise.reject(new Error('network down')))
    global.confirm = vi.fn(() => true)
    render(<RafalePage />)
    expect(await screen.findByText(/Erreur/)).toBeInTheDocument()
  })
})

// ---------------------------------------------------------------------------
// Filtres catégorie/difficulté/état — appliqués côté client (§ compteurs)
// ---------------------------------------------------------------------------

describe('RafalePage — filtres', () => {
  it('filtre par difficulté : ne montre que les questions de la difficulté choisie', async () => {
    renderRafalePage()
    await screen.findByText('Capitale de l\'Italie ?')

    const diff2Chip = screen.getByTitle('Difficulte 2')
    fireEvent.click(diff2Chip)

    await waitFor(() => {
      expect(screen.getByText('Annee de la Revolution francaise ?')).toBeInTheDocument()
      expect(screen.queryByText('Capitale de l\'Italie ?')).not.toBeInTheDocument()
      expect(screen.queryByText('Symbole chimique de l\'or ?')).not.toBeInTheDocument()
    })
  })

  it('filtre "Non utilisees" masque les questions déjà utilisées', async () => {
    renderRafalePage()
    await screen.findByText('Capitale de l\'Italie ?')

    fireEvent.click(screen.getByText('Non utilisees'))

    await waitFor(() => {
      expect(screen.queryByText('Annee de la Revolution francaise ?')).not.toBeInTheDocument()
      expect(screen.getByText('Capitale de l\'Italie ?')).toBeInTheDocument()
      expect(screen.getByText('Symbole chimique de l\'or ?')).toBeInTheDocument()
    })
  })

  it('les filtres sont appliqués côté client — aucun nouveau fetch déclenché', async () => {
    renderRafalePage()
    await screen.findByText('Capitale de l\'Italie ?')
    const callsBefore = global.fetch.mock.calls.length

    fireEvent.click(screen.getByTitle('Difficulte 1'))

    await waitFor(() => {
      expect(screen.queryByText('Annee de la Revolution francaise ?')).not.toBeInTheDocument()
    })
    expect(global.fetch.mock.calls.length).toBe(callsBefore)
  })
})

// ---------------------------------------------------------------------------
// CRUD — POST (create/update) et DELETE (§9)
// ---------------------------------------------------------------------------

describe('RafalePage — création', () => {
  it('soumet POST /api/rafale/questions SANS ID pour une création', async () => {
    const { container } = renderRafalePage()
    await screen.findByText('Capitale de l\'Italie ?')

    // Le formulaire (label sans `for`/`id`, markup réel de RafalePage.jsx)
    // est adressé par structure : un seul textarea (Énoncé), un seul input
    // texte (Réponse), un bouton catégorie icône dans .rafale-form —
    // CategorySelector (v8.0.0, #16/#197, bugfix cohérence UI), même
    // composant que l'éditeur de question standard, plus de <select> texte.
    const form = container.querySelector('.rafale-form')
    fireEvent.change(form.querySelector('textarea'), {
      target: { value: 'Plus long fleuve du monde ?' },
    })
    fireEvent.change(form.querySelector('input[type="text"]'), { target: { value: 'Le Nil' } })
    fireEvent.click(within(form).getByTitle('Geographie'))

    fireEvent.click(screen.getByText('Ajouter'))

    await waitFor(() => {
      const postCall = global.fetch.mock.calls.find(([url, opts]) => url === '/api/rafale/questions' && opts?.method === 'POST')
      expect(postCall).toBeTruthy()
      const body = JSON.parse(postCall[1].body)
      expect(body.ID).toBeUndefined()
      expect(body.QUESTION).toBe('Plus long fleuve du monde ?')
      expect(body.ANSWER).toBe('Le Nil')
      expect(body.CATEGORY).toBe('GEOGRAPHY')
    })
  })

  it('énoncé ou réponse vide : refuse la soumission côté client, aucun POST envoyé', async () => {
    const { container } = renderRafalePage()
    await screen.findByText('Capitale de l\'Italie ?')
    const callsBefore = global.fetch.mock.calls.length

    // fireEvent.submit (plutôt qu'un clic sur le bouton submit) : contourne
    // la validation HTML5 native (`required`) que jsdom applique sur un
    // clic réel, pour exercer directement la validation JS de handleSubmit
    // (form.QUESTION.trim() / form.ANSWER.trim()) — le comportement testé
    // ici.
    fireEvent.submit(container.querySelector('.rafale-form'))

    await waitFor(() => {
      expect(screen.getByText(/obligatoire/i)).toBeInTheDocument()
    })
    const postCalls = global.fetch.mock.calls.filter(([url, opts]) => url === '/api/rafale/questions' && opts?.method === 'POST')
    expect(postCalls).toHaveLength(0)
    expect(global.fetch.mock.calls.length).toBe(callsBefore)
  })
})

describe('RafalePage — édition', () => {
  it('clic sur ✎ pré-remplit le formulaire, la soumission envoie l\'ID (update, pas create)', async () => {
    const { container } = renderRafalePage()
    await screen.findByText('Capitale de l\'Italie ?')

    const row = screen.getByText('Capitale de l\'Italie ?').closest('tr')
    fireEvent.click(within(row).getByTitle('Modifier'))

    expect(await screen.findByDisplayValue('Rome')).toBeInTheDocument()
    expect(container.querySelector('.rafale-form-card').textContent).toContain('Modifier la question')

    fireEvent.click(screen.getByText('Enregistrer'))

    await waitFor(() => {
      const postCall = global.fetch.mock.calls.find(([url, opts]) => url === '/api/rafale/questions' && opts?.method === 'POST')
      expect(postCall).toBeTruthy()
      const body = JSON.parse(postCall[1].body)
      expect(body.ID).toBe('r-001')
    })
  })
})

describe('RafalePage — suppression', () => {
  it('clic sur 🗑 demande confirmation puis envoie DELETE /api/rafale/questions/{id}', async () => {
    renderRafalePage()
    await screen.findByText('Capitale de l\'Italie ?')

    const row = screen.getByText('Capitale de l\'Italie ?').closest('tr')
    fireEvent.click(within(row).getByTitle('Supprimer'))

    expect(global.confirm).toHaveBeenCalled()
    await waitFor(() => {
      const deleteCall = global.fetch.mock.calls.find(([url, opts]) => opts?.method === 'DELETE')
      expect(deleteCall).toBeTruthy()
      expect(deleteCall[0]).toBe('/api/rafale/questions/r-001')
    })
  })

  it('confirmation refusée (confirm=false) : aucun DELETE envoyé', async () => {
    global.fetch = mockFetchRouter()
    global.confirm = vi.fn(() => false)
    render(<RafalePage />)
    await screen.findByText('Capitale de l\'Italie ?')

    const row = screen.getByText('Capitale de l\'Italie ?').closest('tr')
    fireEvent.click(within(row).getByTitle('Supprimer'))

    await new Promise((r) => setTimeout(r, 0))
    const deleteCall = global.fetch.mock.calls.find(([, opts]) => opts?.method === 'DELETE')
    expect(deleteCall).toBeUndefined()
  })
})

// ---------------------------------------------------------------------------
// Non-régression — doublon de catégories dans le listing du pool (retour
// utilisateur QUALIF 8.0.0.2, "le pool RAFALE liste 2 lignes identiques de
// catégorie", SHA ba960bdf). Reproduit la forme RÉELLE de GET /api/categories
// (internal/server/http.go : les 8 catégories codées en dur EN MIROIR,
// isCustom:false, FUSIONNÉES avec les vraies catégories personnalisées,
// isCustom:true) — exactement ce que useCategories() renvoie à RafalePage,
// non pré-filtré par le mock (comme en production). Bout en bout : la page
// entière (fetch réel + CategorySelector + CategoryFilterBar) ne doit
// produire AUCUN doublon visuel.
// ---------------------------------------------------------------------------

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

const MIXED_CATEGORY_QUESTIONS = [
  { ID: 'r-101', QUESTION: 'Q Histoire', ANSWER: 'A', CATEGORY: 'HISTORY', DIFFICULTY: 1, USED: false },
  { ID: 'r-102', QUESTION: 'Q Science', ANSWER: 'A', CATEGORY: 'SCIENCE', DIFFICULTY: 2, USED: false },
  { ID: 'r-103', QUESTION: 'Q Mascotte', ANSWER: 'A', CATEGORY: 'CUSTOM_MASCOTS', DIFFICULTY: 1, USED: true },
]

describe('RafalePage — non-régression : listing du pool sans doublon de catégories (bug QUALIF 8.0.0.2)', () => {
  it('la barre de filtre affiche chaque catégorie EXACTEMENT une fois (mix codées en dur + custom, réponse API brute)', async () => {
    const { container } = renderRafalePage({ questions: MIXED_CATEGORY_QUESTIONS, categories: RAW_API_CATEGORIES_RESPONSE })

    await screen.findByText('Q Histoire')

    // Scopé à la barre de filtre (.rafale-filters) : CategorySelector (dans
    // .rafale-form-card, formulaire de création) rend AUSSI ces mêmes
    // catégories plus bas sur la page — sans ce scope, getByTitle serait
    // ambigu entre les 2 (2 usages légitimes, pas un doublon).
    const filterBar = within(container.querySelector('.rafale-filters'))
    expect(filterBar.getAllByTitle('Histoire')).toHaveLength(1)
    expect(filterBar.getAllByTitle('Sciences & Nature')).toHaveLength(1)
    expect(filterBar.getAllByTitle('Mascottes')).toHaveLength(1)

    // 3 catégories présentes dans le réservoir chargé -> 3 pastilles, pas plus.
    expect(container.querySelectorAll('.category-filter-pill').length).toBe(3)
  })

  it('cliquer sur le filtre "Mascottes" (catégorie custom) fonctionne malgré le mélange de données brutes', async () => {
    const { container } = renderRafalePage({ questions: MIXED_CATEGORY_QUESTIONS, categories: RAW_API_CATEGORIES_RESPONSE })
    await screen.findByText('Q Histoire')

    fireEvent.click(within(container.querySelector('.rafale-filters')).getByTitle('Mascottes'))

    await waitFor(() => {
      expect(screen.getByText('Q Mascotte')).toBeInTheDocument()
      expect(screen.queryByText('Q Histoire')).not.toBeInTheDocument()
      expect(screen.queryByText('Q Science')).not.toBeInTheDocument()
    })
  })

  it('le formulaire de création (CategorySelector) ne présente pas non plus de doublon avec cette même réponse API brute', async () => {
    const { container } = renderRafalePage({ questions: MIXED_CATEGORY_QUESTIONS, categories: RAW_API_CATEGORIES_RESPONSE })
    await screen.findByText('Q Histoire')

    // 8 standard + 1 custom (Mascottes) + le bouton "+" du formulaire de création.
    expect(container.querySelectorAll('.rafale-form-card .category-btn').length).toBe(10)
    expect(within(container.querySelector('.rafale-form-card')).getAllByTitle('Histoire')).toHaveLength(1)
  })
})
