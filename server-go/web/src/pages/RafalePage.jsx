import { useState, useEffect, useCallback, useMemo, useRef } from 'react'
import { motion } from 'framer-motion'
import { useOptionalGame } from '../hooks/GameContext'
import { useCategories } from '../hooks/useCategories'
import { useCategoryFilter } from '../hooks/useCategoryFilter'
import CategoryBadge from '../components/CategoryBadge'
import CategorySelector from '../components/CategorySelector'
import CategoryFilterBar from '../components/CategoryFilterBar'
import RafaleAIGenerateModal from '../components/RafaleAIGenerateModal'
import Button from '../components/Button'
import Card from '../components/Card'
import './RafalePage.css'

const DIFFICULTIES = [1, 2, 3]

const EMPTY_FORM = { ID: null, QUESTION: '', ANSWER: '', CATEGORY: '', DIFFICULTY: 1 }

// #203 (v8.1.0) — plafonds de longueur du réservoir, contrat
// rafale-ai-generation.md §5.1 (source de vérité : `rafaleMaxQuestionRunes`/
// `rafaleMaxAnswerRunes` côté serveur, internal/server/ai_generator_rafale.go).
// Mesurés en RUNES (points de code Unicode), pas en unités UTF-16 : `Array.from`
// itère par point de code (contrairement à `.length`), même discipline que
// `utf8.RuneCountInString` côté Go — un caractère accentué ou un emoji ne
// coûte jamais double.
const RAFALE_MAX_QUESTION_RUNES = 100
const RAFALE_MAX_ANSWER_RUNES = 40

function runeCount(str) {
  return Array.from((str || '').trim()).length
}

/**
 * RafalePage — éditeur du réservoir RAFALE (`/admin/rafale`, contrat
 * rafale.md §9, maquette docs/mockups/rafale-v8.html §7).
 *
 * Page dédiée, SÉPARÉE de l'éditeur Quiz/MEMOTION (QuestionsPage.jsx) —
 * le réservoir est un fichier unique côté serveur (`reservoir.json`,
 * §2.4), texte seul, aucun média (§3.1/D3). CRUD via les endpoints JSON
 * `GET/POST /api/rafale/questions`, `DELETE /api/rafale/questions/{id}`
 * (§9) — pas de multipart, contrairement à QuestionsPage.
 *
 * `USED` est dérivé côté serveur à la lecture (`rafale_used.json`, §3.2) —
 * jamais stocké/édité ici, uniquement affiché (colonne État + filtre
 * "Non utilisées").
 *
 * Filtres réutilisés du reste du projet : `useCategories`/`useCategoryFilter`
 * (mêmes hooks que QuestionsPage), `CategoryBadge` pour l'affichage —
 * aucune liste de catégories parallèle (§3.1).
 */
