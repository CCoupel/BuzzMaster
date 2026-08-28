import { describe, it, expect, afterEach, vi } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import AIGenerateModal from './AIGenerateModal'

// Tests dérivés de _work/mockups/8-generateur-ia.md (#8, v6.0.0) — §2 à §5 (le
// formulaire, inchangé par #137) — et de contracts/ai-generation.md §3 (forme du
// payload envoyé, inchangée). Les états post-soumission (§6/§7 de 8-generateur-ia.md)
// ont été REMPLACÉS par #137 (contracts/ai-multi-provider.md §9 — **BREAKING**,
// documenté dans contracts/CHANGELOG.md [20260805b]) : `POST
// /api/generate-questions` ne renvoie plus le résultat en synchrone, il renvoie
// `202 + job_id` et la suite est pilotée par la prop `aiJob` (alimentée par
// `AI_GENERATION_PROGRESS`, contrat §10). Les anciens describes "machine à états" et
// "mapping code -> message", écrits pour le flux HTTP synchrone de #8, testaient donc
// un chemin retiré — 15/36 tests cassés à la livraison de dev-frontend
// (`_work/handoff/dev-frontend-20260805-221513.md`), corrigés ici.
//
// Le détail des états EN COURS / TERMINÉ / ARRÊTÉ / ÉCHEC (contenu affiché,
// Arrêter/Fermer, ré-attachement) est déjà couvert par
// `AIGenerateModal.progress.test.jsx` (#137, 22/22 PASS d'après dev-frontend) — ne
// PAS le dupliquer ici, dev-frontend l'a explicitement utilisé comme garde-fou
// pendant l'implémentation. Ce fichier se limite désormais à : le formulaire
// (inchangé), le payload de soumission (inchangé), et ce qui est spécifique au
// MOMENT de la soumission (202 → passage en attente de job, rejets synchrones
// 400/405/409/507 avant qu'aucun job n'existe, `onGenerated` appelé avec un ID de
// question — pas un objet résultat comme en #8).
//
// framer-motion est aliasé globalement en mock via vite.config.js test.alias
// (cf. QuestionsPage.v571.test.jsx) — Button/Card ne sont pas re-mockés ici,
// ils rendent réellement pour exercer le vrai DOM (boutons, disabled, etc).
//
// L'algorithme de rebalance (rebalanceOnSlide/rebalanceOnToggle) n'est pas
// exporté par AIGenerateModal.jsx (fonctions privées au module) : ces tests
// l'exercent donc exclusivement via l'UI (déplacer les <input type="range">,
// cocher/décocher les toggles), comme le reste de la codebase teste ses pages
// (aucun test ne réimporte de fonction interne ailleurs dans ce dépôt).

const CATEGORIES = [
  { key: 'ENTERTAINMENT', name: 'Divertissement', color: '#ec4899' },
  { key: 'SCIENCE', name: 'Sciences & Nature', color: '#22c55e' },
]

function defaultModalProps(overrides = {}) {
  return {
    onClose: vi.fn(),
    apiKeyConfigured: true,
    provider: 'anthropic',
    categories: CATEGORIES,
    quizTheme: 'Cinéma français des années 80',
    quizPopulations: ['Adulte (18-64 ans)'],
    quizDifficulties: ['Moyen'],
    quizLanguage: 'Français',
    quizObjectives: '',
    hasUnsavedQuizChanges: false,
    questions: {},
    aiJob: null,
    onCancelGeneration: vi.fn(),
    onGenerated: vi.fn(),
    onNavigateToQuizSettings: vi.fn(),
    ...overrides,
  }
}

function modalTree(props, route) {
  return (
    <MemoryRouter initialEntries={[route]}>
      <Routes>
        <Route path="/admin/settings" element={<div data-testid="settings-page">Settings</div>} />
        <Route path="*" element={<AIGenerateModal {...props} />} />
      </Routes>
    </MemoryRouter>
  )
}

