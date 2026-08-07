import { describe, it, expect, afterEach, vi } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import AIGenerateModal from './AIGenerateModal'

// Tests dérivés de _work/mockups/137-generation-tache-de-fond.md (#137, v6.1.0) —
// §3 à §6 — et de contracts/ai-multi-provider.md §10/§11. Fichier NEUF, compagnon de
// `AIGenerateModal.test.jsx` (#8, inchangé, pas retouché ici) — même motif que le
// fichier de dev-backend `ai_generator_m2_test.go` : nouveau fichier plutôt
// qu'extension, pour ne jamais entrer en collision.
//
// --------------------------------------------------------------------------------
// Hypothèse de props — à faire valider par dev-frontend/code-reviewer
// --------------------------------------------------------------------------------
// Au moment où ce fichier est écrit, AIGenerateModal.jsx n'a PAS encore été modifié
// pour #137 (vérifié : aucune mention de "Provider"/"groq"/job dans le composant —
// seul `web/src/hooks/useWebSocket.js` expose déjà l'état de job, sous la forme
// `aiJob: { jobId, state, batchesDone, batchesTotal, createdCount, skippedCount,
// errorCode, provider } | null` et `cancelAiGeneration(jobId)`, cf. contrat §10/§11).
// Ces tests supposent que ces deux éléments sont transmis à AIGenerateModal via deux
// NOUVELLES props, dans le même style que les props existantes (`apiKeyConfigured`,
// `onGenerated`, `onNavigateToQuizSettings` — passées explicitement par le parent,
// pas de useGame() interne au composant, motif #8 inchangé) :
//
//   aiJob              — le même objet que ci-dessus, ou null
//   onCancelGeneration — (jobId) => void, appelle cancelAiGeneration côté parent
//
// Si dev-frontend a choisi d'autres noms, seuls les points de montage (`renderModal`
// ci-dessous) sont à adapter — le contenu attendu (textes exacts de la maquette,
// machine à états observable) reste valable tel quel.
//
// Hors scope de ce fichier (composants qui ne sont pas AIGenerateModal) :
//  - le toast de fin de job quand la modale a été fermée (maquette §4 — vit dans
//    QuestionsPage, cf. procédure manuelle) ;
//  - le regroupement par catégorie de l'état TERMINÉ (maquette §4 "• Cinéma — 94
//    questions") : AI_GENERATION_PROGRESS (contrat §10) ne porte qu'un total
//    CREATED_COUNT/SKIPPED_COUNT, pas de détail par catégorie — ce regroupement, s'il
//    existe, vient probablement de `questions` (contexte global), pas de `aiJob` seul ;
//  - le décompte "Prochain lot dans Ns…" pendant l'attente inter-lots (minuterie
//    client dont le mécanisme exact n'est pas encore écrit) — laissé à la procédure
//    manuelle QA ;
//  - l'activation du bouton d'entrée « ✨ Générer via IA » selon le provider
//    sélectionné (CA1) : ce bouton vit dans QuestionsPage, pas dans la modale — la
//    modale ne fait que refléter `apiKeyConfigured`, mécanisme inchangé depuis #8.

const CATEGORIES = [
  { key: 'ENTERTAINMENT', name: 'Divertissement', color: '#ec4899' },
]

function baseProps(overrides = {}) {
  return {
    onClose: vi.fn(),
    apiKeyConfigured: true,
    categories: CATEGORIES,
    quizTheme: 'Cinéma français des années 80',
    quizPopulations: ['Adulte (18-64 ans)'],
    quizDifficulties: ['Moyen'],
    quizLanguage: 'Français',
    quizObjectives: '',
    onGenerated: vi.fn(),
    onNavigateToQuizSettings: vi.fn(),
    aiJob: null,
    onCancelGeneration: vi.fn(),
    ...overrides,
  }
}

function renderModal(overrides = {}) {
  const props = baseProps(overrides)
  render(
    <MemoryRouter>
      <AIGenerateModal {...props} />
    </MemoryRouter>
  )
  return props
}

function runningJob(overrides = {}) {
  return {
    jobId: 'gen-20260805-204318-a1b2',
    state: 'RUNNING',
    batchesDone: 3,
    batchesTotal: 10,
    createdCount: 58,
    skippedCount: 2,
    errorCode: '',
    provider: 'groq',
    ...overrides,
  }
}

