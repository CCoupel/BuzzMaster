import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import RafalePoolAlert from './RafalePoolAlert'

// ---------------------------------------------------------------------------
// RafalePoolAlert — alerte de disponibilité du pool RAFALE (milestone
// v8.0.0 #16, contrat contracts/rafale.md §7.2/§7.5), partagée entre
// QuestionsPage.jsx et GamePage.jsx (tâche 26, Batch 2) — PAS RafalePage.jsx
// (l'éditeur de réservoir n'a pas de contexte TIME/RAFALE_QUESTION_TIME
// pour calculer un besoin estimé ; ses propres compteurs simples sont déjà
// couverts par RafalePage.rafale.test.jsx). C'est ICI que vivent les 3
// états d'alerte du contrat §7.2, pas dans RafalePage.
//
// ⚠️ [CHANGED] #216 (milestone v9.0.0, 2026-09-04) — props passées de
// `category`/`difficulty` (scalaires) à `categories`/`difficulties`
// (tableaux, appartenance ensembliste côté backend). Réécriture ASSUMÉE,
// pas une régression : voir contracts/rafale.md §7.1/§7.5 (dev-backend,
// commit ffd59d6e) et le rapport dev-frontend Lot 2. Le principe reste
// inchangé — un couple individuel vide/épuisé n'est PAS le problème de ce
// composant (déjà géré côté moteur, contrat §7.3/§7.4 : le tirage
// rééquilibre tout seul sur les couples restants) — cette alerte ne réagit
// qu'au TOTAL disponible sur l'UNION, exactement comme avant sur un couple
// unique. "Blocage uniquement si l'union totale est insuffisante" (Lot 2
// tâche 3) découle donc directement des 3 états déjà établis, appliqués à
// AVAILABLE de l'union plutôt qu'à un seul couple — aucune nouvelle logique
// de niveau à inventer ici.
// ---------------------------------------------------------------------------

function jsonResponse(body, ok = true, status = 200) {
  return Promise.resolve({
    ok,
    status,
    json: () => Promise.resolve(body),
  })
}

beforeEach(() => {
  vi.clearAllMocks()
})

describe('RafalePoolAlert — filtre incomplet (pas de fetch)', () => {
  it('categories vide : message neutre, AUCUN fetch déclenché', async () => {
    global.fetch = vi.fn()
    render(<RafalePoolAlert categories={[]} difficulties={[2]} roundTime={120} questionTime={3} />)

    expect(screen.getByText(/Sélectionnez au moins une catégorie/)).toBeInTheDocument()
    expect(global.fetch).not.toHaveBeenCalled()
  })

  it('difficulties vide : message neutre, AUCUN fetch déclenché', async () => {
    global.fetch = vi.fn()
    render(<RafalePoolAlert categories={['HISTORY']} difficulties={[]} roundTime={120} questionTime={3} />)

    expect(screen.getByText(/Sélectionnez au moins une catégorie/)).toBeInTheDocument()
    expect(global.fetch).not.toHaveBeenCalled()
  })
})

describe('RafalePoolAlert — appel réseau', () => {
  it('filtre à une seule catégorie/difficulté : appelle GET /api/rafale/pool avec categories/difficulties en query (liste à un élément)', async () => {
    global.fetch = vi.fn(() => jsonResponse({ AVAILABLE: 50, USED: 0, TOTAL: 50 }))
    render(<RafalePoolAlert categories={['HISTORY']} difficulties={[2]} roundTime={120} questionTime={3} />)

    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledWith('/api/rafale/pool?categories=HISTORY&difficulties=2')
    })
  })

  it('filtre à PLUSIEURS catégories/difficultés : liste jointe par virgule (#216)', async () => {
    global.fetch = vi.fn(() => jsonResponse({ AVAILABLE: 50, USED: 0, TOTAL: 50 }))
    render(<RafalePoolAlert categories={['HISTORY', 'SCIENCE']} difficulties={[1, 2]} roundTime={120} questionTime={3} />)

    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledWith('/api/rafale/pool?categories=HISTORY%2CSCIENCE&difficulties=1%2C2')
    })
  })

  it('pendant le chargement : message neutre "Vérification du pool…"', () => {
    global.fetch = vi.fn(() => new Promise(() => {})) // never resolves
    render(<RafalePoolAlert categories={['HISTORY']} difficulties={[1]} roundTime={120} questionTime={3} />)

    expect(screen.getByText(/Vérification du pool/)).toBeInTheDocument()
  })

  it('erreur réseau : message neutre affichant l\'erreur, onLevelChange(null)', async () => {
    global.fetch = vi.fn(() => Promise.reject(new Error('network down')))
    const onLevelChange = vi.fn()
    render(<RafalePoolAlert categories={['HISTORY']} difficulties={[1]} roundTime={120} questionTime={3} onLevelChange={onLevelChange} />)

    expect(await screen.findByText(/Erreur : network down/)).toBeInTheDocument()
    expect(onLevelChange).toHaveBeenCalledWith(null)
  })
})