function renderModal(overrides = {}, { route = '/admin/questions' } = {}) {
  const props = defaultModalProps(overrides)
  const { rerender } = render(modalTree(props, route))
  return {
    props,
    // Re-renders with `props` shallow-merged with `moreOverrides` — used to
    // simulate a WS-driven `aiJob`/`questions` update arriving after submission
    // (contrat §10 : AI_GENERATION_PROGRESS met à jour l'état, pas la réponse HTTP).
    update(moreOverrides) {
      Object.assign(props, moreOverrides)
      rerender(modalTree(props, route))
    },
  }
}

// Range inputs carry no accessible name (only the type toggle checkboxes do,
// via aria-label="Activer <Type>") — they are selected by DOM order, which
// matches the fixed GENERABLE_TYPES order: SPEEDY, QCM, MEMORY, MEMOTION,
// MEMOTION_PLUS, ARDOISE (#196 — MEMOTION_PLUS inséré juste après MEMOTION,
// avant ARDOISE, questionTypeMeta.js).
function distributionSliders(container) {
  return Array.from(container.querySelectorAll('input[type="range"]'))
}

function distributionPercents(container) {
  return Array.from(container.querySelectorAll('.ai-distribution-pct')).map(el => el.textContent)
}

// Selects at least one category so `canSubmit` only depends on whatever the
// test is actually exercising (theme/difficulty/language default from props).
function selectFirstCategory() {
  fireEvent.click(screen.getByText('Divertissement'))
}

describe('AIGenerateModal — état indisponible (maquette §5)', () => {
  afterEach(() => vi.clearAllMocks())

  it('shows the unavailable panel when apiKeyConfigured is false', () => {
    renderModal({ apiKeyConfigured: false })
    expect(screen.getByText(/Génération indisponible/)).toBeInTheDocument()
    expect(screen.getByText('🔌')).toBeInTheDocument()
    expect(screen.getByText('Configurer une clé API')).toBeInTheDocument()
    // The form must not render in this state.
    expect(screen.queryByText('Cette génération')).not.toBeInTheDocument()
  })

  it('"Configurer une clé API" navigates to /admin/settings and closes the modal', async () => {
    const { props } = renderModal({ apiKeyConfigured: false }, { route: '/admin/questions' })
    fireEvent.click(screen.getByText('Configurer une clé API'))

    await waitFor(() => {
      expect(screen.getByTestId('settings-page')).toBeInTheDocument()
    })
    expect(props.onClose).toHaveBeenCalled()
  })

  // #155 (F2, BREAKING) — the "navigates to /anim/settings when opened from
  // an /anim route" case that used to live here is removed, not adapted:
  // /anim used to be an alias serving the admin interface (this modal
  // included) ; it now serves its own page (AnimPage), which never renders
  // AIGenerateModal. "Configurer une clé API" navigating to /admin/settings
  // (previous test) is therefore the only case left — there is no /anim
  // variant anymore.
})

