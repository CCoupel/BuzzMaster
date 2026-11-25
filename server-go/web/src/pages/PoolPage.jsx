import { useState, useEffect, useCallback, useMemo, useRef } from 'react'
import { motion } from 'framer-motion'
import { useOptionalGame } from '../hooks/GameContext'
import { useCategories } from '../hooks/useCategories'
import { useCategoryFilter } from '../hooks/useCategoryFilter'
import CategoryBadge from '../components/CategoryBadge'
import CategoryPicker from '../components/CategoryPicker'
import DifficultyPicker from '../components/DifficultyPicker'
import PoolAIGenerateModal from '../components/PoolAIGenerateModal'
import { getPoolInstance, poolUrls } from '../utils/poolInstances'
import Button from '../components/Button'
import Card from '../components/Card'
import './RafalePage.css'

// Refus du collage / de l'ajout (contrat picto-pool.md §4) — codes serveur → libellé.
const REFUSAL_LABELS = {
  WORD_INVALID: 'mot non valide',
  DUPLICATE: 'déjà dans la catégorie',
  CATEGORY_UNKNOWN: 'catégorie inconnue',
  DIFFICULTY_INVALID: 'difficulté invalide',
  POOL_FULL: 'pool plein',
}
// Types de question dont une manche en cours verrouille le Pool Picto (écritures refusées, contrat §4).
const PICTO_ROUND_TYPES = ['PICTO', 'RAFALE_PICTO']

const EMPTY_FORM = { ID: null, QUESTION: '', ANSWER: '', CATEGORY: '', DIFFICULTY: 1 }

