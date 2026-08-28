import { useState, useMemo, useEffect, useCallback, useRef } from 'react'
import { useNavigate } from 'react-router-dom'
import Button from './Button'
import { GENERABLE_TYPES } from '../utils/questionTypeMeta'
import './AIGenerateModal.css'

// Répartition par type — libellés/couleurs/défauts normatifs (maquette §3).
// Le libellé de SPEEDY reste "Speedy" (nomenclature existante de
// QuestionsPage), pas "Normal" comme dans l'artefact visuel — divergence
// tranchée n°4 de la maquette.
// ARDOISE (T2.3, plan planner-20260806-121743-qualif-137.md §2) — 5e type
// générable, désactivé à 0% par défaut (arbitrage CDP Q2.2 : comme MEMOTION,
// pas comme les 3 premiers) pour ne pas redistribuer silencieusement les
// pourcentages par défaut existants.
// #183/A-F2 — table icône/libellé/couleur fusionnée dans
// `utils/questionTypeMeta.js` (source unique, ne plus dupliquer ici).
// #196 (v7.1.0) — TYPES consomme désormais GENERABLE_TYPES (5 types réels +
// le pseudo-type MEMOTION_PLUS, contrat ai-generation.md §3ter), PAS
// QUESTION_TYPES : c'est la seule table qui doit connaître MEMOTION_PLUS
// (contracts/ai-generation.md §3ter — jamais QUESTION_TYPES lui-même).
// MEMOTION_PLUS désactivé à 0% par défaut, même traitement que MEMOTION —
// ne redistribue pas silencieusement les pourcentages par défaut existants.
const TYPES = GENERABLE_TYPES
const DEFAULT_DISTRIBUTION = { SPEEDY: 40, QCM: 40, MEMORY: 20, MEMOTION: 0, MEMOTION_PLUS: 0, ARDOISE: 0 }
const DEFAULT_TYPE_ENABLED = { SPEEDY: true, QCM: true, MEMORY: true, MEMOTION: false, MEMOTION_PLUS: false, ARDOISE: false }

// #137 — messages ERROR_CODE (contract ai-multi-provider.md §10, réutilise
// ai-generation.md §3 + provider_quota). Utilisés pour l'état ARRÊTÉ/ÉCHEC du
// job asynchrone (pas de upstream_status disponible dans ce payload : le
// backend a déjà résolu 401/429 vers un code stable unique).
const JOB_ERROR_MESSAGES = {
  no_api_key: 'Clé API invalide ou absente. Vérifiez-la dans Paramètres.',
  invalid_request: 'Requête invalide.',
  upstream_error: "Le serveur n'a pas pu joindre le provider IA.",
  timeout: 'Un lot a dépassé le temps imparti.',
  id_exhausted: "Plus d'identifiant disponible pour de nouvelles questions.",
  provider_quota: 'Quota quotidien du provider atteint. Réessayez demain.',
}

function jobErrorMessage(errorCode) {
  return JOB_ERROR_MESSAGES[errorCode] || 'Erreur pendant la génération.'
}

function providerLabel(provider) {
  if (provider === 'groq') return 'Groq (gratuit)'
  if (provider === 'anthropic') return 'Claude (Anthropic)'
  return provider || '—'
}

// État initial de la modale à partir du dernier `aiJob` connu (useWebSocket,
// alimenté par AI_GENERATION_PROGRESS) — pas seulement RUNNING. Couvre le cas
// où la modale est (ré)ouverte alors que le dernier job connu est déjà
// terminal (DONE/CANCELLED/FAILED) : le panneau correspondant s'affiche
// directement, plutôt qu'un formulaire vierge qui ferait perdre le résultat
// d'un job qui vient de se terminer.
function initialViewStateFor(aiJob, apiKeyConfigured) {
  switch (aiJob?.state) {
    case 'RUNNING': return 'loading'
    case 'DONE': return 'success'
    case 'CANCELLED': return 'cancelled'
    case 'FAILED': return 'failed'
    default: return apiKeyConfigured ? 'form' : 'unavailable'
  }
}

