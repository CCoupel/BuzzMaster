import { useState, useMemo, useCallback } from 'react'
import { useNavigate } from 'react-router-dom'
import AIJobModalShell from './ai/AIJobModalShell'
import { QUIZ_POPULATIONS, QUIZ_LANGUAGES } from './QuizMetaForm'
import './RafaleAIGenerateModal.css'

// #203 (v8.1.0, tâche 11) — Modale de génération IA du réservoir RAFALE
// (contrat contracts/rafale-ai-generation.md, maquette
// docs/mockups/rafale-ai-generation-203.html §04/§06).
//
// Ne porte QUE le formulaire Rafale, son payload et sa règle `canSubmit` —
// tout le reste (enveloppe, machine à états, cycle de vie du job, 6 corps
// d'état, filtrage TARGET) vient de la coquille partagée `AIJobModalShell`
// (tâche 10, extraite de AIGenerateModal.jsx). `target="RAFALE"` — la
// coquille ignore tout job Quiz en cours (contrat §6, R6).
//
// Décisions GATE 2 reconduites ici (voir contrat §2bis/§3.1/§6ter) :
// - `count` par PALIERS FERMÉS uniquement (`RAFALE_GENERATION_PRESETS`,
//   constante partagée Go ↔ JS — voir `rafaleGenerationPresets` côté
//   `ai_generator_rafale.go`), filtrés par `maxQuestions` (ai.max_questions).
// - Catégories CHOISIES LIBREMENT parmi toutes les catégories connues (prop
//   `categories`, pas restreintes à l'existant du réservoir), mais annotées
//   du nombre déjà présent pour les difficultés sélectionnées — comptes
//   dérivés côté client de `questions` (déjà chargée par RafalePage),
//   AUCUN appel réseau supplémentaire.
// - AUCUN état "fraîchement ajouté" : `breakdown={[]}` sur l'écran "terminé",
//   pas d'instantané avant lancement, pas de callback `onGenerated`.
const RAFALE_GENERATION_PRESETS = [10, 20, 50, 100, 200]
const RAFALE_DIFFICULTIES = [1, 2, 3]

function stars(d) {
  return '★'.repeat(d) + '☆'.repeat(3 - d)
}

/**
 * @param {function} onClose
 * @param {boolean} apiKeyConfigured
 * @param {string} [provider]
 * @param {Object|null} aiJob - useGame().aiJob
 * @param {function} onCancelGeneration - (jobId) => void
 * @param {number} [interBatchDelayMs]
 * @param {number} [maxConsecutiveFailures]
 * @param {Array} categories - GET /api/categories, TOUTES les catégories connues (dur + custom),
 *   contrat §3.1 — pas restreint à ce qui existe déjà dans le réservoir
 * @param {Array} questions - réservoir déjà chargé par RafalePage ({ID,QUESTION,ANSWER,CATEGORY,
 *   DIFFICULTY,USED}[]) — source des comptes existant/après, aucun fetch propre à la modale
 * @param {number} [maxQuestions=200] - ai.max_questions (config), filtre les paliers visibles
 */