describe('AIGenerateModal — formulaire (maquette §2, retour QUALIF v6.0.7)', () => {
  afterEach(() => vi.clearAllMocks())

  // Retour QUALIF (#137) — Thème/Population/Difficulté/Langue ne sont plus
  // un formulaire éditable : lus directement depuis les props et affichés en
  // lecture seule (règle absolue inchangée : aucune écriture vers le
  // GameState global ne part de cette modale).
  it('shows Thème/Population/Difficulté/Langue read-only, sourced from the quiz globals passed as props', () => {
    renderModal()
    // Each value renders as a text sibling of a <strong> label (e.g. "Thème :
    // Cinéma..."), so a substring regex is used rather than an exact string
    // match against the whole "<label> <value>" element.
    expect(screen.getByText(/Cinéma français des années 80/)).toBeInTheDocument()
    expect(screen.getByText(/Adulte \(18-64 ans\)/)).toBeInTheDocument()
    expect(screen.getByText(/Moyen/)).toBeInTheDocument()
    expect(screen.getByText(/Français/)).toBeInTheDocument()
    // No editable control for these 4 fields anymore.
    expect(screen.queryByPlaceholderText('Ex : Cinéma français des années 80')).not.toBeInTheDocument()
  })

  it('"✨ Générer" is disabled while no category is selected, even with theme/difficulty valid', () => {
    renderModal()
    expect(screen.getByText('✨ Générer').closest('button')).toBeDisabled()
  })

  it('"✨ Générer" becomes enabled once a category is selected', () => {
    renderModal()
    selectFirstCategory()
    expect(screen.getByText('✨ Générer').closest('button')).not.toBeDisabled()
  })

  it('"✨ Générer" is disabled when the global theme is empty (nothing to fall back to, no local field to fill in)', () => {
    renderModal({ quizTheme: '' })
    selectFirstCategory()
    expect(screen.getByText('✨ Générer').closest('button')).toBeDisabled()
  })

  it('"✨ Générer" is disabled when no difficulty is selected', () => {
    renderModal({ quizDifficulties: [] })
    selectFirstCategory()
    expect(screen.getByText('✨ Générer').closest('button')).toBeDisabled()
  })

  it('"✨ Générer" is disabled when no population is selected', () => {
    renderModal({ quizPopulations: [] })
    selectFirstCategory()
    expect(screen.getByText('✨ Générer').closest('button')).toBeDisabled()
  })

  it('the "modifier" link calls onNavigateToQuizSettings', () => {
    const { props } = renderModal()
    fireEvent.click(screen.getByText('modifier'))
    expect(props.onNavigateToQuizSettings).toHaveBeenCalled()
  })
})