function activeTypeKeys(enabled) {
  return TYPES.map(t => t.key).filter(k => enabled[k])
}

// Algorithme de rebalance normatif — repris tel quel de la maquette §3.
// Déplacement d'un slider `changed` vers `newValue` : les autres types actifs
// se partagent le reste (100 - newValue), proportionnellement à leur valeur
// actuelle (ou également si tous étaient à 0). Le dernier absorbe l'arrondi.
function rebalanceOnSlide(distribution, enabled, changed, newValue) {
  const others = activeTypeKeys(enabled).filter(k => k !== changed)
  const next = { ...distribution, [changed]: newValue }
  if (others.length === 0) return next

  const remaining = 100 - newValue
  const othersSum = others.reduce((s, k) => s + distribution[k], 0)

  if (othersSum === 0) {
    const share = Math.floor(remaining / others.length)
    others.forEach((k, i) => {
      next[k] = i === others.length - 1 ? remaining - share * (others.length - 1) : share
    })
    return next
  }

  let acc = 0
  others.forEach((k, i) => {
    if (i === others.length - 1) {
      next[k] = remaining - acc
    } else {
      const v = Math.round(remaining * distribution[k] / othersSum)
      next[k] = v
      acc += v
    }
  })
  return next
}

// Basculement d'un toggle de type — OFF redistribue sa valeur au prorata des
// types encore actifs ; ON repart à 20% puis applique le rebalance ci-dessus.
function rebalanceOnToggle(distribution, enabled, type, turningOn) {
  if (turningOn) {
    const withOn = { ...enabled, [type]: true }
    return rebalanceOnSlide({ ...distribution, [type]: 20 }, withOn, type, 20)
  }

  const v = distribution[type]
  const stillActive = activeTypeKeys(enabled).filter(k => k !== type)
  const next = { ...distribution, [type]: 0 }
  if (stillActive.length === 0 || v === 0) return next

  if (v >= 100) {
    // Cas dégénéré non couvert littéralement par la maquette : le type
    // désactivé portait 100% (donc tous les autres actifs étaient à 0) — un
    // partage proportionnel diviserait par zéro. Repli sur un partage égal,
    // même logique que la branche othersSum===0 de rebalanceOnSlide.
    const share = Math.floor(100 / stillActive.length)
    stillActive.forEach((k, i) => {
      next[k] = i === stillActive.length - 1 ? 100 - share * (stillActive.length - 1) : share
    })
    return next
  }

  let acc = 0
  stillActive.forEach((k, i) => {
    if (i === stillActive.length - 1) {
      next[k] = 100 - acc
    } else {
      const add = Math.round(v * distribution[k] / (100 - v))
      const newVal = distribution[k] + add
      next[k] = newVal
      acc += newVal
    }
  })
  return next
}

function clampInt(raw, min, max, fallback) {
  const n = parseInt(raw, 10)
  if (Number.isNaN(n)) return fallback
  return Math.max(min, Math.min(max, n))
}