describe('AIGenerateModal — ré-attachement (maquette §6, contrat §10)', () => {
  afterEach(() => vi.clearAllMocks())

  it('shows the progress panel (not the form) when a job is already RUNNING at mount', () => {
    renderModal({ aiJob: runningJob() })
    expect(screen.getByText(/lot 3 sur 10/)).toBeInTheDocument()
    expect(screen.queryByText('Cette génération')).not.toBeInTheDocument()
  })

  it('reattaches to a RUNNING job even when apiKeyConfigured is false (job takes priority over Indisponible)', () => {
    renderModal({ apiKeyConfigured: false, aiJob: runningJob() })
    expect(screen.getByText(/lot 3 sur 10/)).toBeInTheDocument()
    expect(screen.queryByText(/Génération indisponible/)).not.toBeInTheDocument()
  })

  it('shows the ordinary form when aiJob is null and a key is configured (unchanged #8 behavior)', () => {
    renderModal({ aiJob: null })
    expect(screen.getByText('Cette génération')).toBeInTheDocument()
  })

  it('transitions from the form to the progress panel if aiJob starts RUNNING while the modal stays open', () => {
    const props = baseProps({ aiJob: null })
    const { rerender } = render(<MemoryRouter><AIGenerateModal {...props} /></MemoryRouter>)
    expect(screen.getByText('Cette génération')).toBeInTheDocument()

    rerender(<MemoryRouter><AIGenerateModal {...props} aiJob={runningJob({ batchesDone: 0 })} /></MemoryRouter>)

    expect(screen.getByText(/lot 0 sur 10/)).toBeInTheDocument()
    expect(screen.queryByText('Cette génération')).not.toBeInTheDocument()
  })
})

describe('AIGenerateModal — état EN COURS (maquette §3)', () => {
  afterEach(() => vi.clearAllMocks())

  it('shows the batch counter, cumulative created/skipped counts, and the provider label', () => {
    renderModal({ aiJob: runningJob({ batchesDone: 3, batchesTotal: 10, createdCount: 58, skippedCount: 2, provider: 'groq' }) })
    expect(screen.getByText(/lot 3 sur 10/)).toBeInTheDocument()
    // Grouping (one line vs several nodes) isn't pinned down — check the counts and
    // their labels are present in the rendered document, not their exact DOM shape.
    const bodyText = document.body.textContent
    expect(bodyText).toMatch(/58/)
    expect(bodyText).toMatch(/cr[ée]{2}e/i)
    expect(bodyText).toMatch(/[ée]cart[ée]e/i)
    expect(screen.getByText(/Groq/i)).toBeInTheDocument()
  })

  it('mentions that questions appear in the list as they are created, and that the modal can be closed', () => {
    renderModal({ aiJob: runningJob() })
    expect(screen.getByText(/apparaissent dans la liste/)).toBeInTheDocument()
  })

  it('renders both "Arrêter" and "Fermer" buttons while RUNNING', () => {
    renderModal({ aiJob: runningJob() })
    expect(screen.getByText('Arrêter')).toBeInTheDocument()
    expect(screen.getByText('Fermer')).toBeInTheDocument()
  })

  it('"Fermer" closes the modal but does NOT cancel the job (changement vs #8: fermeture désormais autorisée)', () => {
    const props = renderModal({ aiJob: runningJob() })
    fireEvent.click(screen.getByText('Fermer'))
    expect(props.onClose).toHaveBeenCalled()
    expect(props.onCancelGeneration).not.toHaveBeenCalled()
  })

  it('"×" and Échap also close the modal while RUNNING (no longer blocked, unlike #8)', () => {
    const props = renderModal({ aiJob: runningJob() })
    fireEvent.click(screen.getByLabelText('Fermer'))
    expect(props.onClose).toHaveBeenCalledTimes(1)

    fireEvent.keyDown(window, { key: 'Escape' })
    expect(props.onClose).toHaveBeenCalledTimes(2)
  })

  it('"Arrêter" calls onCancelGeneration with the running job\'s id', () => {
    const props = renderModal({ aiJob: runningJob({ jobId: 'gen-xyz' }) })
    fireEvent.click(screen.getByText('Arrêter'))
    expect(props.onCancelGeneration).toHaveBeenCalledWith('gen-xyz')
  })

  it('"Arrêter" does not by itself close the modal (closing is a separate action)', () => {
    const props = renderModal({ aiJob: runningJob() })
    fireEvent.click(screen.getByText('Arrêter'))
    expect(props.onClose).not.toHaveBeenCalled()
  })
})

describe('AIGenerateModal — état TERMINÉ (maquette §4)', () => {
  afterEach(() => vi.clearAllMocks())

  it('shows the created count once STATE=DONE', () => {
    renderModal({ aiJob: runningJob({ state: 'DONE', createdCount: 187, skippedCount: 0, batchesDone: 10, batchesTotal: 10 }) })
    expect(screen.getByText('187 questions créées.')).toBeInTheDocument()
  })

  it('shows a skipped-questions warning mentioning the count when skippedCount > 0', () => {
    renderModal({ aiJob: runningJob({ state: 'DONE', createdCount: 187, skippedCount: 13 }) })
    expect(screen.getByText(/13 question/)).toBeInTheDocument()
    expect(screen.getByText(/écartée/)).toBeInTheDocument()
  })

  it('omits the skipped-questions warning when skippedCount is 0', () => {
    renderModal({ aiJob: runningJob({ state: 'DONE', createdCount: 20, skippedCount: 0 }) })
    expect(screen.queryByText(/écartée/)).not.toBeInTheDocument()
  })

  it('shows "Fermer" and "Nouvelle génération", no "Arrêter"', () => {
    renderModal({ aiJob: runningJob({ state: 'DONE', createdCount: 20 }) })
    expect(screen.getByText('Fermer')).toBeInTheDocument()
    expect(screen.getByText('Nouvelle génération')).toBeInTheDocument()
    expect(screen.queryByText('Arrêter')).not.toBeInTheDocument()
  })

  it('closing after DONE calls onClose', () => {
    const props = renderModal({ aiJob: runningJob({ state: 'DONE', createdCount: 20 }) })
    fireEvent.click(screen.getByText('Fermer'))
    expect(props.onClose).toHaveBeenCalled()
  })

  // Bug UX (retour utilisateur, reproduit) — `aiJob` n'est jamais réinitialisé après un
  // job terminé : sans cette action, refermer puis rouvrir la modale réaffichait
  // indéfiniment ce même panneau TERMINÉ, sans aucune issue vers le formulaire.
  it('"Nouvelle génération" returns to the form WITHOUT closing the modal (unlike Fermer)', () => {
    const props = renderModal({ aiJob: runningJob({ state: 'DONE', createdCount: 20 }) })
    fireEvent.click(screen.getByText('Nouvelle génération'))
    expect(screen.getByText('Cette génération')).toBeInTheDocument()
    expect(props.onClose).not.toHaveBeenCalled()
  })
})