export default function RafalePage() {
  const { categories: apiCategories, refetch: refetchCategories } = useCategories()
  // Catégories CUSTOM uniquement (isCustom:true) — même filtrage que
  // QuestionsPage.jsx/GamePage.jsx (`customCategories`), JAMAIS apiCategories
  // brut (qui inclut aussi les 8 catégories codées en dur en miroir,
  // isCustom:false). Bugfix cohérence UI (v8.0.0, #16/#197, retour
  // utilisateur QUALIF 8.0.0.2) : RafalePage.jsx passait apiCategories brut
  // partout, une source de données différente de celle du Quiz — c'est ce
  // que ce filtrage aligne, en plus du composant d'affichage lui-même
  // (CategoryFilterBar ci-dessous).
  const customCategories = useMemo(() => apiCategories.filter(c => c.isCustom), [apiCategories])
  const [questions, setQuestions] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)

  const [selectedDifficulty, setSelectedDifficulty] = useState(null) // null = toutes
  const [onlyUnused, setOnlyUnused] = useState(false)

  const [form, setForm] = useState(EMPTY_FORM)
  const [formError, setFormError] = useState(null)
  const [submitting, setSubmitting] = useState(false)

  // Remise à disponible du flag « déjà utilisée » (contrat rafale.md §9,
  // dev-backend SHA 77e0d5ea, issue #197) — n'affecte JAMAIS le réservoir
  // lui-même (aucune question créée/modifiée/supprimée), seulement
  // `rafale_used.json` (§3.2). `resettingId`/`resettingAll` évitent un
  // double-clic pendant la requête, même discipline que `submitting`
  // ci-dessus pour le formulaire.
  const [resettingId, setResettingId] = useState(null)
  const [resettingAll, setResettingAll] = useState(false)

  // Génération IA du réservoir (#203, v8.1.0, tâche 12) — même motif que
  // QuestionsPage.jsx (aiConfig/providerConfigured/canOpenAiModal) : la clé
  // API n'est jamais transmise au frontend, seul son état "configurée ou
  // non" l'est via GET /config.json.
  // useOptionalGame (pas useGame) : RafalePage.rafale.test.jsx (27 tests,
  // antérieurs à #203) rend `<RafalePage />` sans GameProvider — useGame()
  // y lèverait. aiJob/cancelAiGeneration retombent alors sur leurs valeurs
  // par défaut ci-dessous, sans rien changer au comportement déjà testé.
  const game = useOptionalGame()
  const aiJob = game?.aiJob ?? null
  const cancelAiGeneration = game?.cancelAiGeneration
  const [showAIModal, setShowAIModal] = useState(false)
  const [aiConfig, setAiConfig] = useState({
    provider: 'anthropic',
    apiKeyConfigured: false,
    groqApiKeyConfigured: false,
    interBatchDelayMs: 60000,
    maxConsecutiveFailures: 2,
    maxQuestions: 200,
  })

  useEffect(() => {
    let cancelled = false
    const fetchAiStatus = async () => {
      try {
        const res = await fetch('/config.json')
        if (!res.ok) return
        const data = await res.json()
        if (!cancelled && data?.ai) {
          setAiConfig({
            provider: data.ai.provider || 'anthropic',
            apiKeyConfigured: !!data.ai.api_key_configured,
            groqApiKeyConfigured: !!data.ai.groq_api_key_configured,
            interBatchDelayMs: data.ai.inter_batch_delay_ms || 60000,
            maxConsecutiveFailures: data.ai.max_consecutive_failures || 2,
            maxQuestions: data.ai.max_questions || 200,
          })
        }
      } catch {
        // Génération indisponible — le bouton reste désactivé, la modale (si
        // ouverte malgré tout) retombera sur l'état "Indisponible".
      }
    }
    fetchAiStatus()
    return () => { cancelled = true }
  }, [])

  const providerConfigured = aiConfig.provider === 'groq' ? aiConfig.groqApiKeyConfigured : aiConfig.apiKeyConfigured
  // Reste cliquable si une génération RAFALE tourne déjà (ré-attachement) —
  // MÊME cible seulement : un job Quiz en cours ailleurs ne doit pas activer
  // ce bouton (contrat rafale-ai-generation.md §6, filtrage TARGET).
  const canOpenAiModal = providerConfigured || (aiJob?.state === 'RUNNING' && (aiJob?.target || 'QUIZ') === 'RAFALE')

  const loadQuestions = useCallback(() => {
    setLoading(true)
    setError(null)
    fetch('/api/rafale/questions')
      .then(res => {
        if (!res.ok) throw new Error(`HTTP ${res.status}`)
        return res.json()
      })
      .then(data => {
        setQuestions(data?.QUESTIONS ?? [])
        setLoading(false)
      })
      .catch(err => {
        setError(err.message)
        setLoading(false)
      })
  }, [])

  useEffect(() => {
    loadQuestions()
  }, [loadQuestions])

  // Refetch du réservoir piloté par la progression du job IA (#203, tâche 12,
  // contrat rafale-ai-generation.md §6 "Rafraîchissement de la liste du
  // réservoir") — AUCUN nouveau broadcast WebSocket : le réservoir n'a pas
  // d'équivalent de OnQuestionUpload, ce refetch réutilise `loadQuestions`
  // (déjà exercée par les 5 mutations existantes de cette page). Se déclenche
  // à chaque progression où CREATED_COUNT a augmenté ET TARGET === "RAFALE"
  // — jamais sur un job Quiz (filtrage identique à celui de la coquille de
  // modale, ai/AIJobModalShell.jsx, mais nécessaire ICI séparément : cette
  // page n'affiche pas forcément la modale au moment où le job avance).
  const lastRafaleProgressRef = useRef({ jobId: null, createdCount: 0 })
  useEffect(() => {
    if (!aiJob || (aiJob.target || 'QUIZ') !== 'RAFALE') return
    const tracked = lastRafaleProgressRef.current
    const createdCount = aiJob.createdCount || 0
    if (tracked.jobId !== aiJob.jobId) {
      lastRafaleProgressRef.current = { jobId: aiJob.jobId, createdCount: 0 }
    }
    if (createdCount > lastRafaleProgressRef.current.createdCount) {
      lastRafaleProgressRef.current = { jobId: aiJob.jobId, createdCount }
      loadQuestions()
    }
  }, [aiJob, loadQuestions])

  // useCategoryFilter attend q.CATEGORY (case commune Quiz/RAFALE, §3.1).
  const {
    selectedCategories,
    availableCategories,
    filteredQuestions: categoryFiltered,
    toggleCategoryFilter,
    clearCategoryFilters,
  } = useCategoryFilter(questions, customCategories)

  const filteredQuestions = useMemo(() => {
    return categoryFiltered.filter(q => {
      if (selectedDifficulty && q.DIFFICULTY !== selectedDifficulty) return false
      if (onlyUnused && q.USED) return false
      return true
    })
  }, [categoryFiltered, selectedDifficulty, onlyUnused])

  const usedCount = useMemo(() => questions.filter(q => q.USED).length, [questions])

  // #203 (tâche 13) — compteurs de caractères de l'éditeur manuel, mêmes
  // plafonds que la génération IA (contrat §5.1/§5.3).
  const questionRuneCount = useMemo(() => runeCount(form.QUESTION), [form.QUESTION])
  const answerRuneCount = useMemo(() => runeCount(form.ANSWER), [form.ANSWER])
  const questionOverLimit = questionRuneCount > RAFALE_MAX_QUESTION_RUNES
  const answerOverLimit = answerRuneCount > RAFALE_MAX_ANSWER_RUNES

  const resetForm = () => {
    setForm(EMPTY_FORM)
    setFormError(null)
  }

  const handleEdit = (q) => {
    setForm({ ID: q.ID, QUESTION: q.QUESTION, ANSWER: q.ANSWER, CATEGORY: q.CATEGORY, DIFFICULTY: q.DIFFICULTY })
    setFormError(null)
  }

  const handleDelete = async (id) => {
    if (!window.confirm('Supprimer cette question du reservoir RAFALE ?')) return
    try {
      const res = await fetch(`/api/rafale/questions/${encodeURIComponent(id)}`, { method: 'DELETE' })
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      if (form.ID === id) resetForm()
      loadQuestions()
    } catch (err) {
      setError(err.message)
    }
  }

  // Remet UNE question à disponible (contrat rafale.md §9). No-op silencieux
  // côté serveur si elle n'était pas marquée utilisée — pas de confirmation
  // ici (action peu risquée, à la différence du reset global ci-dessous) :
  // remettre par erreur une question à disponible n'a aucun effet destructif,
  // au pire elle sera simplement retirée du tirage un peu plus tôt/tard.
  const handleResetOne = async (id) => {
    setResettingId(id)
    try {
      const res = await fetch(`/api/rafale/questions/${encodeURIComponent(id)}/reset`, { method: 'POST' })
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      loadQuestions()
    } catch (err) {
      setError(err.message)
    } finally {
      setResettingId(null)
    }
  }

  // Remet TOUT le réservoir à disponible (contrat rafale.md §9). Aucun undo
  // côté UI une fois confirmé (le flag "déjà utilisée" est entièrement vidé
  // côté serveur) — confirmation obligatoire, même patron que les autres
  // actions globales du projet (BackupPage.jsx handleResetSelect,
  // ConfigPage.jsx "Remettre tous les scores a zero").
  const handleResetAll = async () => {
    if (!window.confirm(`Remettre les ${usedCount} question(s) utilisee(s) du reservoir RAFALE disponibles ? Le contenu des questions n'est pas modifie, seul le flag "deja utilisee" est efface.`)) return
    setResettingAll(true)
    try {
      const res = await fetch('/api/rafale/questions/reset-all', { method: 'POST' })
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      loadQuestions()
    } catch (err) {
      setError(err.message)
    } finally {
      setResettingAll(false)
    }
  }

  const handleSubmit = async (e) => {
    e.preventDefault()
    setFormError(null)

    if (!form.QUESTION.trim() || !form.ANSWER.trim()) {
      setFormError('Enonce et reponse sont obligatoires.')
      return
    }
    // #203 (tâche 13) — mêmes plafonds que la génération IA et que la
    // validation serveur de POST /api/rafale/questions (contrat
    // rafale-ai-generation.md §5.1/§5.3) : garde-fou client pour que le
    // compteur de caractères ci-dessous ne soit jamais contredit par un 400
    // surprise. Le bouton d'enregistrement est déjà désactivé au-delà de ces
    // plafonds (voir questionOverLimit/answerOverLimit) — ce contrôle reste
    // un filet, ex. soumission par la touche Entrée sur le champ Réponse.
    if (runeCount(form.QUESTION) > RAFALE_MAX_QUESTION_RUNES) {
      setFormError(`Enonce trop long (${RAFALE_MAX_QUESTION_RUNES} caracteres max).`)
      return
    }
    if (runeCount(form.ANSWER) > RAFALE_MAX_ANSWER_RUNES) {
      setFormError(`Reponse trop longue (${RAFALE_MAX_ANSWER_RUNES} caracteres max).`)
      return
    }
    if (!form.CATEGORY) {
      setFormError('Selectionnez une categorie.')
      return
    }

    setSubmitting(true)
    try {
      const body = {
        QUESTION: form.QUESTION.trim(),
        ANSWER: form.ANSWER.trim(),
        CATEGORY: form.CATEGORY,
        DIFFICULTY: form.DIFFICULTY,
      }
      if (form.ID) body.ID = form.ID

      const res = await fetch('/api/rafale/questions', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      })
      if (!res.ok) {
        const detail = await res.text().catch(() => '')
        throw new Error(detail || `HTTP ${res.status}`)
      }
      resetForm()
      loadQuestions()
    } catch (err) {
      setFormError(err.message)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="rafale-page page">
      <header className="page-header">
        <h1 className="page-title">Reservoir RAFALE</h1>
        <p className="page-subtitle">Questions texte seul, independantes du Quiz/MEMOTION</p>
      </header>

      <div className="rafale-layout">
        {/* Filtres + liste */}
        <Card padding="lg" className="rafale-list-card">
          {/* #203 (v8.1.0, tâche 12) — point d'entrée de la génération IA du
              réservoir, dans l'en-tête de la carte (maquette §03). Même
              motif que QuestionsPage.jsx : désactivé tant qu'aucune clé
              n'est configurée pour le provider sélectionné, tooltip
              explicite, hint additionnel sous l'en-tête. */}
          <div className="rafale-card-header">
            <h2 className="rafale-card-title">Réservoir RAFALE</h2>
            <Button
              variant="primary"
              size="sm"
              onClick={() => setShowAIModal(true)}
              disabled={!canOpenAiModal}
              title={canOpenAiModal ? 'Générer des questions via IA' : 'Configurer une clé API dans Paramètres pour activer la génération IA'}
            >
              ✨ Générer via IA
            </Button>
          </div>
          {!canOpenAiModal && (
            <p className="ai-generate-hint">
              <span className="ai-generate-hint-dot" aria-hidden="true">●</span>
              Configurer une clé API dans Paramètres pour activer la génération IA
            </p>
          )}

          <div className="rafale-filters">
            {/* CategoryFilterBar (v8.0.0, #16/#197, bugfix cohérence UI) —
                même composant/pattern que QuestionsPage.jsx (base
                GamePage.css + modificateur "Quiz" QuestionsPage.css),
                remplace l'ancienne variante .rafale-chip qui produisait un
                rendu incohérent (garde `if (!meta) return null` manquante +
                apiCategories non filtré). */}
            <CategoryFilterBar
              availableCategories={availableCategories}
              selectedCategories={selectedCategories}
              customCategories={customCategories}
              onToggle={toggleCategoryFilter}
              onClear={clearCategoryFilters}
            />
            <div className="rafale-filter-group">
              {DIFFICULTIES.map(d => (
                <button
                  key={d}
                  type="button"
                  className={`rafale-chip ${selectedDifficulty === d ? 'on' : ''}`}
                  onClick={() => setSelectedDifficulty(prev => (prev === d ? null : d))}
                  title={`Difficulte ${d}`}
                >
                  {'★'.repeat(d)}{'☆'.repeat(3 - d)}
                </button>
              ))}
              <button
                type="button"
                className={`rafale-chip ${onlyUnused ? 'on' : ''}`}
                onClick={() => setOnlyUnused(v => !v)}
              >
                Non utilisees
              </button>
            </div>
            {/* Reset global du flag "deja utilisee" (contrat rafale.md §9,
                issue #197) — visible seulement s'il y a quelque chose a
                remettre disponible. */}
            {usedCount > 0 && (
              <div className="rafale-filter-group">
                <button
                  type="button"
                  className="rafale-chip rafale-reset-all-btn"
                  onClick={handleResetAll}
                  disabled={resettingAll}
                  title="Remet toutes les questions utilisees du reservoir a l'etat disponible (le contenu des questions n'est pas modifie)"
                >
                  ↺ Remettre tout disponible ({usedCount})
                </button>
              </div>
            )}
          </div>

          {loading ? (
            <p className="rafale-status">Chargement...</p>
          ) : error ? (
            <p className="rafale-status rafale-status-error">Erreur : {error}</p>
          ) : filteredQuestions.length === 0 ? (
            <p className="rafale-status">Aucune question pour ce filtre.</p>
          ) : (
            <div className="rafale-table-wrap">
              <table className="rafale-table">
                <thead>
                  <tr>
                    <th>Question</th>
                    <th>Reponse</th>
                    <th>Categorie</th>
                    <th>Diff.</th>
                    <th>Etat</th>
                    <th />
                  </tr>
                </thead>
                <tbody>
                  {filteredQuestions.map(q => (
                    <tr key={q.ID} className={form.ID === q.ID ? 'editing' : ''}>
                      <td>{q.QUESTION}</td>
                      <td>{q.ANSWER}</td>
                      <td>
                        <CategoryBadge catKey={q.CATEGORY} customCategories={customCategories} size="sm" />
                      </td>
                      <td>{'★'.repeat(q.DIFFICULTY || 1)}</td>
                      <td className={q.USED ? 'rafale-used' : 'rafale-available'}>
                        {q.USED ? 'utilisee' : 'disponible'}
                      </td>
                      <td className="rafale-row-actions">
                        <button type="button" className="rafale-row-btn" onClick={() => handleEdit(q)} title="Modifier">✎</button>
                        {/* Reset individuel (contrat rafale.md §9, issue
                            #197) — visible SEULEMENT si la question est
                            marquee utilisee (sinon rien a remettre). */}
                        {q.USED && (
                          <button
                            type="button"
                            className="rafale-row-btn"
                            onClick={() => handleResetOne(q.ID)}
                            disabled={resettingId === q.ID}
                            title="Remettre disponible"
                          >
                            ↺
                          </button>
                        )}
                        <button type="button" className="rafale-row-btn rafale-row-btn-danger" onClick={() => handleDelete(q.ID)} title="Supprimer">🗑</button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}

          <p className="rafale-counts">
            {questions.length} question{questions.length > 1 ? 's' : ''} · {usedCount} utilisee{usedCount > 1 ? 's' : ''} · {questions.length - usedCount} disponible{questions.length - usedCount > 1 ? 's' : ''}
          </p>
        </Card>

        {/* Formulaire ajout / edition */}
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.3 }}
        >
          <Card padding="lg" className="rafale-form-card">
            <div className="section-header">
              <h2 className="section-title">{form.ID ? 'Modifier la question' : 'Nouvelle question'}</h2>
              <span className="section-icon">{form.ID ? '✎' : '➕'}</span>
            </div>

            <form onSubmit={handleSubmit} className="rafale-form">
              <div className="form-group">
                <label>Enonce</label>
                <textarea
                  value={form.QUESTION}
                  onChange={e => setForm(prev => ({ ...prev, QUESTION: e.target.value }))}
                  rows={3}
                  required
                />
                {/* #203 (tâche 13) — compteur de caractères (en runes, comme
                    la validation serveur) pour que le 400 de POST
                    /api/rafale/questions (plafond 100, contrat §5.3) ne soit
                    jamais atteint par surprise. */}
                <span className={`rafale-char-count ${questionOverLimit ? 'over' : ''}`}>
                  {questionRuneCount}/{RAFALE_MAX_QUESTION_RUNES}
                </span>
              </div>
              <div className="form-group">
                <label>Reponse</label>
                <input
                  type="text"
                  value={form.ANSWER}
                  onChange={e => setForm(prev => ({ ...prev, ANSWER: e.target.value }))}
                  required
                />
                <span className={`rafale-char-count ${answerOverLimit ? 'over' : ''}`}>
                  {answerRuneCount}/{RAFALE_MAX_ANSWER_RUNES}
                </span>
              </div>
              <div className="form-group">
                <label>Categorie</label>
                {/* CategorySelector (v8.0.0, #16/#197, bugfix cohérence UI) —
                    meme composant que l'editeur de question standard
                    (QuestionsPage.jsx), plus de <select> texte distinct. */}
                <CategorySelector
                  value={form.CATEGORY}
                  onChange={(key) => setForm(prev => ({ ...prev, CATEGORY: key }))}
                  customCategories={customCategories}
                  onRefetchCategories={refetchCategories}
                />
              </div>
              <div className="form-group">
                <label>Difficulte</label>
                <div className="rafale-diff-row">
                  {DIFFICULTIES.map(d => (
                    <button
                      key={d}
                      type="button"
                      className={`rafale-diff-btn ${form.DIFFICULTY === d ? 'active' : ''}`}
                      onClick={() => setForm(prev => ({ ...prev, DIFFICULTY: d }))}
                    >
                      {'★'.repeat(d)}
                    </button>
                  ))}
                </div>
              </div>

              {formError && <p className="rafale-status rafale-status-error">{formError}</p>}

              <div className="section-actions">
                <Button
                  type="submit"
                  variant="primary"
                  disabled={submitting || questionOverLimit || answerOverLimit}
                  loading={submitting}
                  title={questionOverLimit || answerOverLimit ? 'Raccourcissez le champ en dépassement avant d\'enregistrer' : undefined}
                >
                  {form.ID ? 'Enregistrer' : 'Ajouter'}
                </Button>
                {form.ID && (
                  <Button type="button" variant="secondary" onClick={resetForm} disabled={submitting}>
                    Annuler
                  </Button>
                )}
              </div>
            </form>
          </Card>
        </motion.div>
      </div>

      {showAIModal && (
        <RafaleAIGenerateModal
          onClose={() => setShowAIModal(false)}
          apiKeyConfigured={providerConfigured}
          provider={aiConfig.provider}
          aiJob={aiJob}
          onCancelGeneration={cancelAiGeneration}
          interBatchDelayMs={aiConfig.interBatchDelayMs}
          maxConsecutiveFailures={aiConfig.maxConsecutiveFailures}
          categories={apiCategories}
          questions={questions}
          maxQuestions={aiConfig.maxQuestions}
        />
      )}
    </div>
  )
}