describe('RafalePoolAlert — 3 états d\'alerte, sur le TOTAL de l\'union (contrat §7.1/§7.2)', () => {
  // Textes raccourcis (retour utilisateur 2026-08-30, #198) — inchangés par
  // #216 : seul ce qui alimente le calcul (union multi vs couple unique)
  // change, pas le rendu des 3 états eux-mêmes.
  it('AVAILABLE=0 sur l\'union : état BLOQUANT — icône ✕, message court', async () => {
    global.fetch = vi.fn(() => jsonResponse({ AVAILABLE: 0, USED: 5, TOTAL: 5 }))
    const { container } = render(<RafalePoolAlert categories={['HISTORY', 'SCIENCE']} difficulties={[1, 2]} roundTime={120} questionTime={3} />)

    await waitFor(() => {
      expect(container.querySelector('.rafale-pool-alert-blocking')).not.toBeNull()
    })
    expect(screen.getByText(/0 disponible — bloquant/)).toBeInTheDocument()
  })

  it('0 < AVAILABLE(union) < besoin estimé : état AVERTISSEMENT, PAS bloquant — un couple vide parmi plusieurs ne bloque jamais (Lot 2 tâche 3)', async () => {
    // roundTime=120, questionTime=3 -> besoin=40 ; 10 disponibles sur l'union
    // (ex: 0 pour HISTORY/1 épuisé + 10 pour les 3 autres couples) < 40.
    global.fetch = vi.fn(() => jsonResponse({ AVAILABLE: 10, USED: 0, TOTAL: 10 }))
    const { container } = render(<RafalePoolAlert categories={['HISTORY', 'SCIENCE']} difficulties={[1, 2]} roundTime={120} questionTime={3} />)

    await waitFor(() => {
      expect(container.querySelector('.rafale-pool-alert-warning')).not.toBeNull()
    })
    expect(container.querySelector('.rafale-pool-alert-blocking')).toBeNull()
    expect(screen.getByText(/10 questions disponibles — insuffisant/)).toBeInTheDocument()
  })

  it('AVAILABLE(union) >= besoin estimé : état NEUTRE (OK)', async () => {
    global.fetch = vi.fn(() => jsonResponse({ AVAILABLE: 45, USED: 5, TOTAL: 50 }))
    const { container } = render(<RafalePoolAlert categories={['HISTORY', 'SCIENCE']} difficulties={[1, 2]} roundTime={120} questionTime={3} />)

    await waitFor(() => {
      expect(container.querySelector('.rafale-pool-alert-ok')).not.toBeNull()
    })
    expect(screen.getByText(/45 questions disponibles/)).toBeInTheDocument()
  })
})

describe('RafalePoolAlert — onLevelChange (consommé par GamePage pour bloquer START)', () => {
  it('notifie "blocking" / "warning" / "ok" selon le niveau résolu, sur le total de l\'union', async () => {
    const onLevelChange = vi.fn()
    global.fetch = vi.fn(() => jsonResponse({ AVAILABLE: 0, USED: 5, TOTAL: 5 }))
    render(<RafalePoolAlert categories={['HISTORY', 'SCIENCE']} difficulties={[1, 2]} roundTime={120} questionTime={3} onLevelChange={onLevelChange} />)

    await waitFor(() => {
      expect(onLevelChange).toHaveBeenCalledWith('blocking')
    })
  })

  it('filtre incomplet (categories ou difficulties vide) : onLevelChange(null), jamais appelé avec une chaîne', () => {
    const onLevelChange = vi.fn()
    global.fetch = vi.fn()
    render(<RafalePoolAlert categories={[]} difficulties={[1]} roundTime={120} questionTime={3} onLevelChange={onLevelChange} />)

    expect(onLevelChange).toHaveBeenCalledWith(null)
  })
})