describe('AIGenerateModal — état ARRÊTÉ (maquette §5)', () => {
  afterEach(() => vi.clearAllMocks())

  it('shows the cancellation message with the number of questions kept', () => {
    renderModal({ aiJob: runningJob({ state: 'CANCELLED', createdCount: 60 }) })
    expect(screen.getByText(/60 questions conservées/)).toBeInTheDocument()
  })

  it('never says "arrêtée" without stating how many questions were kept, even at 0', () => {
    renderModal({ aiJob: runningJob({ state: 'CANCELLED', createdCount: 0 }) })
    expect(screen.getByText(/0 questions? conservées?/)).toBeInTheDocument()
  })

  it('shows "Nouvelle génération" (same dead-end bug as DONE — aiJob never resets on its own)', () => {
    const props = renderModal({ aiJob: runningJob({ state: 'CANCELLED', createdCount: 5 }) })
    fireEvent.click(screen.getByText('Nouvelle génération'))
    expect(screen.getByText('Cette génération')).toBeInTheDocument()
    expect(props.onClose).not.toHaveBeenCalled()
  })
})

describe('AIGenerateModal — état ÉCHEC (maquette §5, §6.3 de 8-generateur-ia.md)', () => {
  afterEach(() => vi.clearAllMocks())

  it('shows the consecutive-failures message with the number of questions kept', () => {
    renderModal({ aiJob: runningJob({ state: 'FAILED', createdCount: 120, errorCode: 'upstream_error' }) })
    expect(screen.getByText(/2 échecs consécutifs/)).toBeInTheDocument()
    expect(screen.getByText(/120 questions conservées/)).toBeInTheDocument()
  })

  it('maps errorCode=provider_quota to the dedicated daily-quota message', () => {
    renderModal({ aiJob: runningJob({ state: 'FAILED', createdCount: 40, errorCode: 'provider_quota' }) })
    expect(screen.getByText(/Quota quotidien du provider atteint/)).toBeInTheDocument()
  })

  // issue #142 — ERROR_MESSAGE (détail assaini du provider) en complément du
  // message générique dérivé d'ERROR_CODE, pas à sa place.
  it('shows the sanitized provider ERROR_MESSAGE as a collapsible technical detail when present', () => {
    renderModal({ aiJob: runningJob({
      state: 'FAILED',
      createdCount: 40,
      errorCode: 'upstream_error',
      errorMessage: 'discriminator: multiple candidate properties',
    }) })
    // The generic errorCode-derived message is still shown as the main text.
    expect(screen.getByText("Le serveur n'a pas pu joindre le provider IA.")).toBeInTheDocument()
    expect(screen.getByText('Détail technique')).toBeInTheDocument()
    expect(screen.getByText('discriminator: multiple candidate properties')).toBeInTheDocument()
  })

  it('does not render the technical detail block when ERROR_MESSAGE is absent (server prior to #142)', () => {
    renderModal({ aiJob: runningJob({ state: 'FAILED', createdCount: 40, errorCode: 'upstream_error' }) })
    expect(screen.queryByText('Détail technique')).not.toBeInTheDocument()
  })

  it('"Réessayer" returns to the form with previously entered values kept', () => {
    renderModal({ aiJob: runningJob({ state: 'FAILED', createdCount: 10, errorCode: 'timeout' }) })
    fireEvent.click(screen.getByText('Réessayer'))
    expect(screen.getByText('Cette génération')).toBeInTheDocument()
    // Retour QUALIF (v6.0.7) — Thème n'est plus un champ éditable local,
    // c'est un rappel en lecture seule sourcé depuis la prop quizTheme (par
    // défaut "Cinéma français des années 80" dans baseProps ci-dessus).
    expect(screen.getByText(/Cinéma français des années 80/)).toBeInTheDocument()
  })

  it('"Fermer" on the failure panel closes the modal without retrying', () => {
    const props = renderModal({ aiJob: runningJob({ state: 'FAILED', createdCount: 10 }) })
    fireEvent.click(screen.getByText('Fermer'))
    expect(props.onClose).toHaveBeenCalled()
  })
})
