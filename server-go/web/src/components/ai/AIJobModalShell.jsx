import { useState, useEffect, useCallback, useRef } from 'react'
import Button from '../Button'
import { jobErrorMessage, providerLabel, mapSubmitError, matchesTarget } from './aiJobHelpers'
import './AIJobModalShell.css'

// AIJobModalShell — coquille de modale de génération IA, PARTAGÉE par les
// deux chemins de génération (Quiz : AIGenerateModal.jsx, Rafale :
// RafaleAIGenerateModal.jsx, #203 v8.1.0).
//
// Extraite de AIGenerateModal.jsx (#8 v6.0.0, refondu #137 v6.1.0) —
// tâche 10 du plan (_work/reports/plan-20260901-162105.md), contrat
// rafale-ai-generation.md §6bis. DÉPLACEMENT, pas réécriture : les 6 corps
// d'état ci-dessous sont copiés tels quels depuis AIGenerateModal.jsx, ils
// ne référençaient déjà rien de propre au Quiz (le seul paramètre
// spécifique, `breakdown`, était déjà rendu sous condition côté DoneBody).
//
// 🔴 Critère bloquant R10 — ne JAMAIS modifier le comportement visible par
// AIGenerateModal.test.jsx / .progress.test.jsx / .tooltip.test.jsx /
// .unsavedBanner.test.jsx en touchant ce fichier : ces 4 fichiers doivent
// continuer à passer sans être modifiés.
//
// Porte aussi le filtrage sur `target` (contrat §6) : un `aiJob` (singleton
// global) dont le `target` diffère de celui de CETTE modale est
// intégralement ignoré (adoption, transitions, décompte) — implémenté ICI,
// une seule fois, pour servir les deux modales (remplace la tâche 12 du
// plan initial, cf. handoff planner §2bis).
//
// Ce qui reste PROPRE à chaque modale (n'est PAS dans ce fichier) : le
// formulaire (`renderForm`), la construction du payload (`buildPayload`),
// la règle `canSubmit`/`submitDisabledTitle`, et tout calcul de delta
// spécifique au Quiz (breakdown par catégorie, `onGenerated` — Rafale n'a
// délibérément pas d'équivalent, décision GATE 2 §6ter "pas d'état ajouté").

function initialViewStateFor(aiJob, apiKeyConfigured, target) {
  const relevant = matchesTarget(aiJob, target) ? aiJob : null
  switch (relevant?.state) {
    case 'RUNNING': return 'loading'
    case 'DONE': return 'success'
    case 'CANCELLED': return 'cancelled'
    case 'FAILED': return 'failed'
    default: return apiKeyConfigured ? 'form' : 'unavailable'
  }
}

// #137 — remplace le spinner opaque de #8. Barre de progression par lot,
// compteurs cumulatifs, décompte inter-lots (maquette §3).
function RunningBody({ aiJob, countdown, provider }) {
  const batchesTotal = aiJob?.batchesTotal || 0
  const batchesDone = aiJob?.batchesDone || 0
  const pct = batchesTotal > 0 ? Math.round((batchesDone / batchesTotal) * 100) : 0
  const createdCount = aiJob?.createdCount || 0
  const skippedCount = aiJob?.skippedCount || 0

  return (
    <div className="ai-modal-status ai-modal-status--running">
      <p className="ai-status-title">
        {batchesTotal > 0
          ? `Génération en cours — lot ${batchesDone} sur ${batchesTotal}`
          : 'Génération en cours…'}
      </p>

      {countdown ? (
        <p className="ai-status-countdown">Prochain lot dans {countdown}s…</p>
      ) : (
        <div className="ai-progress-wrap">
          <div className="ai-progress-bar">
            <div className="ai-progress-fill" style={{ width: `${pct}%` }} />
          </div>
          <span className="ai-progress-pct">{pct}%</span>
        </div>
      )}

      <p className="ai-status-counts">
        {createdCount} question{createdCount > 1 ? 's' : ''} créée{createdCount > 1 ? 's' : ''}
        {skippedCount > 0 && <> · {skippedCount} écartée{skippedCount > 1 ? 's' : ''}</>}
      </p>
      <p className="ai-status-provider">Provider : {providerLabel(provider)}</p>
      <p className="ai-status-sub">
        Les questions apparaissent dans la liste au fur et à mesure. Vous pouvez fermer cette
        fenêtre, la génération se poursuit.
      </p>
    </div>
  )
}

