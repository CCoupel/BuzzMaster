import { useState, useMemo, useCallback, useRef } from 'react'
import { useNavigate } from 'react-router-dom'
import AIJobModalShell from './ai/AIJobModalShell'
import { clampInt } from './ai/aiJobHelpers'
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
// #203 (v8.1.0) — RAFALE reste délibérément hors de GENERABLE_TYPES : ce
// n'est pas un type de plus dans cette répartition, mais un chemin de
// génération dédié (RafaleAIGenerateModal.jsx), voir contrat
// rafale-ai-generation.md §1.1. Le filtre `t.key !== 'RAFALE'` de
// questionTypeMeta.js et son test restent inchangés.
const TYPES = GENERABLE_TYPES
const DEFAULT_DISTRIBUTION = { SPEEDY: 40, QCM: 40, MEMORY: 20, MEMOTION: 0, MEMOTION_PLUS: 0, ARDOISE: 0 }
const DEFAULT_TYPE_ENABLED = { SPEEDY: true, QCM: true, MEMORY: true, MEMOTION: false, MEMOTION_PLUS: false, ARDOISE: false }

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

/**
 * AIGenerateModal — Modale de génération de questions Quiz via IA (#8 v6.0.0,
 * refondue en tâche de fond par #137 v6.1.0, retour QUALIF v6.0.7).
 *
 * #203 (v8.1.0, tâche 10, GATE 2 §6bis) — allégée : l'enveloppe, la machine à
 * états, le cycle de vie du job, les 6 corps d'état, le pied par état et le
 * filtrage sur `TARGET` ont été EXTRAITS vers `ai/AIJobModalShell.jsx`,
 * partagée avec `RafaleAIGenerateModal.jsx`. Ce composant ne porte plus que
 * le formulaire Quiz et la construction de son payload.
 * 🔴 Critère bloquant (R10) — props publiques inchangées depuis #137, et les
 * 4 fichiers de test existants passent sans modification.
 *
 * Retour QUALIF (#137) — le bloc "Paramètres du Quiz" (Thème/Population/
 * Difficulté/Langue) n'est plus un formulaire éditable : ces 4 valeurs sont
 * lues directement depuis les props à chaque rendu et affichées en lecture
 * seule (+ lien "modifier" vers la section Quiz de QuestionsPage).
 *
 * v6.1.0 (#137 Batch 2b) — publics/difficultés multi-valeurs
 * (quizPopulations/quizDifficulties, tableaux), ajout de quizObjectives
 * (jamais diffusé aux joueurs, cf. game-state.md) et de
 * hasUnsavedQuizChanges (bandeau d'avertissement — l'appelant DOIT sourcer
 * ces props depuis gameState.quiz*, jamais depuis l'état local d'un
 * formulaire non enregistré).
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

  // Bloc 2 — Cette génération
  const [instructions, setInstructions] = useState('')
  const [selectedCategories, setSelectedCategories] = useState(() => new Set())
  const [volumeMode, setVolumeMode] = useState('count')
  const [volumeCount, setVolumeCount] = useState(20)
  const [volumeDuration, setVolumeDuration] = useState(45)
  const [distribution, setDistribution] = useState(DEFAULT_DISTRIBUTION)
  const [typeEnabled, setTypeEnabled] = useState(DEFAULT_TYPE_ENABLED)

  // Snapshot des IDs de questions déjà présentes au moment où l'on commence à
  // suivre un job (soumission fraîche, ou ré-attachement au montage/adoption
  // via onJobAdopted). La progression AI_GENERATION_PROGRESS ne porte que des
  // compteurs cumulatifs (contract §10) — aucune liste de questions créées.
  // Le détail par catégorie affiché en fin de job (maquette §4) est donc
  // reconstitué côté client par différence avec `questions` (useGame(),
  // alimenté au fil de l'eau par broadcastQuestions() après chaque lot). Sur
  // un ré-attachement après rechargement de page, les lots traités AVANT la
  // reconnexion ne sont pas dans ce delta — seul le total CREATED_COUNT du
  // job (fourni par le serveur) reste exact dans ce cas ; c'est une
  // approximation assumée, pas une donnée manquante côté contrat.
  const startingQuestionIdsRef = useRef(null)
  if (startingQuestionIdsRef.current === null && aiJob) {
    startingQuestionIdsRef.current = new Set(Object.keys(questions || {}))
  }

  const handleNavigateToSettings = useCallback(() => {
    // AIGenerateModal only ever renders on /admin/* (QuestionsPage) — /anim is
    // its own page (AnimPage) without question generation, so the prefix is a
    // constant now, not derived from the URL (#155/F2, was an alias before).
    navigate('/admin/settings')
    onClose()
  }, [navigate, onClose])

  const handleJobAdopted = useCallback(() => {
    if (startingQuestionIdsRef.current === null) {
      startingQuestionIdsRef.current = new Set(Object.keys(questions || {}))
    }
  }, [questions])

  const handleSubmitted = useCallback(() => {
    // Snapshot AVANT l'appel réseau : delta correct même si le premier
    // AI_GENERATION_PROGRESS (et le premier broadcastQuestions) arrive très vite.
    startingQuestionIdsRef.current = new Set(Object.keys(questions || {}))
  }, [questions])

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
  // (plus d'état local) — publics et difficultés sont multi-valeurs, au
  // moins un élément requis dans chacun (maquette §2, règle 5).
  const canSubmit =
    (quizTheme || '').trim() !== '' &&
    quizPopulations.length > 0 &&
    quizDifficulties.length > 0 &&
    selectedCategories.size > 0 &&
    hasActiveType &&
    volumeValid

  // Tooltip explicatif sur le bouton "Générer" (bugfix/config-api-key-help,
  // tâche #8) — liste précisément la/les condition(s) manquante(s) plutôt
  // qu'un message générique.
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

  const buildPayload = useCallback(() => ({
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
  }), [quizTheme, quizPopulations, quizLanguage, quizDifficulties, quizObjectives, instructions, selectedCategories, volumeMode, volumeCount, volumeDuration, distribution])

  // Delta "questions créées par CE job" — voir le commentaire sur
  // startingQuestionIdsRef ci-dessus pour les limites de cette approche.
  const newQuestions = useMemo(() => {
    if (!startingQuestionIdsRef.current) return []
    return Object.values(questions || {}).filter(q => q?.ID && !startingQuestionIdsRef.current.has(q.ID))
  }, [questions])

  // Nom de catégorie affiché → clé technique (pour le détail par catégorie
  // de l'état TERMINÉ, maquette §4).
  const nameByKey = useMemo(() => {
    const m = {}
    categories.forEach(c => { m[c.key] = c.name })
    return m
  }, [categories])

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

  const handleCloseTerminal = useCallback(() => {
    onGenerated?.(firstNewQuestionId)
    onClose()
  }, [onGenerated, firstNewQuestionId, onClose])

  const renderForm = () => (
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
  )

  return (
    <AIJobModalShell
      title="✨ Générer des questions via IA"
      ariaLabel="Générer des questions via IA"
      target="QUIZ"
      endpoint="/api/generate-questions"
      buildPayload={buildPayload}
      canSubmit={canSubmit}
      submitDisabledTitle={submitDisabledTitle}
      renderForm={renderForm}
      apiKeyConfigured={apiKeyConfigured}
      provider={provider}
      aiJob={aiJob}
      onCancelGeneration={onCancelGeneration}
      onClose={onClose}
      onCloseTerminal={handleCloseTerminal}
      onSubmitted={handleSubmitted}
      onJobAdopted={handleJobAdopted}
      onNavigateToSettings={handleNavigateToSettings}
      interBatchDelayMs={interBatchDelayMs}
      maxConsecutiveFailures={maxConsecutiveFailures}
      breakdown={breakdown}
    />
  )
}