// Traduit le rejet **synchrone** de POST /api/generate-questions (avant même
// qu'un job existe : 400/405/409/507, contract ai-multi-provider.md §9) en
// message lisible. Distinct de jobErrorMessage ci-dessus, qui couvre l'échec
// **asynchrone** d'un job déjà démarré (ERROR_CODE de AI_GENERATION_PROGRESS).
function mapSubmitError({ networkFailure, data }) {
  if (networkFailure) {
    return { message: "Le serveur n'a pas pu être joint. Vérifiez l'accès réseau.", showConfigLink: false, detail: null }
  }
  const code = data?.code
  if (code === 'no_api_key') {
    return { message: 'Clé API invalide ou absente. Vérifiez-la dans Paramètres.', showConfigLink: true, detail: null }
  }
  if (code === 'generation_in_progress') {
    // Ne devrait pas arriver en pratique : le bouton ré-attache déjà à
    // EnCours si un job tourne. Filet de sécurité si l'état a changé entre
    // l'ouverture de la modale et la soumission (second onglet/admin).
    return { message: 'Une génération est déjà en cours. Fermez et rouvrez la fenêtre pour suivre sa progression.', showConfigLink: false, detail: null }
  }
  return { message: 'Erreur pendant la génération.', showConfigLink: false, detail: data?.message || null }
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

// #137 — nouvel état terminal, absent de #8 (maquette §5, §6).
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
      {/* issue #142 — détail assaini du message d'erreur provider réel (ex.
          "discriminator: multiple candidate properties" pour #142 lui-même),
          en complément du message générique dérivé de ERROR_CODE ci-dessus.
          Absent (chaîne vide) sur un serveur antérieur à #142 — repliable,
          motif ai-error-detail déjà en place sur SubmitErrorBody. */}
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
 * AIGenerateModal — Modale de génération de questions via IA (#8 v6.0.0,
 * refondue en tâche de fond par #137 v6.1.0, retour QUALIF v6.0.7).
 *
 * Reprend le motif ReclaimConfirmModal (TeamsPage.jsx) : overlay +
 * stopPropagation + bouton "×" aria-label="Fermer". Machine à états (maquette
 * 137-generation-tache-de-fond.md §6, remplace §7 de 8-generateur-ia.md) :
 * unavailable/form/loading(EnCours)/success(Terminé)/cancelled(Arrêté)/
 * failed(Échec)/submit-error. Différence structurante avec #8 : la
 * génération est un **job asynchrone global** (contract ai-multi-provider.md
 * §10-§12) suivi via `aiJob` (useWebSocket, alimenté par
 * AI_GENERATION_PROGRESS) — pas un état local à la modale. Fermer la modale
 * pendant EnCours est désormais autorisé : le job continue, et rouvrir le
 * bouton "✨ Générer via IA" ré-attache directement à la progression (y
 * compris après un rechargement de page, `aiJob` étant réémis à la connexion
 * si un job tourne).
 *
 * Retour QUALIF (#137) — le bloc "Paramètres du Quiz" (Thème/Population/
 * Difficulté/Langue) n'est plus un formulaire éditable : ces 4 valeurs sont
 * lues directement depuis les props à chaque rendu et affichées en lecture
 * seule (+ lien "modifier" vers la section Quiz de QuestionsPage) — retirer
 * une resaisie que l'utilisateur jugeait redondante avec les réglages
 * globaux déjà existants (#8).
 *
 * v6.1.0 (#137 Batch 2b) — publics/difficultés deviennent multi-valeurs
 * (quizPopulations/quizDifficulties, tableaux), ajout de quizObjectives
 * (jamais diffusé aux joueurs, cf. game-state.md) et de
 * hasUnsavedQuizChanges (bandeau d'avertissement, correction du bug de
 * fraîcheur — l'appelant DOIT sourcer ces props depuis gameState.quiz*,
 * jamais depuis l'état local d'un formulaire non enregistré).
 *
 * @param {function} onClose
 * @param {boolean} apiKeyConfigured - clé configurée pour le provider ACTUELLEMENT sélectionné
 *   (nom de prop inchangé depuis #8 — la modale ne fait que le refléter, l'activation du bouton
 *   d'entrée selon le provider sélectionné vit dans QuestionsPage)
 * @param {string} provider - 'anthropic' | 'groq', provider actuellement sélectionné (Paramètres) —
 *   utilisé seulement en repli tant qu'aucun aiJob.provider n'est encore connu
 * @param {Array} categories - liste GET /api/categories ({key, name, color, isCustom})
 * @param {string} quizTheme - valeur globale (GameState.QUIZ_THEME), affichée en lecture seule et envoyée telle quelle
 * @param {string[]} quizPopulations - GameState.QUIZ_POPULATIONS (v6.1.0 — remplace quizPopulation string)
 * @param {string[]} quizDifficulties - GameState.QUIZ_DIFFICULTIES (v6.1.0 — remplace quizDifficulty string)
 * @param {string} quizLanguage - GameState.QUIZ_LANGUAGE
 * @param {string} quizObjectives - GameState.QUIZ_OBJECTIVES (v6.1.0) - jamais affiché aux joueurs
 * @param {boolean} hasUnsavedQuizChanges - T2.5 : au moins un des 5 champs ci-dessus diverge de
 *   l'état local (non enregistré) du formulaire de la section Quiz — affiche le bandeau d'avertissement
 * @param {Object} questions - useGame().questions, pour dériver le delta "questions créées par ce job"
 * @param {Object|null} aiJob - useGame().aiJob : {jobId,state,batchesDone,batchesTotal,createdCount,skippedCount,errorCode,errorMessage,provider}
 *   errorMessage (issue #142) : détail assaini du message d'erreur provider réel, présent
 *   uniquement quand state==='FAILED' — affiché en complément du message générique dérivé
 *   d'errorCode, pas à sa place (errorCode reste la source stable pour le cas no_api_key)
 * @param {function} onCancelGeneration - (jobId) => void, émet CANCEL_AI_GENERATION
 * @param {number} interBatchDelayMs - ai.inter_batch_delay_ms (config), pour le décompte "Prochain lot dans Ns…"
 * @param {number} maxConsecutiveFailures - ai.max_consecutive_failures (config), pour le message d'échec
 * @param {function} onGenerated - appelé avec l'ID de la première question créée, à la fermeture d'un état terminal (scroll)
 * @param {function} onNavigateToQuizSettings - lien "modifier" du rappel Thème/Publics/Difficultés/Langue/Objectif
 */
export default function AIGenerateModal({
  onClose,
  apiKeyConfigured,
  provider = 'anthropic',
  categories = [],
  quizTheme = '',
  quizPopulations = [],
  quizDifficulties = [],
  quizLanguage = '',
  quizObjectives = '',
  hasUnsavedQuizChanges = false,
  questions = {},
  aiJob = null,
  onCancelGeneration,
  interBatchDelayMs = 60000,
  maxConsecutiveFailures = 2,
  onGenerated,
  onNavigateToQuizSettings,
}) {
  const navigate = useNavigate()

  // 'unavailable' | 'form' | 'loading' | 'success' | 'cancelled' | 'failed' | 'submit-error'
  const [viewState, setViewState] = useState(() => initialViewStateFor(aiJob, apiKeyConfigured))
  // Job suivi par CETTE modale — distingue "notre" job d'un job antérieur
  // dont `aiJob` porterait encore la trace. Initialisé dès qu'un aiJob existe
  // au montage (pas seulement RUNNING), pour que le retour à un état
  // terminal déjà connu (ex. FAILED -> "Réessayer" puis nouveau montage) reste cohérent.
  const [trackedJobId, setTrackedJobId] = useState(() => aiJob?.jobId ?? null)
  const [cancelRequested, setCancelRequested] = useState(false)

  // Retour QUALIF (#137) — le bloc "Paramètres du Quiz" (Thème/Population/
  // Difficulté/Langue), auparavant recopié dans un état local éditable, est
  // retiré : ces 4 valeurs sont désormais lues DIRECTEMENT depuis les props
  // à chaque rendu (jamais copiées dans un state local), ce qui règle du
  // même coup le bug de fraîcheur qu'aurait introduit un simple retrait des
  // champs — un état local initialisé par useState(quizTheme) ne se serait
  // pas mis à jour si le global changeait entre deux ouvertures de la
  // modale. Un rappel en lecture seule (+ lien "modifier") remplace le
  // formulaire retiré, cf. rendu ci-dessous.

  // Bloc 2 — Cette génération
  const [instructions, setInstructions] = useState('')
  const [selectedCategories, setSelectedCategories] = useState(() => new Set())
  const [volumeMode, setVolumeMode] = useState('count')
  const [volumeCount, setVolumeCount] = useState(20)
  const [volumeDuration, setVolumeDuration] = useState(45)
  const [distribution, setDistribution] = useState(DEFAULT_DISTRIBUTION)
  const [typeEnabled, setTypeEnabled] = useState(DEFAULT_TYPE_ENABLED)

  const [submitError, setSubmitError] = useState(null)

  // Snapshot des IDs de questions déjà présentes au moment où l'on commence à
  // suivre un job (soumission fraîche, ou ré-attachement au montage). La
  // progression AI_GENERATION_PROGRESS ne porte que des compteurs cumulatifs
  // (contract §10) — aucune liste de questions créées. Le détail par
  // catégorie affiché en fin de job (maquette §4) est donc reconstitué
  // côté client par différence avec `questions` (useGame(), alimenté au fil
  // de l'eau par broadcastQuestions() après chaque lot). Sur un
  // ré-attachement après rechargement de page, les lots traités AVANT la
  // reconnexion ne sont pas dans ce delta — seul le total CREATED_COUNT du
  // job (fourni par le serveur) reste exact dans ce cas ; c'est une
  // approximation assumée, pas une donnée manquante côté contrat.
  const startingQuestionIdsRef = useRef(null)
  if (startingQuestionIdsRef.current === null && aiJob) {
    startingQuestionIdsRef.current = new Set(Object.keys(questions || {}))
  }

  const handleConfigure = useCallback(() => {
    // AIGenerateModal only ever renders on /admin/* (QuestionsPage) — /anim is
    // its own page (AnimPage) without question generation, so the prefix is a
    // constant now, not derived from the URL (#155/F2, was an alias before).
    navigate('/admin/settings')
    onClose()
  }, [navigate, onClose])

  // #137 — fermeture désormais TOUJOURS autorisée, y compris pendant EnCours
  // (maquette §3 : "plus de blocage, puisque rien n'est perdu") — le job
  // continue en tâche de fond, contrairement à #8 où la fermeture était
  // bloquée pendant l'appel HTTP synchrone.
  const handleClose = useCallback(() => {
    onClose()
  }, [onClose])

  useEffect(() => {
    const handleKeyDown = (e) => {
      if (e.key === 'Escape') handleClose()
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [handleClose])

  // Réagit aux mises à jour de `aiJob` :
  // - un job RUNNING qu'on ne suit pas encore (trackedJobId === null) est
  //   ADOPTÉ — couvre le cas où un job démarre pendant que la modale est
  //   déjà ouverte sur le formulaire (un autre admin l'a lancé, ou un second
  //   onglet), pas seulement le ré-attachement au montage.
  // - les mises à jour du job déjà suivi font avancer/terminer la vue.
  // - tout `aiJob` d'un AUTRE job (jobId différent de celui suivi) est ignoré
  //   — trace d'un job antérieur dont l'état global n'a pas encore été
  //   écrasé par le prochain RUNNING.
  useEffect(() => {
    if (!aiJob) return
    if (aiJob.state === 'RUNNING') {
      if (trackedJobId === null) {
        setTrackedJobId(aiJob.jobId)
        setViewState('loading')
        if (startingQuestionIdsRef.current === null) {
          startingQuestionIdsRef.current = new Set(Object.keys(questions || {}))
        }
      } else if (aiJob.jobId === trackedJobId && viewState !== 'loading') {
        setViewState('loading')
      }
      return
    }
    if (!trackedJobId || aiJob.jobId !== trackedJobId || viewState !== 'loading') return
    if (aiJob.state === 'DONE') setViewState('success')
    else if (aiJob.state === 'CANCELLED') setViewState('cancelled')
    else if (aiJob.state === 'FAILED') setViewState('failed')
  }, [aiJob, trackedJobId, viewState, questions])

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

  const toggleCategory = (key) => {
    setSelectedCategories(prev => {
      const next = new Set(prev)
      if (next.has(key)) next.delete(key); else next.add(key)
      return next
    })
  }

  const handleToggleType = (key) => {
    const turningOn = !typeEnabled[key]
    setDistribution(prev => rebalanceOnToggle(prev, typeEnabled, key, turningOn))
    setTypeEnabled(prev => ({ ...prev, [key]: turningOn }))
  }

  const handleSlide = (key, rawValue) => {
    const value = clampInt(rawValue, 0, 100, distribution[key])
    setDistribution(prev => rebalanceOnSlide(prev, typeEnabled, key, value))
  }

  const hasActiveType = TYPES.some(t => typeEnabled[t.key])
  const volumeValid = volumeMode === 'count' ? volumeCount > 0 : volumeDuration > 0
  // Thème/Publics/Difficultés/Langue/Objectif viennent directement des props
  // (plus d'état local, cf. commentaire plus haut) — publics et difficultés
  // sont désormais multi-valeurs (v6.1.0), au moins un élément requis dans
  // chacun (maquette §2, règle 5).
  const canSubmit =
    (quizTheme || '').trim() !== '' &&
    quizPopulations.length > 0 &&
    quizDifficulties.length > 0 &&
    selectedCategories.size > 0 &&
    hasActiveType &&
    volumeValid

  // Tooltip explicatif sur le bouton "Générer" (bugfix/config-api-key-help,
  // tâche #8) — liste précisément la/les condition(s) manquante(s) plutôt
  // qu'un message générique. Contrairement au bouton "✨ Générer via IA" de
  // QuestionsPage (désactivé pour une seule raison possible : pas de clé
  // pour le provider sélectionné, déjà pourvu d'un title), ce bouton dépend
  // de 6 conditions indépendantes.
  const submitMissingReasons = canSubmit ? [] : [
    (quizTheme || '').trim() === '' && 'le thème (section Quiz)',
    quizPopulations.length === 0 && 'au moins un public (section Quiz)',
    quizDifficulties.length === 0 && 'au moins une difficulté (section Quiz)',
    selectedCategories.size === 0 && 'au moins une catégorie cible',
    !hasActiveType && 'au moins un type de question activé',
    !volumeValid && (volumeMode === 'count' ? 'un nombre de questions valide' : 'une durée de partie valide'),
  ].filter(Boolean)
  const submitDisabledTitle = submitMissingReasons.length > 0
    ? `Champ(s) requis manquant(s) : ${submitMissingReasons.join(', ')}`
    : undefined

  const handleGenerate = async () => {
    if (!canSubmit) return
    setSubmitError(null)
    setCancelRequested(false)
    // Snapshot AVANT l'appel : delta correct même si le premier
    // AI_GENERATION_PROGRESS (et le premier broadcastQuestions) arrive très vite.
    startingQuestionIdsRef.current = new Set(Object.keys(questions || {}))
    setViewState('loading')
    const payload = {
      theme: (quizTheme || '').trim(),
      populations: quizPopulations,
      language: quizLanguage,
      difficulties: quizDifficulties,
      objectives: (quizObjectives || '').trim(),
      instructions: instructions.trim(),
      categories: Array.from(selectedCategories),
      volume: volumeMode === 'count'
        ? { mode: 'count', value: volumeCount }
        : { mode: 'duration', value: volumeDuration },
      distribution,
    }
    try {
      const res = await fetch('/api/generate-questions', {
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

  // Nom de catégorie affiché → clé technique (pour le détail par catégorie
  // de l'état TERMINÉ, maquette §4).
  const nameByKey = useMemo(() => {
    const m = {}
    categories.forEach(c => { m[c.key] = c.name })
    return m
  }, [categories])

  // Delta "questions créées par CE job" — voir le commentaire sur
  // startingQuestionIdsRef ci-dessus pour les limites de cette approche.
  const newQuestions = useMemo(() => {
    if (!startingQuestionIdsRef.current) return []
    return Object.values(questions || {}).filter(q => q?.ID && !startingQuestionIdsRef.current.has(q.ID))
  }, [questions])

  const breakdown = useMemo(() => {
    const map = new Map()
    newQuestions.forEach(q => {
      const label = nameByKey[q.CATEGORY] || q.CATEGORY
      map.set(label, (map.get(label) || 0) + 1)
    })
    return Array.from(map.entries())
  }, [newQuestions, nameByKey])

  const firstNewQuestionId = useMemo(() => {
    if (newQuestions.length === 0) return null
    return [...newQuestions].sort((a, b) => parseInt(a.ID, 10) - parseInt(b.ID, 10))[0].ID
  }, [newQuestions])

  const handleCloseTerminal = () => {
    onGenerated?.(firstNewQuestionId)
    onClose()
  }

  const handleBackToForm = () => {
    setSubmitError(null)
    setTrackedJobId(null)
    setViewState('form') // Les valeurs saisies sont conservées (même state React)
  }

  const isCompact = viewState === 'unavailable'
  const createdCount = aiJob?.createdCount ?? newQuestions.length
  const skippedCount = aiJob?.skippedCount ?? 0

  return (
    <div className="ai-modal-overlay" onClick={handleClose}>
      <div
        className={`ai-modal ${isCompact ? 'ai-modal--compact' : ''}`}
        onClick={(e) => e.stopPropagation()}
        role="dialog"
        aria-modal="true"
        aria-label="Générer des questions via IA"
      >
        <div className="ai-modal-header">
          <h2>✨ Générer des questions via IA</h2>
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

          {viewState === 'form' && (
            <>
              {/* Retour QUALIF (#137) — remplace le bloc éditable "Paramètres
                  du Quiz" : l'utilisateur demandait à ne plus RESAISIR ces
                  5 valeurs (déjà réglées dans la section Quiz de
                  QuestionsPage), pas à ne plus savoir lesquelles s'appliquent.
                  Rappel en lecture seule + lien vers la section globale.
                  T2.5 — le bandeau rend visible un écart entre le formulaire
                  (non enregistré) et ce qui est réellement diffusé/utilisé. */}
              <div className="ai-quiz-summary">
                <div className="ai-quiz-summary-head">
                  <span className="ai-quiz-summary-title">Réglages de la partie</span>
                  <button
                    type="button"
                    className="ai-inline-link ai-quiz-summary-link"
                    onClick={onNavigateToQuizSettings}
                  >
                    modifier
                  </button>
                </div>
                {hasUnsavedQuizChanges && (
                  <div className="ai-quiz-summary-banner" role="alert">
                    <span className="ai-quiz-summary-banner-icon" aria-hidden="true">⚠️</span>
                    <span>
                      <strong>Des modifications de la section Quiz ne sont pas enregistrées.</strong>{' '}
                      La génération utilisera les valeurs ci-dessous.
                    </span>
                  </div>
                )}
                <dl className="ai-quiz-summary-list">
                  <dt>Thème</dt>
                  <dd>{quizTheme || '—'}</dd>
                  <dt>Publics</dt>
                  <dd>
                    {quizPopulations.length > 0 ? (
                      quizPopulations.map(p => <span key={p} className="ai-mini-chip">{p}</span>)
                    ) : (
                      <span className="ai-quiz-summary-missing">Aucun public sélectionné — renseignez la section Quiz</span>
                    )}
                  </dd>
                  <dt>Difficultés</dt>
                  <dd>
                    {quizDifficulties.length > 0 ? (
                      quizDifficulties.map(d => <span key={d} className="ai-mini-chip">{d}</span>)
                    ) : (
                      <span className="ai-quiz-summary-missing">Aucune difficulté sélectionnée — renseignez la section Quiz</span>
                    )}
                  </dd>
                  <dt>Langue</dt>
                  <dd>{quizLanguage || '—'}</dd>
                  <dt>Objectif</dt>
                  <dd>{quizObjectives || '—'}</dd>
                </dl>
              </div>

              {/* Bloc 2 — Cette génération */}
              <div className="ai-modal-block">
                <div className="ai-block-header">
                  <h3 className="ai-block-title">Cette génération</h3>
                </div>

                <label className="ai-field ai-field--full">
                  <span>Précisions pour cette génération <em>(optionnel)</em></span>
                  <textarea
                    value={instructions}
                    onChange={(e) => setInstructions(e.target.value)}
                    placeholder="ex. insister sur les comédies, éviter le sport, ton humoristique..."
                    rows={2}
                    maxLength={2000}
                  />
                </label>

                <div className="ai-field ai-field--full">
                  <span>Catégories cibles</span>
                  <div className="ai-chip-row">
                    {categories.map(c => (
                      <button
                        type="button"
                        key={c.key}
                        className={`ai-chip ai-chip--category ${selectedCategories.has(c.key) ? 'active' : ''}`}
                        style={{ '--chip-color': c.color || '#6b7280' }}
                        onClick={() => toggleCategory(c.key)}
                      >
                        {selectedCategories.has(c.key) && <span className="ai-chip-check" aria-hidden="true">✓</span>}
                        {c.name}
                      </button>
                    ))}
                    {categories.length === 0 && (
                      <span className="ai-chip-row-empty">Aucune catégorie disponible.</span>
                    )}
                  </div>
                </div>

                <div className="ai-field ai-field--full">
                  <span>Volume</span>
                  <div className="ai-volume-toggle">
                    <button
                      type="button"
                      className={volumeMode === 'count' ? 'active' : ''}
                      onClick={() => setVolumeMode('count')}
                    >
                      Nombre de questions
                    </button>
                    <button
                      type="button"
                      className={volumeMode === 'duration' ? 'active' : ''}
                      onClick={() => setVolumeMode('duration')}
                    >
                      Durée de partie
                    </button>
                  </div>
                  {volumeMode === 'count' ? (
                    <div className="ai-volume-input">
                      <input
                        type="number"
                        min={1}
                        max={200}
                        value={volumeCount}
                        onChange={(e) => setVolumeCount(clampInt(e.target.value, 1, 200, volumeCount))}
                      />
                      <span>questions — le temps de réponse de chacune est déterminé par l'IA</span>
                    </div>
                  ) : (
                    <div className="ai-volume-input">
                      <input
                        type="number"
                        min={5}
                        max={240}
                        value={volumeDuration}
                        onChange={(e) => setVolumeDuration(clampInt(e.target.value, 5, 240, volumeDuration))}
                      />
                      <span>minutes — le nombre de questions et le temps de réponse de chacune sont déterminés par l'IA</span>
                    </div>
                  )}
                </div>

                <div className="ai-field ai-field--full">
                  <div className="ai-distribution-header">
                    <span>Répartition par type</span>
                    <span className="ai-distribution-hint">Total 100%</span>
                  </div>
                  <div className="ai-distribution-rows">
                    {TYPES.map(t => (
                      <div key={t.key} className={`ai-distribution-row ${!typeEnabled[t.key] ? 'disabled' : ''}`}>
                        <label className="ai-toggle-switch">
                          <input
                            type="checkbox"
                            checked={typeEnabled[t.key]}
                            onChange={() => handleToggleType(t.key)}
                            aria-label={`Activer ${t.label}`}
                          />
                          <span className="ai-toggle-slider" style={{ '--type-color': t.color }} />
                        </label>
                        <span className="ai-distribution-label">{t.label}</span>
                        <input
                          type="range"
                          min={0}
                          max={100}
                          value={distribution[t.key]}
                          disabled={!typeEnabled[t.key]}
                          onChange={(e) => handleSlide(t.key, e.target.value)}
                          style={{ '--type-color': t.color, '--pct': `${distribution[t.key]}%` }}
                        />
                        <span className="ai-distribution-pct">{distribution[t.key]}%</span>
                      </div>
                    ))}
                  </div>
                </div>
              </div>
            </>
          )}
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
                le formulaire — reproduit et confirmé (test ligne ~198 "shows a single
                Fermer button" verrouillait ce comportement). Complète la décision CDP
                déjà actée ("réaffichage du dernier résultat avec action claire pour
                relancer") : le réaffichage existait, l'action de relance manquait. */}
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