describe('AIGenerateModal — rebalance des sliders (maquette §3, normatif)', () => {
  afterEach(() => vi.clearAllMocks())

  it('renders the default distribution 40/40/20/0/0/0 with MEMOTION, MEMOTION_PLUS and ARDOISE off', () => {
    const { container } = render(
      <MemoryRouter><AIGenerateModal onClose={vi.fn()} apiKeyConfigured categories={CATEGORIES} /></MemoryRouter>
    )
    expect(distributionPercents(container)).toEqual(['40%', '40%', '20%', '0%', '0%', '0%'])
    const sliders = distributionSliders(container)
    expect(sliders[3]).toBeDisabled() // MEMOTION, OFF by default
    expect(sliders[4]).toBeDisabled() // MEMOTION_PLUS (#196), OFF by default
    expect(sliders[5]).toBeDisabled() // ARDOISE (T2.3), OFF by default
  })

  it('proportional rebalance: moving SPEEDY to 100 drives QCM and MEMORY to 0', () => {
    const { container } = render(
      <MemoryRouter><AIGenerateModal onClose={vi.fn()} apiKeyConfigured categories={CATEGORIES} /></MemoryRouter>
    )
    const [speedy] = distributionSliders(container)
    fireEvent.change(speedy, { target: { value: '100' } })
    expect(distributionPercents(container)).toEqual(['100%', '0%', '0%', '0%', '0%', '0%'])
  })

  it('othersSum===0 branch: equal split when the two other active types are both at 0', () => {
    const { container } = render(
      <MemoryRouter><AIGenerateModal onClose={vi.fn()} apiKeyConfigured categories={CATEGORIES} /></MemoryRouter>
    )
    const [speedy] = distributionSliders(container)
    // Step 1 — drive QCM and MEMORY to 0 (proportional branch, remaining=0).
    fireEvent.change(speedy, { target: { value: '100' } })
    expect(distributionPercents(container)).toEqual(['100%', '0%', '0%', '0%', '0%', '0%'])
    // Step 2 — now move SPEEDY down: others (QCM, MEMORY) are both at 0, so
    // remaining (50) must split EQUALLY between them (25/25), not stay at 0.
    fireEvent.change(speedy, { target: { value: '50' } })
    expect(distributionPercents(container)).toEqual(['50%', '25%', '25%', '0%', '0%', '0%'])
  })

  it('sum of active types is always 100 after a slide', () => {
    const { container } = render(
      <MemoryRouter><AIGenerateModal onClose={vi.fn()} apiKeyConfigured categories={CATEGORIES} /></MemoryRouter>
    )
    const [, qcm] = distributionSliders(container)
    fireEvent.change(qcm, { target: { value: '73' } })
    const total = distributionPercents(container).reduce((sum, p) => sum + parseInt(p, 10), 0)
    expect(total).toBe(100)
  })

  it('toggling a type OFF redistributes its share proportionally to the remaining active types', () => {
    const { container } = render(
      <MemoryRouter><AIGenerateModal onClose={vi.fn()} apiKeyConfigured categories={CATEGORIES} /></MemoryRouter>
    )
    // Default: SPEEDY 40 / QCM 40 / MEMORY 20 / MEMOTION 0(off) /
    // MEMOTION_PLUS 0(off) / ARDOISE 0(off).
    fireEvent.click(screen.getByLabelText('Activer QCM'))
    // QCM's 40 splits proportionally between SPEEDY(40) and MEMORY(20):
    // SPEEDY += round(40*40/60)=27 -> 67 ; MEMORY (last) absorbs -> 100-67=33.
    expect(distributionPercents(container)).toEqual(['67%', '0%', '33%', '0%', '0%', '0%'])
    expect(distributionSliders(container)[1]).toBeDisabled()
  })

  it('toggling a type ON resets it to 20% then rebalances the rest (maquette §3)', () => {
    const { container } = render(
      <MemoryRouter><AIGenerateModal onClose={vi.fn()} apiKeyConfigured categories={CATEGORIES} /></MemoryRouter>
    )
    fireEvent.click(screen.getByLabelText('Activer Memotion'))
    // remaining=80 shared proportionally among SPEEDY(40)/QCM(40)/MEMORY(20)
    // over their sum 100: SPEEDY+=32, QCM+=32, MEMORY(last)=80-64=16.
    // MEMOTION_PLUS/ARDOISE stay disabled at 0, untouched by this rebalance.
    expect(distributionPercents(container)).toEqual(['32%', '32%', '16%', '20%', '0%', '0%'])
  })

  // #196 — MEMOTION_PLUS suit exactement le même traitement que MEMOTION
  // (nouveau type désactivé par défaut) : même algorithme de rebalance,
  // aucune branche spéciale requise (F3 du plan cycle #196).
  it('#196 — toggling MEMOTION_PLUS ON resets it to 20% then rebalances the rest, same as MEMOTION', () => {
    const { container } = render(
      <MemoryRouter><AIGenerateModal onClose={vi.fn()} apiKeyConfigured categories={CATEGORIES} /></MemoryRouter>
    )
    fireEvent.click(screen.getByLabelText('Activer MEMOTION+'))
    expect(distributionPercents(container)).toEqual(['32%', '32%', '16%', '0%', '20%', '0%'])
    const total = distributionPercents(container).reduce((sum, p) => sum + parseInt(p, 10), 0)
    expect(total).toBe(100)
  })

  it('#196 — toggling MEMOTION_PLUS OFF after activation redistributes its share back proportionally', () => {
    const { container } = render(
      <MemoryRouter><AIGenerateModal onClose={vi.fn()} apiKeyConfigured categories={CATEGORIES} /></MemoryRouter>
    )
    fireEvent.click(screen.getByLabelText('Activer MEMOTION+')) // ON: 20%
    fireEvent.click(screen.getByLabelText('Activer MEMOTION+')) // OFF again
    expect(distributionPercents(container)).toEqual(['40%', '40%', '20%', '0%', '0%', '0%'])
    expect(distributionSliders(container)[4]).toBeDisabled()
  })

  it('cas limite — disabling all but one type leaves the survivor at 100%', () => {
    const { container } = render(
      <MemoryRouter><AIGenerateModal onClose={vi.fn()} apiKeyConfigured categories={CATEGORIES} /></MemoryRouter>
    )
    fireEvent.click(screen.getByLabelText('Activer QCM'))
    fireEvent.click(screen.getByLabelText('Activer Memory'))
    expect(distributionPercents(container)).toEqual(['100%', '0%', '0%', '0%', '0%', '0%'])
  })

  it('cas limite — disabling every type disables "✨ Générer" (maquette §3 fin)', () => {
    renderModal()
    selectFirstCategory() // satisfy every other canSubmit condition first
    fireEvent.click(screen.getByLabelText('Activer Speedy'))
    fireEvent.click(screen.getByLabelText('Activer QCM'))
    fireEvent.click(screen.getByLabelText('Activer Memory'))
    expect(screen.getByText('✨ Générer').closest('button')).toBeDisabled()
  })
})

