import { describe, it, expect, afterEach, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import AIJobModalShell from './AIJobModalShell'

// Tests dérivés de contracts/rafale-ai-generation.md §6 ("Rejoué à la
// connexion... deux modales distinctes l'écoutent désormais") et §6bis
// ("Filtrage du job sur TARGET... implémenté ICI, une seule fois") — issue
// #203, v8.1.0, risque R6 du plan ("Élevée sans mitigation").
//
// AIJobModalShell.jsx est la coquille PARTAGÉE par AIGenerateModal.jsx
// (Quiz) et RafaleAIGenerateModal.jsx — le filtrage sur `target` (via
// `matchesTarget`, aiJobHelpers.js) est implémenté une seule fois ici et
// doit fonctionner dans les DEUX sens : un job Quiz ne doit jamais faire
// basculer une modale Rafale en "en cours" (et réciproquement).
//
// Ce fichier teste la coquille de façon isolée (pas via l'une ou l'autre
// modale concrète) avec un renderForm() minimal — AIGenerateModal.test.jsx/
// .progress.test.jsx/.tooltip.test.jsx/.unsavedBanner.test.jsx restent le
// filet de non-régression du chemin Quiz complet (critère bloquant R10, non
// touché par ce fichier).

function shellProps(overrides = {}) {
  return {
    title: 'Test Modal',
    target: 'QUIZ',
    endpoint: '/api/test-endpoint',
    buildPayload: () => ({}),
    canSubmit: true,
    renderForm: () => <div data-testid="the-form">FORM</div>,
    apiKeyConfigured: true,
    provider: 'anthropic',
    aiJob: null,
    onCancelGeneration: vi.fn(),
    onClose: vi.fn(),
    onNavigateToSettings: vi.fn(),
    ...overrides,
  }
}

function renderShell(overrides = {}) {
  const props = shellProps(overrides)
  const { rerender } = render(<AIJobModalShell {...props} />)
  return {
    props,
    update(moreOverrides) {
      Object.assign(props, moreOverrides)
      rerender(<AIJobModalShell {...props} />)
    },
  }
}

function runningJob(target, overrides = {}) {
  return {
    jobId: 'gen-1', state: 'RUNNING', target,
    batchesDone: 0, batchesTotal: 1, createdCount: 0, skippedCount: 0,
    errorCode: '', errorMessage: '', provider: 'anthropic',
    ...overrides,
  }
}

describe('AIJobModalShell — filtrage TARGET (contrat §6/§6bis, risque R6)', () => {
  afterEach(() => vi.clearAllMocks())

  it('a QUIZ-targeted shell IGNORES a RUNNING RAFALE job — stays on the form', () => {
    renderShell({ target: 'QUIZ', aiJob: runningJob('RAFALE') })
    expect(screen.getByTestId('the-form')).toBeInTheDocument()
    expect(screen.queryByText(/Génération en cours/)).not.toBeInTheDocument()
  })

  it('a RAFALE-targeted shell IGNORES a RUNNING QUIZ job — stays on the form', () => {
    renderShell({ target: 'RAFALE', aiJob: runningJob('QUIZ') })
    expect(screen.getByTestId('the-form')).toBeInTheDocument()
    expect(screen.queryByText(/Génération en cours/)).not.toBeInTheDocument()
  })

  it('a QUIZ-targeted shell ADOPTS a matching RUNNING QUIZ job — switches to the running panel', () => {
    renderShell({ target: 'QUIZ', aiJob: runningJob('QUIZ') })
    expect(screen.getByText(/Génération en cours/)).toBeInTheDocument()
    expect(screen.queryByTestId('the-form')).not.toBeInTheDocument()
  })

  it('a RAFALE-targeted shell ADOPTS a matching RUNNING RAFALE job — switches to the running panel', () => {
    renderShell({ target: 'RAFALE', aiJob: runningJob('RAFALE') })
    expect(screen.getByText(/Génération en cours/)).toBeInTheDocument()
    expect(screen.queryByTestId('the-form')).not.toBeInTheDocument()
  })

  it('a job with NO target field (legacy server, pre-#203) is treated as QUIZ — a QUIZ shell adopts it', () => {
    const { jobId, state, batchesDone, batchesTotal, createdCount, skippedCount, errorCode, errorMessage, provider } = runningJob('QUIZ')
    const legacyJob = { jobId, state, batchesDone, batchesTotal, createdCount, skippedCount, errorCode, errorMessage, provider } // no `target` key at all
    renderShell({ target: 'QUIZ', aiJob: legacyJob })
    expect(screen.getByText(/Génération en cours/)).toBeInTheDocument()
  })

  it('a job with NO target field is treated as QUIZ — a RAFALE shell must still IGNORE it', () => {
    const { jobId, state, batchesDone, batchesTotal, createdCount, skippedCount, errorCode, errorMessage, provider } = runningJob('QUIZ')
    const legacyJob = { jobId, state, batchesDone, batchesTotal, createdCount, skippedCount, errorCode, errorMessage, provider }
    renderShell({ target: 'RAFALE', aiJob: legacyJob })
    expect(screen.getByTestId('the-form')).toBeInTheDocument()
    expect(screen.queryByText(/Génération en cours/)).not.toBeInTheDocument()
  })

  it('a mismatched job updating over time (RUNNING -> DONE) never leaks into the other target\'s shell', () => {
    const { update } = renderShell({ target: 'QUIZ', aiJob: null })
    expect(screen.getByTestId('the-form')).toBeInTheDocument()

    update({ aiJob: runningJob('RAFALE') })
    expect(screen.getByTestId('the-form')).toBeInTheDocument()

    update({ aiJob: { ...runningJob('RAFALE'), state: 'DONE', createdCount: 12 } })
    // Still on the form — a DONE RAFALE job must not surface as a QUIZ success panel.
    expect(screen.getByTestId('the-form')).toBeInTheDocument()
    expect(screen.queryByText(/question.*créée/)).not.toBeInTheDocument()
  })

  it('mounting directly with an already-DONE job of a DIFFERENT target does not show the success panel (initial view state)', () => {
    renderShell({ target: 'RAFALE', apiKeyConfigured: true, aiJob: { ...runningJob('QUIZ'), state: 'DONE', createdCount: 3 } })
    expect(screen.getByTestId('the-form')).toBeInTheDocument()
  })

  it('mounting directly with an already-DONE job of the MATCHING target shows the success panel (initial view state)', () => {
    renderShell({ target: 'RAFALE', apiKeyConfigured: true, aiJob: { ...runningJob('RAFALE'), state: 'DONE', createdCount: 3 } })
    expect(screen.queryByTestId('the-form')).not.toBeInTheDocument()
    expect(screen.getByText(/3 questions créées/)).toBeInTheDocument()
  })
})