function DoneBody({ createdCount, skippedCount, breakdown }) {
  return (
    <div className="ai-modal-status">
      <div className="ai-status-icon" aria-hidden="true">✅</div>
      <p className="ai-status-title">
        {createdCount} question{createdCount > 1 ? 's' : ''} créée{createdCount > 1 ? 's' : ''}.
      </p>
      {breakdown.length > 0 && (
        <ul className="ai-success-list">
          {breakdown.map(([cat, count]) => (
            <li key={cat}>• {cat} — {count} question{count > 1 ? 's' : ''}</li>
          ))}
        </ul>
      )}
      {skippedCount > 0 && (
        <p className="ai-status-warning">
          ⚠️ {skippedCount} question{skippedCount > 1 ? 's' : ''} écartée{skippedCount > 1 ? 's' : ''} (format invalide ou catégorie inconnue).
        </p>
      )}
    </div>
  )
}

// #137 — état terminal, absent de #8 (maquette §5, §6).
function CancelledBody({ createdCount }) {
  return (
    <div className="ai-modal-status">
      <div className="ai-status-icon" aria-hidden="true">⏹</div>
      <p className="ai-status-title">
        Génération arrêtée — {createdCount} question{createdCount > 1 ? 's' : ''} conservée{createdCount > 1 ? 's' : ''}.
      </p>
    </div>
  )
}

// #137 — échec **d'un job déjà démarré** : contrairement à #8, on ne perd
// plus tout. Règle absolue de la maquette §5 : ne jamais dire "échec" sans
// dire combien de questions ont été conservées.
function FailedBody({ createdCount, maxConsecutiveFailures, errorCode, errorMessage, onConfigure }) {
  const n = maxConsecutiveFailures || 2
  return (
    <div className="ai-modal-status">
      <div className="ai-status-icon" aria-hidden="true">⚠️</div>
      <p className="ai-status-title">
        Génération interrompue après {n} échec{n > 1 ? 's' : ''} consécutif{n > 1 ? 's' : ''} —
        {' '}{createdCount} question{createdCount > 1 ? 's' : ''} conservée{createdCount > 1 ? 's' : ''}.
      </p>
      <p className="ai-status-sub">{jobErrorMessage(errorCode)}</p>
      {/* issue #142 — détail assaini du message d'erreur provider réel, en
          complément du message générique dérivé de ERROR_CODE ci-dessus.
          Absent (chaîne vide) sur un serveur antérieur à #142. */}
      {errorMessage && (
        <details className="ai-error-detail">
          <summary>Détail technique</summary>
          <pre>{errorMessage}</pre>
        </details>
      )}
      {errorCode === 'no_api_key' && (
        <Button variant="secondary" size="sm" onClick={onConfigure}>Configurer une clé API</Button>
      )}
    </div>
  )
}

// Rejet synchrone à la soumission — aucun job n'a démarré, rien à conserver.
function SubmitErrorBody({ error, onConfigure }) {
  return (
    <div className="ai-modal-status">
      <div className="ai-status-icon" aria-hidden="true">⚠️</div>
      <p className="ai-status-title">{error?.message}</p>
      {error?.detail && (
        <details className="ai-error-detail">
          <summary>Détail technique</summary>
          <pre>{error.detail}</pre>
        </details>
      )}
      {error?.showConfigLink && (
        <Button variant="secondary" size="sm" onClick={onConfigure}>
          Configurer une clé API
        </Button>
      )}
    </div>
  )
}