describe('AIGenerateModal — payload envoyé (contrat §3)', () => {
  afterEach(() => vi.clearAllMocks())

  it('POSTs /api/generate-questions with the exact contract field names', async () => {
    global.fetch = vi.fn(() => new Promise(() => {})) // never resolves — we only inspect the call
    renderModal()
    selectFirstCategory()
    fireEvent.click(screen.getByText('✨ Générer'))

    await waitFor(() => expect(global.fetch).toHaveBeenCalled())
    const [url, options] = global.fetch.mock.calls[0]
    expect(url).toBe('/api/generate-questions')
    expect(options.method).toBe('POST')
    const body = JSON.parse(options.body)
    expect(body).toEqual({
      theme: 'Cinéma français des années 80',
      populations: ['Adulte (18-64 ans)'],
      language: 'Français',
      difficulties: ['Moyen'],
      objectives: '',
      instructions: '',
      categories: ['ENTERTAINMENT'],
      volume: { mode: 'count', value: 20 },
      distribution: { SPEEDY: 40, QCM: 40, MEMORY: 20, MEMOTION: 0, MEMOTION_PLUS: 0, ARDOISE: 0 },
    })
  })

  it('switches to duration mode and sends volume={mode:"duration",value}', async () => {
    global.fetch = vi.fn(() => new Promise(() => {}))
    renderModal()
    selectFirstCategory()
    fireEvent.click(screen.getByText('Durée de partie'))
    fireEvent.click(screen.getByText('✨ Générer'))

    await waitFor(() => expect(global.fetch).toHaveBeenCalled())
    const body = JSON.parse(global.fetch.mock.calls[0][1].body)
    expect(body.volume).toEqual({ mode: 'duration', value: 45 })
  })
})

