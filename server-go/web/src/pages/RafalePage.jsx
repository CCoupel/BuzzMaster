import { useState, useEffect, useCallback, useMemo } from 'react'
import { motion } from 'framer-motion'
import { useCategories } from '../hooks/useCategories'
import { useCategoryFilter } from '../hooks/useCategoryFilter'
import { CATEGORIES, categoryMeta } from '../utils/categoryUtils'
import CategoryBadge from '../components/CategoryBadge'
import Button from '../components/Button'
import Card from '../components/Card'
import './RafalePage.css'

const DIFFICULTIES = [1, 2, 3]

const EMPTY_FORM = { ID: null, QUESTION: '', ANSWER: '', CATEGORY: '', DIFFICULTY: 1 }

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
  const { categories: apiCategories } = useCategories()
  const [questions, setQuestions] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)

  const [selectedDifficulty, setSelectedDifficulty] = useState(null) // null = toutes
  const [onlyUnused, setOnlyUnused] = useState(false)

  const [form, setForm] = useState(EMPTY_FORM)
  const [formError, setFormError] = useState(null)
  const [submitting, setSubmitting] = useState(false)

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

  // useCategoryFilter attend q.CATEGORY (case commune Quiz/RAFALE, §3.1).
  const {
    selectedCategories,
    availableCategories,
    filteredQuestions: categoryFiltered,
    toggleCategoryFilter,
    clearCategoryFilters,
  } = useCategoryFilter(questions, apiCategories)

  const filteredQuestions = useMemo(() => {
    return categoryFiltered.filter(q => {
      if (selectedDifficulty && q.DIFFICULTY !== selectedDifficulty) return false
      if (onlyUnused && q.USED) return false
      return true
    })
  }, [categoryFiltered, selectedDifficulty, onlyUnused])

  const usedCount = useMemo(() => questions.filter(q => q.USED).length, [questions])

  // Catalogue complet pour le formulaire — pas dérivé des questions déjà
  // présentes (contrairement à `availableCategories` du filtre) : toutes
  // les catégories connues (enum + custom, §3.1) doivent être proposables
  // à la création, y compris une catégorie encore inutilisée dans le
  // réservoir.
  const formCategoryOptions = useMemo(() => {
    const hardcoded = Object.entries(CATEGORIES).map(([key, meta]) => ({ key, label: meta.label }))
    const custom = apiCategories.filter(c => c.isCustom).map(c => ({ key: c.key, label: c.name }))
    return [...hardcoded, ...custom]
  }, [apiCategories])

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

  const handleSubmit = async (e) => {
    e.preventDefault()
    setFormError(null)

    if (!form.QUESTION.trim() || !form.ANSWER.trim()) {
      setFormError('Enonce et reponse sont obligatoires.')
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
          <div className="rafale-filters">
            <div className="rafale-filter-group">
              <button
                type="button"
                className={`rafale-chip ${selectedCategories.size === 0 ? 'on' : ''}`}
                onClick={clearCategoryFilters}
              >
                Toutes
              </button>
              {availableCategories.map(catKey => (
                <button
                  key={catKey}
                  type="button"
                  className={`rafale-chip ${selectedCategories.has(catKey) ? 'on' : ''}`}
                  onClick={() => toggleCategoryFilter(catKey)}
                >
                  <CategoryBadge catKey={catKey} customCategories={apiCategories} size="sm" chip={false} />
                  {' '}
                  {categoryMeta(catKey, apiCategories)?.label || catKey}
                </button>
              ))}
            </div>
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
                        <CategoryBadge catKey={q.CATEGORY} customCategories={apiCategories} size="sm" />
                      </td>
                      <td>{'★'.repeat(q.DIFFICULTY || 1)}</td>
                      <td className={q.USED ? 'rafale-used' : 'rafale-available'}>
                        {q.USED ? 'utilisee' : 'disponible'}
                      </td>
                      <td className="rafale-row-actions">
                        <button type="button" className="rafale-row-btn" onClick={() => handleEdit(q)} title="Modifier">✎</button>
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
              </div>
              <div className="form-group">
                <label>Reponse</label>
                <input
                  type="text"
                  value={form.ANSWER}
                  onChange={e => setForm(prev => ({ ...prev, ANSWER: e.target.value }))}
                  required
                />
              </div>
              <div className="form-group">
                <label>Categorie</label>
                <select
                  value={form.CATEGORY}
                  onChange={e => setForm(prev => ({ ...prev, CATEGORY: e.target.value }))}
                  required
                >
                  <option value="" disabled>Selectionner...</option>
                  {formCategoryOptions.map(({ key, label }) => (
                    <option key={key} value={key}>{label}</option>
                  ))}
                </select>
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
                <Button type="submit" variant="primary" disabled={submitting} loading={submitting}>
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
    </div>
  )
}