function UnavailableBody({ onConfigure }) {
  return (
    <div className="ai-modal-status">
      <div className="ai-status-icon" aria-hidden="true">🔌</div>
      <p className="ai-status-title">
        Génération indisponible. Aucune clé API configurée pour le fournisseur sélectionné, ou pas
        d'accès réseau externe depuis ce serveur.
      </p>
      <Button variant="primary" onClick={onConfigure}>
        Configurer une clé API
      </Button>
    </div>
  )
}

/**
 * @param {string} title - texte du `<h2>` d'en-tête (avec emoji éventuel)
 * @param {string} [ariaLabel] - `aria-label` du `role="dialog"` (par défaut : `title`)
 * @param {'QUIZ'|'RAFALE'} target - cible de CETTE modale (contrat §6) — un `aiJob` d'une
 *   autre cible est intégralement ignoré (adoption, transitions, décompte)
 * @param {string} endpoint - URL POST de soumission
 * @param {function} buildPayload - () => object, appelé au clic sur "Générer"
 * @param {boolean} canSubmit
 * @param {string} [submitDisabledTitle] - `title` du bouton "Générer" quand désactivé
 * @param {function} renderForm - () => JSX, rendu uniquement en viewState==='form'
 * @param {boolean} apiKeyConfigured
 * @param {string} [provider]
 * @param {Object|null} aiJob - useGame().aiJob (singleton global)
 * @param {function} onCancelGeneration - (jobId) => void
 * @param {function} onClose - fermeture ordinaire (×, Échap, Annuler, Fermer pendant EnCours/rejet)
 * @param {function} [onCloseTerminal] - fermeture depuis un panneau terminal (Terminé/Arrêté/Échec) ;
 *   par défaut `onClose`. Permet à l'appelant d'exécuter une action propre à sa modale avant de
 *   fermer (ex. AIGenerateModal y calcule `onGenerated` — Rafale n'en a pas besoin, GATE 2 §6ter).
 * @param {function} [onSubmitted] - appelé de façon SYNCHRONE au moment où "Générer" est cliqué,
 *   AVANT l'appel réseau (ex. AIGenerateModal y prend un instantané de `questions` pour calculer
 *   son delta — voir son commentaire sur `startingQuestionIdsRef`)
 * @param {function} [onJobAdopted] - appelé quand un job RUNNING externe (démarré par un autre
 *   admin/onglet, `target` correspondant) est adopté alors qu'aucun job n'était suivi
 * @param {function} onNavigateToSettings - navigue vers /admin/settings puis ferme
 * @param {number} [interBatchDelayMs=60000]
 * @param {number} [maxConsecutiveFailures=2]
 * @param {Array} [breakdown=[]] - détail par catégorie de l'écran "Terminé" (Quiz uniquement —
 *   Rafale passe toujours `[]`, décision GATE 2 §6ter "pas d'état ajouté")
 */