describe('AIGenerateModal — soumission asynchrone (contrat ai-multi-provider.md §9)', () => {
  afterEach(() => vi.clearAllMocks())

  it('202 + job_id shows the loading panel and starts tracking the job (no more synchronous result)', async () => {
    global.fetch = vi.fn(() => Promise.resolve({
      status: 202,
      json: async () => ({ status: 'accepted', job_id: 'gen-20260806-000000-a1b2', batches_total: 3 }),
    }))
    renderModal()
    selectFirstCategory()
    fireEvent.click(screen.getByText('✨ Générer'))

    await waitFor(() => {
      // aiJob is still null at this point (no AI_GENERATION_PROGRESS received yet) —
      // RunningBody falls back to the plain message (contrat §10, maquette §3).
      expect(screen.getByText('Génération en cours…')).toBeInTheDocument()
    })
    // The old #8 single-call framing ("1 à 3 minutes") is gone — replaced by the
    // per-batch progress owned by AIGenerateModal.progress.test.jsx.
    expect(screen.queryByText(/1 à 3 minutes/)).not.toBeInTheDocument()
  })

  it('"×" and the footer "Fermer" both close the modal right after submitting, even before any progress update — job continues in background (changement vs #8)', async () => {
    global.fetch = vi.fn(() => new Promise(() => {})) // 202 never arrives within this test
    const { props } = renderModal()
    selectFirstCategory()
    fireEvent.click(screen.getByText('✨ Générer'))
    await waitFor(() => expect(screen.getByText('Génération en cours…')).toBeInTheDocument())

    fireEvent.click(screen.getByLabelText('Fermer')) // header × (aria-label)
    expect(props.onClose).toHaveBeenCalledTimes(1)

    fireEvent.click(screen.getByText('Fermer')) // footer button (visible text)
    expect(props.onClose).toHaveBeenCalledTimes(2)
  })

  it('Escape closes the modal both in the form state and right after submitting (no longer blocked, unlike #8)', async () => {
    const { props } = renderModal()
    fireEvent.keyDown(window, { key: 'Escape' })
    expect(props.onClose).toHaveBeenCalledTimes(1)

    global.fetch = vi.fn(() => new Promise(() => {}))
    selectFirstCategory()
    fireEvent.click(screen.getByText('✨ Générer'))
    await waitFor(() => expect(screen.getByText('Génération en cours…')).toBeInTheDocument())
    fireEvent.keyDown(window, { key: 'Escape' })
    expect(props.onClose).toHaveBeenCalledTimes(2) // called again — no longer blocked while a job runs
  })

  it('closing after the job reaches DONE (via aiJob, contrat §10) calls onGenerated with the id of the first newly created question, then onClose', async () => {
    global.fetch = vi.fn(() => Promise.resolve({
      status: 202,
      json: async () => ({ status: 'accepted', job_id: 'gen-1', batches_total: 1 }),
    }))
    const initialQuestions = { 5: { ID: '5', CATEGORY: 'ENTERTAINMENT' } }
    const { props, update } = renderModal({ questions: initialQuestions })
    selectFirstCategory()
    fireEvent.click(screen.getByText('✨ Générer'))
    await waitFor(() => expect(screen.getByText('Génération en cours…')).toBeInTheDocument())

    // Simulate what the parent actually does on AI_GENERATION_PROGRESS: `questions`
    // grows (broadcastQuestions after the batch) and `aiJob` reaches DONE. Unlike
    // #8's `onGenerated(result)`, #137 passes a question ID — no result object is
    // ever returned by the (now 202) HTTP response.
    update({
      questions: { ...initialQuestions, 12: { ID: '12', CATEGORY: 'ENTERTAINMENT' } },
      aiJob: { jobId: 'gen-1', state: 'DONE', batchesDone: 1, batchesTotal: 1, createdCount: 1, skippedCount: 0, errorCode: '', provider: 'anthropic' },
    })

    await waitFor(() => expect(screen.getByText('1 question créée.')).toBeInTheDocument())
    fireEvent.click(screen.getByText('Fermer'))

    expect(props.onGenerated).toHaveBeenCalledWith('12')
    expect(props.onClose).toHaveBeenCalled()
  })
})

// Le détail des panneaux EN COURS / TERMINÉ / ARRÊTÉ / ÉCHEC (contenu affiché,
// Arrêter/Fermer, ré-attachement, mapping ERROR_CODE -> message) est couvert par
// AIGenerateModal.progress.test.jsx — volontairement PAS dupliqué ici (dev-frontend
// s'en est servi comme garde-fou réel pendant l'implémentation, cf. handoff
// _work/handoff/dev-frontend-20260805-221513.md).