// #203 (v8.1.0) — plafonds de longueur du réservoir, contrat
// rafale-ai-generation.md §5.1 (source de vérité : `rafaleMaxQuestionRunes`/
// `rafaleMaxAnswerRunes` côté serveur, internal/server/ai_generator_rafale.go).
// Mesurés en RUNES (points de code Unicode), pas en unités UTF-16 : `Array.from`
// itère par point de code (contrairement à `.length`), même discipline que
// `utf8.RuneCountInString` côté Go — un caractère accentué ou un emoji ne
// coûte jamais double.
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
export default function PoolPage({ instance = 'rafale' }) {
  const inst = getPoolInstance(instance)
  const L = inst.labels
  const urls = useMemo(() => poolUrls(inst), [inst])
  const { categories: apiCategories } = useCategories()
  // Catégories CUSTOM uniquement (isCustom:true) — même filtrage que
  // QuestionsPage.jsx/GamePage.jsx (`customCategories`), JAMAIS apiCategories
  // brut (qui inclut aussi les 8 catégories codées en dur en miroir,
  // isCustom:false). Bugfix cohérence UI (v8.0.0, #16/#197, retour
  // utilisateur QUALIF 8.0.0.2) : RafalePage.jsx passait apiCategories brut
  // partout, une source de données différente de celle du Quiz — c'est ce
  // que ce filtrage aligne, en plus du composant d'affichage lui-même
  // (CategoryPicker ci-dessous).
  // #285 : liste API brute (surcharges nom/couleur des intégrées incluses) ; les composants filtrent eux-mêmes.
  const customCategories = apiCategories
  const [questions, setQuestions] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)

  const [selectedDifficulty, setSelectedDifficulty] = useState(null) // null = toutes
  const [onlyUnused, setOnlyUnused] = useState(false)

  const [form, setForm] = useState(EMPTY_FORM)
  const [formError, setFormError] = useState(null)
  const [submitting, setSubmitting] = useState(false)

  // Collage d'une liste (Pool Picto seulement, M1 A) : « Un mot » / « Une liste ».
  const [entryMode, setEntryMode] = useState('one') // 'one' | 'list'
  const [listText, setListText] = useState('')
  const [listCategory, setListCategory] = useState('')
  const [listDifficulty, setListDifficulty] = useState(1)
  const [listReport, setListReport] = useState(null) // { created: [{ID,QUESTION}], refused: [{QUESTION,CODE}] }
  const listLines = useMemo(() => listText.split('\n').map(l => l.trim()).filter(Boolean), [listText])

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
  // Manche Picto en cours (STARTED / PAUSED) : le serveur refuse écritures, remises à zéro et génération (409
  // picto_round_in_progress) — l'interface les grise et l'explique (contrat picto-pool.md §4, maquette M1).
  const gs = game?.gameState
  const roundLocked = inst.key === 'picto' && PICTO_ROUND_TYPES.includes(gs?.question?.TYPE)
    && ['STARTED', 'PAUSED'].includes(gs?.phase)
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
  const canOpenAiModal = providerConfigured || (aiJob?.state === 'RUNNING' && (aiJob?.target || 'QUIZ') === inst.target)

  const loadQuestions = useCallback(() => {
    setLoading(true)
    setError(null)
    fetch(urls.list)
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
  }, [urls])

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
    if (!aiJob || (aiJob.target || 'QUIZ') !== inst.target) return
    const tracked = lastRafaleProgressRef.current
    const createdCount = aiJob.createdCount || 0
    if (tracked.jobId !== aiJob.jobId) {
      lastRafaleProgressRef.current = { jobId: aiJob.jobId, createdCount: 0 }
    }
    if (createdCount > lastRafaleProgressRef.current.createdCount) {
      lastRafaleProgressRef.current = { jobId: aiJob.jobId, createdCount }
      loadQuestions()
    }
  }, [aiJob, loadQuestions, inst.target])

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
  const questionOverLimit = questionRuneCount > inst.maxEntryRunes
  const answerOverLimit = answerRuneCount > inst.maxAnswerRunes

  const resetForm = () => {
    setForm(EMPTY_FORM)
    setFormError(null)
  }

  const handleEdit = (q) => {
    setForm({ ID: q.ID, QUESTION: q.QUESTION, ANSWER: q.ANSWER, CATEGORY: q.CATEGORY, DIFFICULTY: q.DIFFICULTY })
    setFormError(null)
  }

  // Ajout d'une liste collée : POST /words/batch, compte rendu des refus (jamais tronqués).
  const handleSubmitList = async (e) => {
    e.preventDefault()
    setFormError(null)
    setListReport(null)
    if (listLines.length === 0) { setFormError(L.required); return }
    if (!listCategory) { setFormError('Selectionnez une categorie.'); return }
    if (listLines.length > 200) { setFormError('200 mots maximum par liste.'); return }
    setSubmitting(true)
    try {
      const res = await fetch(urls.batch, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ CATEGORY: listCategory, DIFFICULTY: listDifficulty, WORDS: listLines }),
      })
      if (!res.ok) {
        const detail = await res.text().catch(() => '')
        throw new Error(detail || `HTTP ${res.status}`)
      }
      const data = await res.json()
      setListReport({ created: data?.CREATED ?? [], refused: data?.REFUSED ?? [] })
      // Les refusés restent dans la zone pour correction ; les créés en sortent.
      const refusedSet = new Set((data?.REFUSED ?? []).map(r => r.QUESTION))
      setListText(listLines.filter(l => refusedSet.has(l)).join('\n'))
      loadQuestions()
    } catch (err) {
      setFormError(err.message)
    } finally {
      setSubmitting(false)
    }
  }

  const handleDelete = async (id) => {
    if (!window.confirm(L.confirmDelete)) return
    try {
      const res = await fetch(urls.item(id), { method: 'DELETE' })
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
      const res = await fetch(urls.reset(id), { method: 'POST' })
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
    if (!window.confirm(L.confirmResetAll(usedCount))) return
    setResettingAll(true)
    try {
      const res = await fetch(urls.resetAll, { method: 'POST' })
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

    if (!form.QUESTION.trim() || (inst.hasAnswer && !form.ANSWER.trim())) {
      setFormError(L.required)
      return
    }
    // #203 (tâche 13) — mêmes plafonds que la génération IA et que la
    // validation serveur de POST /api/rafale/questions (contrat
    // rafale-ai-generation.md §5.1/§5.3) : garde-fou client pour que le
    // compteur de caractères ci-dessous ne soit jamais contredit par un 400
    // surprise. Le bouton d'enregistrement est déjà désactivé au-delà de ces
    // plafonds (voir questionOverLimit/answerOverLimit) — ce contrôle reste
    // un filet, ex. soumission par la touche Entrée sur le champ Réponse.
    if (runeCount(form.QUESTION) > inst.maxEntryRunes) {
      setFormError(L.entryTooLong(inst.maxEntryRunes))
      return
    }
    if (inst.hasAnswer && runeCount(form.ANSWER) > inst.maxAnswerRunes) {
      setFormError(L.answerTooLong(inst.maxAnswerRunes))
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
        ANSWER: inst.hasAnswer ? form.ANSWER.trim() : '',
        CATEGORY: form.CATEGORY,
        DIFFICULTY: form.DIFFICULTY,
      }
      if (form.ID) body.ID = form.ID

      const res = await fetch(urls.list, {
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
        <h1 className="page-title">{L.pageTitle}</h1>
        <p className="page-subtitle">{L.pageSubtitle}</p>
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
            <h2 className="rafale-card-title">{L.cardTitle}</h2>
            <Button
              variant="primary"
              size="sm"
              onClick={() => setShowAIModal(true)}
              disabled={!canOpenAiModal || roundLocked}
              title={canOpenAiModal ? L.aiButtonTitle : 'Configurer une clé API dans Paramètres pour activer la génération IA'}
            >
              ✨ Générer via IA
            </Button>
          </div>
          {roundLocked && (
            <p className="rafale-status rafale-status-error" role="status">
              ⏸ Manche Picto en cours : le pool ne peut pas être modifié ni complété par l'IA avant la fin de la manche.
            </p>
          )}
          {!canOpenAiModal && (
            <p className="ai-generate-hint">
              <span className="ai-generate-hint-dot" aria-hidden="true">●</span>
              Configurer une clé API dans Paramètres pour activer la génération IA
            </p>
          )}

          <div className="rafale-filters">
            {/* Filtre catégories (#221) — CategoryPicker variant="filter", même
                composant que QuestionsPage.jsx et GamePage.jsx. */}
            <CategoryPicker
              variant="filter"
              className="questions-page-filter-bar"
              showLabel
              value={selectedCategories}
              availableCategories={availableCategories}
              customCategories={customCategories}
              onToggle={toggleCategoryFilter}
              onClear={clearCategoryFilters}
            />
            <div className="rafale-filter-group">
              <DifficultyPicker
                mode="single"
                allowNone
                ghost
                value={selectedDifficulty}
                onChange={setSelectedDifficulty}
              />
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
                  disabled={resettingAll || roundLocked}
                  title={L.resetAllTitle}
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
            <p className="rafale-status">{L.emptyFilter}</p>
          ) : (
            <div className="rafale-table-wrap">
              <table className="rafale-table">
                <thead>
                  <tr>
                    <th>{L.entryColumn}</th>
                    {inst.hasAnswer && <th>{L.answerColumn}</th>}
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
                      {inst.hasAnswer && <td>{q.ANSWER}</td>}
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
                            disabled={resettingId === q.ID || roundLocked}
                            title="Remettre disponible"
                          >
                            ↺
                          </button>
                        )}
                        <button type="button" className="rafale-row-btn rafale-row-btn-danger" onClick={() => handleDelete(q.ID)} disabled={roundLocked} title="Supprimer">🗑</button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}

          <p className="rafale-counts">
            {questions.length} {L.noun}{questions.length > 1 ? 's' : ''} · {usedCount} utilisee{usedCount > 1 ? 's' : ''} · {questions.length - usedCount} disponible{questions.length - usedCount > 1 ? 's' : ''}
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
              <h2 className="section-title">{form.ID ? L.formTitleEdit : L.formTitleNew}</h2>
              <span className="section-icon">{form.ID ? '✎' : '➕'}</span>
            </div>

            {inst.pasteList && !form.ID && (
              <div className="rafale-entry-mode" role="group" aria-label="Mode de saisie">
                <button type="button" className={`rafale-chip ${entryMode === 'one' ? 'on' : ''}`} aria-pressed={entryMode === 'one'} onClick={() => setEntryMode('one')}>Un mot</button>
                <button type="button" className={`rafale-chip ${entryMode === 'list' ? 'on' : ''}`} aria-pressed={entryMode === 'list'} onClick={() => setEntryMode('list')}>Une liste</button>
              </div>
            )}

            {inst.pasteList && !form.ID && entryMode === 'list' ? (
              <form onSubmit={handleSubmitList} className="rafale-form" data-testid="pool-list-form">
                <div className="form-group">
                  <label>Mots — un par ligne</label>
                  <textarea
                    value={listText}
                    onChange={e => setListText(e.target.value)}
                    rows={8}
                    placeholder={'Tortue\nLion\nZèbre'}
                  />
                </div>
                <div className="form-group">
                  <CategoryPicker
                    mode="single"
                    value={listCategory}
                    onChange={setListCategory}
                    customCategories={customCategories}
                  />
                </div>
                <div className="form-group">
                  <DifficultyPicker
                    mode="single"
                    label="Difficulté du lot"
                    value={listDifficulty}
                    onChange={setListDifficulty}
                  />
                </div>

                {formError && <p className="rafale-status rafale-status-error">{formError}</p>}

                {listReport && (
                  <div className="rafale-list-report" role="status">
                    <p><b>{listReport.created.length} ajouté{listReport.created.length > 1 ? 's' : ''}</b>{listReport.created.length > 0 ? ` — ${listReport.created.map(c => c.QUESTION).join(', ')}` : ''}</p>
                    {listReport.refused.length > 0 && (
                      <>
                        <p><b>{listReport.refused.length} refusé{listReport.refused.length > 1 ? 's' : ''}</b></p>
                        <ul>
                          {listReport.refused.map((r, i) => (
                            <li key={`${r.QUESTION}-${i}`}>« {r.QUESTION} » : {REFUSAL_LABELS[r.CODE] || r.CODE}</li>
                          ))}
                        </ul>
                      </>
                    )}
                  </div>
                )}

                <div className="section-actions">
                  <Button type="submit" variant="primary" disabled={submitting || roundLocked || listLines.length === 0} loading={submitting}>
                    {`Ajouter les ${listLines.length} ligne${listLines.length > 1 ? 's' : ''}`}
                  </Button>
                </div>
              </form>
            ) : (
              <form onSubmit={handleSubmit} className="rafale-form">
                <div className="form-group">
                  <label>{L.entryField}</label>
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
                    {questionRuneCount}/{inst.maxEntryRunes}
                  </span>
                </div>
                {inst.hasAnswer && (
                  <div className="form-group">
                    <label>{L.answerField}</label>
                    <input
                      type="text"
                      value={form.ANSWER}
                      onChange={e => setForm(prev => ({ ...prev, ANSWER: e.target.value }))}
                      required
                    />
                    <span className={`rafale-char-count ${answerOverLimit ? 'over' : ''}`}>
                      {answerRuneCount}/{inst.maxAnswerRunes}
                    </span>
                  </div>
                )}
                <div className="form-group">
                  {/* CategoryPicker (#221, ex-CategorySelector v8.0.0 #16/#197) — même
                      composant que l'éditeur de question standard (QuestionsPage.jsx). */}
                  <CategoryPicker
                    mode="single"
                    value={form.CATEGORY}
                    onChange={(key) => setForm(prev => ({ ...prev, CATEGORY: key }))}
                    customCategories={customCategories}
                  />
                </div>
                <div className="form-group">
                  <DifficultyPicker
                    mode="single"
                    label="Difficulté"
                    chipTitle={(d) => `Difficulte de la question : ${d}`}
                    value={form.DIFFICULTY}
                    onChange={(d) => setForm(prev => ({ ...prev, DIFFICULTY: d }))}
                  />
                </div>

                {formError && <p className="rafale-status rafale-status-error">{formError}</p>}

                <div className="section-actions">
                  <Button
                    type="submit"
                    variant="primary"
                    disabled={submitting || roundLocked || questionOverLimit || (inst.hasAnswer && answerOverLimit)}
                    loading={submitting}
                    title={questionOverLimit || (inst.hasAnswer && answerOverLimit) ? 'Raccourcissez le champ en dépassement avant d\'enregistrer' : undefined}
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
            )}
          </Card>
        </motion.div>
      </div>

      {showAIModal && (
        <PoolAIGenerateModal
          instance={instance}
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