export default function RafaleAIGenerateModal({
  onClose,
  apiKeyConfigured,
  provider = 'anthropic',
  aiJob = null,
  onCancelGeneration,
  interBatchDelayMs = 60000,
  maxConsecutiveFailures = 2,
  categories = [],
  questions = [],
  maxQuestions = 200,
}) {
  const navigate = useNavigate()

  const [theme, setTheme] = useState('')
  const [language, setLanguage] = useState('Français')
  const [selectedPopulations, setSelectedPopulations] = useState(() => new Set())
  const [instructions, setInstructions] = useState('')
  const [selectedCategories, setSelectedCategories] = useState(() => new Set())
  const [selectedDifficulties, setSelectedDifficulties] = useState(() => new Set())
  const [count, setCount] = useState(20)

  const handleNavigateToSettings = useCallback(() => {
    navigate('/admin/settings')
    onClose()
  }, [navigate, onClose])

  const togglePopulation = (p) => {
    setSelectedPopulations(prev => {
      const next = new Set(prev)
      if (next.has(p)) next.delete(p); else next.add(p)
      return next
    })
  }

  const toggleCategory = (key) => {
    setSelectedCategories(prev => {
      const next = new Set(prev)
      if (next.has(key)) next.delete(key); else next.add(key)
      return next
    })
  }

  const toggleDifficulty = (d) => {
    setSelectedDifficulties(prev => {
      const next = new Set(prev)
      if (next.has(d)) next.delete(d); else next.add(d)
      return next
    })
  }

  // Ordre stable (celui du catalogue), pas l'ordre de clic — envoyé tel
  // quel au serveur, et utilisé comme "l'ordre de la requête" (contrat §2bis)
  // pour la répartition de la matrice existant→après ci-dessous.
  const orderedSelectedPopulations = useMemo(
    () => QUIZ_POPULATIONS.filter(p => selectedPopulations.has(p)),
    [selectedPopulations]
  )
  const orderedSelectedCategories = useMemo(
    () => categories.filter(c => selectedCategories.has(c.key)).map(c => c.key),
    [categories, selectedCategories]
  )
  const orderedSelectedDifficulties = useMemo(
    () => RAFALE_DIFFICULTIES.filter(d => selectedDifficulties.has(d)),
    [selectedDifficulties]
  )

  const nameByKey = useMemo(() => {
    const m = {}
    categories.forEach(c => { m[c.key] = c.name })
    return m
  }, [categories])

  // Nombre déjà présent au réservoir par catégorie, pour les difficultés
  // SÉLECTIONNÉES (aucune sélection = toutes difficultés confondues, repli
  // raisonnable avant tout choix) — contrat §3.1, "carte des trous".
  const existingCountByCategory = useMemo(() => {
    const counts = {}
    const filterDiff = selectedDifficulties.size > 0 ? selectedDifficulties : null
    ;(questions || []).forEach(q => {
      if (filterDiff && !filterDiff.has(q.DIFFICULTY)) return
      counts[q.CATEGORY] = (counts[q.CATEGORY] || 0) + 1
    })
    return counts
  }, [questions, selectedDifficulties])

  // Compte exact par couple (catégorie, difficulté) — pour la matrice
  // existant → après uniquement (contrairement à existingCountByCategory
  // ci-dessus, qui agrège les difficultés sélectionnées pour l'annotation
  // des chips).
  const existingCoupleCount = useMemo(() => {
    const map = new Map()
    ;(questions || []).forEach(q => {
      const key = `${q.CATEGORY}|${q.DIFFICULTY}`
      map.set(key, (map.get(key) || 0) + 1)
    })
    return map
  }, [questions])

  const visiblePresets = useMemo(
    () => RAFALE_GENERATION_PRESETS.filter(p => p <= maxQuestions),
    [maxQuestions]
  )
  // Le palier sélectionné peut devenir invisible si `maxQuestions` diminue
  // après coup (config rechargée) — repli sur le plus grand palier encore
  // visible plutôt que de soumettre une valeur que le serveur refuserait.
  const effectiveCount = visiblePresets.includes(count)
    ? count
    : (visiblePresets[visiblePresets.length - 1] ?? 0)

  // Répartition uniforme sur les couples catégorie × difficulté SÉLECTIONNÉS,
  // reste attribué aux premiers couples "dans l'ordre de la requête" (contrat
  // §2bis) — ordre catégorie (externe) puis difficulté (interne), identique
  // à celui envoyé dans buildPayload ci-dessous.
  const shareByCouple = useMemo(() => {
    const map = new Map()
    const total = orderedSelectedCategories.length * orderedSelectedDifficulties.length
    if (total === 0) return map
    const perCouple = Math.floor(effectiveCount / total)
    const remainder = effectiveCount % total
    let idx = 0
    orderedSelectedCategories.forEach(cat => {
      orderedSelectedDifficulties.forEach(diff => {
        map.set(`${cat}|${diff}`, perCouple + (idx < remainder ? 1 : 0))
        idx += 1
      })
    })
    return map
  }, [orderedSelectedCategories, orderedSelectedDifficulties, effectiveCount])

  const canSubmit =
    theme.trim() !== '' &&
    orderedSelectedPopulations.length > 0 &&
    orderedSelectedCategories.length > 0 &&
    orderedSelectedDifficulties.length > 0 &&
    effectiveCount > 0

  const submitMissingReasons = canSubmit ? [] : [
    theme.trim() === '' && 'le thème',
    orderedSelectedPopulations.length === 0 && 'au moins un public',
    orderedSelectedCategories.length === 0 && 'au moins une catégorie cible',
    orderedSelectedDifficulties.length === 0 && 'au moins une difficulté',
    // Garde défensive non atteignable en pratique via l'UI (visiblePresets
    // n'est vide que si ai.max_questions est mal configuré, < 10) — même
    // discipline que AIGenerateModal.jsx (cf. son commentaire sur
    // volumeValid, AIGenerateModal.tooltip.test.jsx).
    effectiveCount === 0 && 'un palier de volume disponible',
  ].filter(Boolean)
  const submitDisabledTitle = submitMissingReasons.length > 0
    ? `Champ(s) requis manquant(s) : ${submitMissingReasons.join(', ')}`
    : undefined

  const buildPayload = useCallback(() => ({
    theme: theme.trim(),
    populations: orderedSelectedPopulations,
    language,
    instructions: instructions.trim(),
    categories: orderedSelectedCategories,
    difficulties: orderedSelectedDifficulties,
    count: effectiveCount,
  }), [theme, orderedSelectedPopulations, language, instructions, orderedSelectedCategories, orderedSelectedDifficulties, effectiveCount])

  const renderForm = () => (
    <>
      <div className="ai-modal-block">
        <div className="ai-block-header">
          <h3 className="ai-block-title">Contenu à générer</h3>
        </div>

        <label className="ai-field ai-field--full">
          <span>Thème</span>
          <input
            type="text"
            value={theme}
            onChange={(e) => setTheme(e.target.value)}
            placeholder="ex. Culture générale — France"
            maxLength={200}
          />
          <span className="rafale-ai-hint">
            Propre à cette génération — le réservoir est global, il n'hérite pas du thème d'une
            partie en cours.
          </span>
        </label>

        <label className="ai-field">
          <span>Langue</span>
          <select value={language} onChange={(e) => setLanguage(e.target.value)}>
            {QUIZ_LANGUAGES.map(l => <option key={l} value={l}>{l}</option>)}
          </select>
        </label>

        <div className="ai-field ai-field--full">
          <span>Publics cibles</span>
          <div className="ai-chip-row">
            {QUIZ_POPULATIONS.map(p => (
              <button
                type="button"
                key={p}
                className={`ai-chip ${selectedPopulations.has(p) ? 'active' : ''}`}
                onClick={() => togglePopulation(p)}
              >
                {selectedPopulations.has(p) && <span className="ai-chip-check" aria-hidden="true">✓</span>}
                {p}
              </button>
            ))}
          </div>
        </div>

        <label className="ai-field ai-field--full">
          <span>Précisions pour cette génération <em>(optionnel)</em></span>
          <textarea
            value={instructions}
            onChange={(e) => setInstructions(e.target.value)}
            placeholder="ex. éviter l'actualité récente, privilégier les classiques…"
            rows={2}
            maxLength={2000}
          />
        </label>
      </div>

      <div className="ai-modal-block">
        <div className="ai-block-header">
          <h3 className="ai-block-title">Ce qu'on ajoute au réservoir</h3>
        </div>

        <div className="ai-field ai-field--full">
          <span>Taille du lot</span>
          <div className="ai-volume-toggle">
            {visiblePresets.map(p => (
              <button
                type="button"
                key={p}
                className={effectiveCount === p ? 'active' : ''}
                onClick={() => setCount(p)}
              >
                {p}
              </button>
            ))}
            {visiblePresets.length === 0 && (
              <span className="ai-chip-row-empty">Aucun palier disponible (configuration).</span>
            )}
          </div>
          <span className="rafale-ai-hint">
            Un réservoir se remplit par paliers, pas au questionnaire près — les paliers au-delà du
            plafond configuré ({maxQuestions}) sont masqués.
          </span>
        </div>

        <div className="ai-field ai-field--full">
          <span>
            Catégories cibles <em>— le nombre indique ce que contient déjà le réservoir pour les
            difficultés sélectionnées</em>
          </span>
          <div className="ai-chip-row">
            {categories.map(c => {
              const n = existingCountByCategory[c.key] || 0
              return (
                <button
                  type="button"
                  key={c.key}
                  className={`ai-chip ai-chip--category ${selectedCategories.has(c.key) ? 'active' : ''}`}
                  style={{ '--chip-color': c.color || '#6b7280' }}
                  onClick={() => toggleCategory(c.key)}
                >
                  {selectedCategories.has(c.key) && <span className="ai-chip-check" aria-hidden="true">✓</span>}
                  {c.name}
                  <span className={`rafale-ai-cat-count ${n === 0 ? 'zero' : ''}`}>{n}</span>
                </button>
              )
            })}
            {categories.length === 0 && (
              <span className="ai-chip-row-empty">Aucune catégorie disponible.</span>
            )}
          </div>
        </div>

        <div className="ai-field ai-field--full">
          <span>Difficultés</span>
          <div className="ai-chip-row">
            {RAFALE_DIFFICULTIES.map(d => (
              <button
                type="button"
                key={d}
                className={`ai-chip rafale-ai-star-chip ${selectedDifficulties.has(d) ? 'active' : ''}`}
                onClick={() => toggleDifficulty(d)}
              >
                {stars(d)}
              </button>
            ))}
          </div>
          <span className="rafale-ai-hint">
            Échelle native du réservoir (★ à ★★★) — pas de conversion depuis les 4 libellés de la
            génération Quiz, qui serait imprécise.
          </span>
        </div>
      </div>

      {orderedSelectedCategories.length > 0 && orderedSelectedDifficulties.length > 0 && (
        <div className="rafale-ai-matrix-block">
          <p className="rafale-ai-matrix-title">
            Répartition calculée — existant <span className="rafale-ai-matrix-arrow">→</span> après génération
          </p>
          <div className="rafale-ai-matrix-scroll">
            <table className="rafale-ai-matrix">
              <thead>
                <tr>
                  <th />
                  {orderedSelectedDifficulties.map(d => <th key={d}>{stars(d)}</th>)}
                </tr>
              </thead>
              <tbody>
                {orderedSelectedCategories.map(catKey => (
                  <tr key={catKey}>
                    <td>{nameByKey[catKey] || catKey}</td>
                    {orderedSelectedDifficulties.map(d => {
                      const existing = existingCoupleCount.get(`${catKey}|${d}`) || 0
                      const share = shareByCouple.get(`${catKey}|${d}`) || 0
                      return (
                        <td key={d} className="rafale-ai-matrix-cell">
                          {existing} <span className="rafale-ai-matrix-arrow">→</span> <strong>{existing + share}</strong>
                        </td>
                      )
                    })}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </>
  )

  return (
    <AIJobModalShell
      title="✨ Générer des questions RAFALE via IA"
      ariaLabel="Générer des questions RAFALE via IA"
      target="RAFALE"
      endpoint="/api/rafale/generate-questions"
      buildPayload={buildPayload}
      canSubmit={canSubmit}
      submitDisabledTitle={submitDisabledTitle}
      renderForm={renderForm}
      apiKeyConfigured={apiKeyConfigured}
      provider={provider}
      aiJob={aiJob}
      onCancelGeneration={onCancelGeneration}
      onClose={onClose}
      onNavigateToSettings={handleNavigateToSettings}
      interBatchDelayMs={interBatchDelayMs}
      maxConsecutiveFailures={maxConsecutiveFailures}
      breakdown={[]}
    />
  )
}
