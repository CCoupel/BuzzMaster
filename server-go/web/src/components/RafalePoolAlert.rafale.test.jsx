import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import RafalePoolAlert from './RafalePoolAlert'

// ---------------------------------------------------------------------------
// RafalePoolAlert — alerte de disponibilité du pool RAFALE (milestone
// v8.0.0 #16, contrat contracts/rafale.md §7.2), partagée entre
// QuestionsPage.jsx et GamePage.jsx (tâche 26, Batch 2) — PAS RafalePage.jsx
// (l'éditeur de réservoir n'a pas de contexte TIME/RAFALE_QUESTION_TIME
// pour calculer un besoin estimé ; ses propres compteurs simples sont déjà
// couverts par RafalePage.rafale.test.jsx). C'est ICI que vivent les 3
// états d'alerte du contrat §7.2, pas dans RafalePage.
//
// Composant déjà livré par dev-frontend (Batch 2/3) au moment où ce fichier
// est écrit — tests écrits contre son API réelle.
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
    render(<RafalePoolAlert categories={[]} difficulty={2} roundTime={120} questionTime={3} />)

    expect(screen.getByText(/Sélectionnez au moins une catégorie/)).toBeInTheDocument()
    expect(global.fetch).not.toHaveBeenCalled()
  })

  it('difficulty hors 1..3 : message neutre, AUCUN fetch déclenché', async () => {
    global.fetch = vi.fn()
    render(<RafalePoolAlert categories={['HISTORY']} difficulty={0} roundTime={120} questionTime={3} />)

    expect(screen.getByText(/Sélectionnez au moins une catégorie/)).toBeInTheDocument()
    expect(global.fetch).not.toHaveBeenCalled()
  })
})

describe('RafalePoolAlert — appel réseau', () => {
  it('filtre complet : appelle GET /api/rafale/pool avec categories (jointes) et difficulty en query', async () => {
    global.fetch = vi.fn(() => jsonResponse({ AVAILABLE: 50, USED: 0, TOTAL: 50 }))
    render(<RafalePoolAlert categories={['HISTORY', 'SCIENCE']} difficulty={2} roundTime={120} questionTime={3} />)

    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledWith('/api/rafale/pool?categories=HISTORY%2CSCIENCE&difficulty=2')
    })
  })

  it('pendant le chargement : message neutre "Vérification du pool…"', () => {
    global.fetch = vi.fn(() => new Promise(() => {})) // never resolves
    render(<RafalePoolAlert categories={['HISTORY']} difficulty={1} roundTime={120} questionTime={3} />)

    expect(screen.getByText(/Vérification du pool/)).toBeInTheDocument()
  })

  it('erreur réseau : message neutre affichant l\'erreur, onLevelChange(null)', async () => {
    global.fetch = vi.fn(() => Promise.reject(new Error('network down')))
    const onLevelChange = vi.fn()
    render(<RafalePoolAlert categories={['HISTORY']} difficulty={1} roundTime={120} questionTime={3} onLevelChange={onLevelChange} />)

    expect(await screen.findByText(/Erreur : network down/)).toBeInTheDocument()
    expect(onLevelChange).toHaveBeenCalledWith(null)
  })
})