describe('AIGenerateModal — rejets synchrones à la soumission (contrat §9 : 400/405/409/507)', () => {
  afterEach(() => vi.clearAllMocks())

  // Distinct de jobErrorMessage (job déjà démarré, couvert par
  // AIGenerateModal.progress.test.jsx) : ici, la requête POST elle-même est rejetée
  // AVANT qu'aucun job_id n'existe — panneau "submit-error", pas "failed".
  function submitAndReject(response) {
    global.fetch = vi.fn(() => Promise.resolve(response))
    renderModal()
    selectFirstCategory()
    fireEvent.click(screen.getByText('✨ Générer'))
  }

  it('no_api_key -> "Clé API invalide ou absente..." with a configure-key button (libellé générique multi-provider, plus "Claude")', async () => {
    submitAndReject({ status: 409, json: async () => ({ status: 'error', code: 'no_api_key', message: 'Aucune clé API configurée.' }) })
    await waitFor(() => {
      expect(screen.getByText('Clé API invalide ou absente. Vérifiez-la dans Paramètres.')).toBeInTheDocument()
    })
    expect(screen.getByText('Configurer une clé API')).toBeInTheDocument()
  })

  it('generation_in_progress -> dedicated message (race: a job started between opening the modal and submitting)', async () => {
    submitAndReject({ status: 409, json: async () => ({ status: 'error', code: 'generation_in_progress' }) })
    await waitFor(() => {
      expect(screen.getByText('Une génération est déjà en cours. Fermez et rouvrez la fenêtre pour suivre sa progression.')).toBeInTheDocument()
    })
    // This is a synchronous rejection, not a job — no "Configurer une clé API" link.
    expect(screen.queryByText('Configurer une clé API')).not.toBeInTheDocument()
  })

  it('an unrecognized/generic rejection code (e.g. invalid_request) shows the generic message with a collapsible technical detail', async () => {
    submitAndReject({ status: 400, json: async () => ({ status: 'error', code: 'invalid_request', message: 'somme des distributions != 100' }) })
    await waitFor(() => {
      expect(screen.getByText('Erreur pendant la génération.')).toBeInTheDocument()
    })
    expect(screen.getByText('Détail technique')).toBeInTheDocument()
    fireEvent.click(screen.getByText('Détail technique'))
    expect(screen.getByText('somme des distributions != 100')).toBeInTheDocument()
  })

  it('a client-side network failure (fetch throws) before any response -> network message, no config-key button', async () => {
    global.fetch = vi.fn(() => Promise.reject(new Error('Failed to fetch')))
    renderModal()
    selectFirstCategory()
    fireEvent.click(screen.getByText('✨ Générer'))

    await waitFor(() => {
      expect(screen.getByText("Le serveur n'a pas pu être joint. Vérifiez l'accès réseau.")).toBeInTheDocument()
    })
    expect(screen.queryByText('Configurer une clé API')).not.toBeInTheDocument()
  })

  it('"Réessayer" returns to the form with the previously entered values kept', async () => {
    submitAndReject({ status: 400, json: async () => ({ status: 'error', code: 'invalid_request' }) })
    await waitFor(() => {
      expect(screen.getByText('Erreur pendant la génération.')).toBeInTheDocument()
    })
    fireEvent.click(screen.getByText('Réessayer'))

    expect(screen.getByText('Cette génération')).toBeInTheDocument()
    // Thème/Population/Difficulté/Langue are read-only, sourced from props —
    // "kept" trivially holds for them (retour QUALIF v6.0.7, cf. describe
    // "formulaire" above). What "Réessayer" actually needs to preserve is
    // Bloc 2's local state: the category selected before submit must still
    // be checked.
    expect(screen.getByText('Divertissement').closest('button')).toHaveClass('active')
  })

  it('"Fermer" on the submit-error panel closes the modal without retrying and without calling onGenerated', async () => {
    const fetchMock = vi.fn(() => Promise.resolve({ status: 400, json: async () => ({ status: 'error', code: 'invalid_request' }) }))
    global.fetch = fetchMock
    const { props } = renderModal()
    selectFirstCategory()
    fireEvent.click(screen.getByText('✨ Générer'))
    await waitFor(() => expect(screen.getByText('Erreur pendant la génération.')).toBeInTheDocument())

    fetchMock.mockClear()
    fireEvent.click(screen.getByText('Fermer'))
    expect(props.onClose).toHaveBeenCalled()
    expect(props.onGenerated).not.toHaveBeenCalled()
    expect(fetchMock).not.toHaveBeenCalled()
  })
})