export default function AIJobModalShell({
  title,
  ariaLabel,
  target,
  endpoint,
  buildPayload,
  canSubmit,
  submitDisabledTitle,
  renderForm,
  apiKeyConfigured,
  provider = 'anthropic',
  aiJob = null,
  onCancelGeneration,
  onClose,
  onCloseTerminal,
  onSubmitted,
  onJobAdopted,
  onNavigateToSettings,
  interBatchDelayMs = 60000,
  maxConsecutiveFailures = 2,
  breakdown = [],
}) {
  // 'unavailable' | 'form' | 'loading' | 'success' | 'cancelled' | 'failed' | 'submit-error'
  const [viewState, setViewState] = useState(() => initialViewStateFor(aiJob, apiKeyConfigured, target))
  // Job suivi par CETTE modale — distingue "notre" job d'un job antérieur
  // dont `aiJob` porterait encore la trace.
  const [trackedJobId, setTrackedJobId] = useState(() => (matchesTarget(aiJob, target) ? aiJob.jobId : null))
  const [cancelRequested, setCancelRequested] = useState(false)
  const [submitError, setSubmitError] = useState(null)

  const handleClose = useCallback(() => {
    onClose()
  }, [onClose])

  const handleCloseTerminal = useCallback(() => {
    ;(onCloseTerminal || onClose)()
  }, [onCloseTerminal, onClose])

  // #137 — fermeture désormais TOUJOURS autorisée, y compris pendant EnCours
  // (maquette §3 : "plus de blocage, puisque rien n'est perdu").
  useEffect(() => {
    const handleKeyDown = (e) => {
      if (e.key === 'Escape') handleClose()
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [handleClose])

  // Réagit aux mises à jour de `aiJob` — IGNORE tout job dont la cible ne
  // correspond pas à `target` (contrat §6, R6 du plan) :
  // - un job RUNNING qu'on ne suit pas encore (trackedJobId === null) est
  //   ADOPTÉ — couvre le cas où un job démarre pendant que la modale est
  //   déjà ouverte sur le formulaire (un autre admin l'a lancé, ou un second
  //   onglet), pas seulement le ré-attachement au montage.
  // - les mises à jour du job déjà suivi font avancer/terminer la vue.
  // - tout `aiJob` d'un AUTRE job (jobId différent de celui suivi) est ignoré.
  useEffect(() => {
    if (!matchesTarget(aiJob, target)) return
    if (aiJob.state === 'RUNNING') {
      if (trackedJobId === null) {
        setTrackedJobId(aiJob.jobId)
        setViewState('loading')
        onJobAdopted?.()
      } else if (aiJob.jobId === trackedJobId && viewState !== 'loading') {
        setViewState('loading')
      }
      return
    }
    if (!trackedJobId || aiJob.jobId !== trackedJobId || viewState !== 'loading') return
    if (aiJob.state === 'DONE') setViewState('success')
    else if (aiJob.state === 'CANCELLED') setViewState('cancelled')
    else if (aiJob.state === 'FAILED') setViewState('failed')
  }, [aiJob, target, trackedJobId, viewState, onJobAdopted])

  // Décompte "Prochain lot dans Ns…" (maquette §3) : estimé côté client à
  // partir du dernier BATCHES_DONE reçu + ai.inter_batch_delay_ms — le
  // serveur ne pousse pas de tick intermédiaire, seulement un message par lot.
  const [countdown, setCountdown] = useState(null)
  const lastProgressRef = useRef(null) // { batchesDone, at }

  useEffect(() => {
    if (viewState !== 'loading' || aiJob?.state !== 'RUNNING') {
      setCountdown(null)
      lastProgressRef.current = null
      return
    }
    const now = Date.now()
    if (!lastProgressRef.current || lastProgressRef.current.batchesDone !== aiJob.batchesDone) {
      lastProgressRef.current = { batchesDone: aiJob.batchesDone, at: now }
    }
    const tick = () => {
      const elapsed = Date.now() - lastProgressRef.current.at
      const remaining = Math.max(0, Math.ceil((interBatchDelayMs - elapsed) / 1000))
      setCountdown(remaining > 0 ? remaining : null)
    }
    tick()
    const id = setInterval(tick, 1000)
    return () => clearInterval(id)
  }, [viewState, aiJob?.state, aiJob?.batchesDone, interBatchDelayMs])

  const handleGenerate = async () => {
    if (!canSubmit) return
    setSubmitError(null)
    setCancelRequested(false)
    // Hook synchrone AVANT l'appel réseau (ex. snapshot Quiz) — voir jsdoc onSubmitted.
    onSubmitted?.()
    setViewState('loading')
    const payload = buildPayload()
    try {
      const res = await fetch(endpoint, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      })
      let data = null
      try { data = await res.json() } catch { /* réponse non-JSON (ex. 405 texte brut) */ }

      if (res.status === 202 && data?.job_id) {
        // Job accepté — la suite est pilotée par AI_GENERATION_PROGRESS
        // (effet ci-dessus), pas par cette réponse HTTP.
        setTrackedJobId(data.job_id)
      } else {
        setSubmitError(mapSubmitError({ networkFailure: false, data }))
        setViewState('submit-error')
      }
    } catch (err) {
      setSubmitError(mapSubmitError({ networkFailure: true, data: { message: err.message } }))
      setViewState('submit-error')
    }
  }

  const handleCancelJob = () => {
    if (!trackedJobId || cancelRequested) return
    setCancelRequested(true)
    onCancelGeneration?.(trackedJobId)
  }

  const handleBackToForm = () => {
    setSubmitError(null)
    setTrackedJobId(null)
    setViewState('form') // Les valeurs saisies sont conservées (même state React côté formulaire)
  }

  const handleConfigure = useCallback(() => {
    onNavigateToSettings?.()
  }, [onNavigateToSettings])

  const isCompact = viewState === 'unavailable'
  const createdCount = aiJob?.createdCount ?? 0
  const skippedCount = aiJob?.skippedCount ?? 0

  return (
    <div className="ai-modal-overlay" onClick={handleClose}>
      <div
        className={`ai-modal ${isCompact ? 'ai-modal--compact' : ''}`}
        onClick={(e) => e.stopPropagation()}
        role="dialog"
        aria-modal="true"
        aria-label={ariaLabel || title}
      >
        <div className="ai-modal-header">
          <h2>{title}</h2>
          <button className="ai-modal-close" onClick={handleClose} aria-label="Fermer">×</button>
        </div>

        <div className="ai-modal-body">
          {viewState === 'unavailable' && <UnavailableBody onConfigure={handleConfigure} />}
          {viewState === 'loading' && (
            <RunningBody aiJob={aiJob} countdown={countdown} provider={aiJob?.provider || provider} />
          )}
          {viewState === 'success' && (
            <DoneBody createdCount={createdCount} skippedCount={skippedCount} breakdown={breakdown} />
          )}
          {viewState === 'cancelled' && <CancelledBody createdCount={createdCount} />}
          {viewState === 'failed' && (
            <FailedBody
              createdCount={createdCount}
              maxConsecutiveFailures={maxConsecutiveFailures}
              errorCode={aiJob?.errorCode}
              errorMessage={aiJob?.errorMessage}
              onConfigure={handleConfigure}
            />
          )}
          {viewState === 'submit-error' && (
            <SubmitErrorBody error={submitError} onConfigure={handleConfigure} />
          )}

          {viewState === 'form' && renderForm()}
        </div>

        {viewState === 'form' && (
          <div className="ai-modal-footer">
            <Button variant="secondary" onClick={handleClose}>Annuler</Button>
            <Button
              variant="primary"
              onClick={handleGenerate}
              disabled={!canSubmit}
              title={submitDisabledTitle}
            >
              ✨ Générer
            </Button>
          </div>
        )}
        {viewState === 'loading' && (
          <div className="ai-modal-footer">
            <Button variant="secondary" onClick={handleCancelJob} disabled={cancelRequested} loading={cancelRequested}>
              Arrêter
            </Button>
            <Button variant="primary" onClick={handleClose}>Fermer</Button>
          </div>
        )}
        {(viewState === 'success' || viewState === 'cancelled') && (
          <div className="ai-modal-footer">
            {/* Bug UX (retour utilisateur) — un job terminé (DONE/CANCELLED) restait
                affiché indéfiniment : `aiJob` n'est jamais réinitialisé après un job
                (seul un nouveau RUNNING l'écrase), donc rouvrir la modale après avoir
                fermé ce panneau réaffichait le MÊME résultat obsolète, sans issue vers
                le formulaire. */}
            <Button variant="secondary" onClick={handleCloseTerminal}>Fermer</Button>
            <Button variant="primary" onClick={handleBackToForm}>Nouvelle génération</Button>
          </div>
        )}
        {(viewState === 'failed' || viewState === 'submit-error') && (
          <div className="ai-modal-footer">
            <Button variant="secondary" onClick={viewState === 'failed' ? handleCloseTerminal : handleClose}>Fermer</Button>
            <Button variant="primary" onClick={handleBackToForm}>Réessayer</Button>
          </div>
        )}
      </div>
    </div>
  )
}