describe('RafalePoolAlert — 3 états d\'alerte (contrat §7.2)', () => {
  it('AVAILABLE=0 : état BLOQUANT — icône ✕, "Démarrage bloqué."', async () => {
    global.fetch = vi.fn(() => jsonResponse({ AVAILABLE: 0, USED: 5, TOTAL: 5 }))
    const { container } = render(<RafalePoolAlert categories={['HISTORY']} difficulty={1} roundTime={120} questionTime={3} />)

    await waitFor(() => {
      expect(container.querySelector('.rafale-pool-alert-blocking')).not.toBeNull()
    })
    expect(screen.getByText('Aucune question disponible pour ce filtre')).toBeInTheDocument()
    expect(screen.getByText(/Démarrage bloqué\./)).toBeInTheDocument()
  })

  it('0 < AVAILABLE < besoin estimé : état AVERTISSEMENT — icône !, "Démarrage autorisé."', async () => {
    // roundTime=120, questionTime=3 -> besoin=40 ; 10 disponibles < 40
    global.fetch = vi.fn(() => jsonResponse({ AVAILABLE: 10, USED: 0, TOTAL: 10 }))
    const { container } = render(<RafalePoolAlert categories={['HISTORY']} difficulty={1} roundTime={120} questionTime={3} />)

    await waitFor(() => {
      expect(container.querySelector('.rafale-pool-alert-warning')).not.toBeNull()
    })
    expect(screen.getByText(/10 questions disponibles/)).toBeInTheDocument()
    expect(screen.getByText(/Démarrage autorisé\./)).toBeInTheDocument()
    expect(screen.getByText(/Besoin estimé pour 120s : ~40 questions\./)).toBeInTheDocument()
  })

  it('AVAILABLE >= besoin estimé : état NEUTRE (OK) — icône ✓, aucun texte de blocage/avertissement', async () => {
    global.fetch = vi.fn(() => jsonResponse({ AVAILABLE: 45, USED: 5, TOTAL: 50 }))
    const { container } = render(<RafalePoolAlert categories={['HISTORY']} difficulty={1} roundTime={120} questionTime={3} />)

    await waitFor(() => {
      expect(container.querySelector('.rafale-pool-alert-ok')).not.toBeNull()
    })
    expect(screen.getByText(/45 questions disponibles/)).toBeInTheDocument()
    expect(screen.queryByText(/Démarrage bloqué/)).not.toBeInTheDocument()
    expect(screen.queryByText(/Démarrage autorisé/)).not.toBeInTheDocument()
  })

  it('limite EXACTE (AVAILABLE == besoin estimé) : neutre (OK), pas avertissement — limite incluse côté OK', async () => {
    // roundTime=120, questionTime=3 -> besoin=40 pile
    global.fetch = vi.fn(() => jsonResponse({ AVAILABLE: 40, USED: 0, TOTAL: 40 }))
    const { container } = render(<RafalePoolAlert categories={['HISTORY']} difficulty={1} roundTime={120} questionTime={3} />)

    await waitFor(() => {
      expect(container.querySelector('.rafale-pool-alert-ok')).not.toBeNull()
    })
    expect(container.querySelector('.rafale-pool-alert-warning')).toBeNull()
  })

  it('besoin estimé arrondi au PLAFOND (contrat §7.2 : estimation majorante) — 121s/3s doit donner 41, pas 40', async () => {
    global.fetch = vi.fn(() => jsonResponse({ AVAILABLE: 40, USED: 0, TOTAL: 40 }))
    render(<RafalePoolAlert categories={['HISTORY']} difficulty={1} roundTime={121} questionTime={3} />)

    // 40 disponibles < 41 (besoin arrondi au plafond) -> AVERTISSEMENT, pas OK
    expect(await screen.findByText(/Démarrage autorisé\./)).toBeInTheDocument()
    expect(screen.getByText(/Besoin estimé pour 121s : ~41 questions\./)).toBeInTheDocument()
  })
})

describe('RafalePoolAlert — onLevelChange (consommé par GamePage pour bloquer START)', () => {
  it('notifie "blocking" / "warning" / "ok" selon le niveau résolu', async () => {
    const onLevelChange = vi.fn()
    global.fetch = vi.fn(() => jsonResponse({ AVAILABLE: 0, USED: 5, TOTAL: 5 }))
    render(<RafalePoolAlert categories={['HISTORY']} difficulty={1} roundTime={120} questionTime={3} onLevelChange={onLevelChange} />)

    await waitFor(() => {
      expect(onLevelChange).toHaveBeenCalledWith('blocking')
    })
  })

  it('filtre incomplet : onLevelChange(null), jamais appelé avec une chaîne', () => {
    const onLevelChange = vi.fn()
    global.fetch = vi.fn()
    render(<RafalePoolAlert categories={[]} difficulty={1} roundTime={120} questionTime={3} onLevelChange={onLevelChange} />)

    expect(onLevelChange).toHaveBeenCalledWith(null)
  })
})
