import { useState, useEffect } from 'react'
import './RafalePoolAlert.css'

/**
 * RafalePoolAlert — alerte de disponibilité du pool RAFALE (contrat
 * rafale.md §7.2, maquette docs/mockups/rafale-v8.html §2), partagée entre
 * `QuestionsPage.jsx` (édition d'une manche RAFALE) et `GamePage.jsx`
 * (avant lancement). Un seul composant, un seul appel à
 * `GET /api/rafale/pool` — pas de logique dupliquée entre les deux pages.
 *
 * Trois états normatifs (contrat §7.2) :
 *   - `disponibles === 0` → **bloquant** (rouge)
 *   - `disponibles < besoin_estimé` → **avertissement** (orange), démarrage
 *     autorisé quand même
 *   - `disponibles >= besoin_estimé` → **neutre** (vert)
 *
 * `besoin_estimé = plafond(roundTime / questionTime)` — estimation
 * MAJORANTE (suppose que chaque question consomme tout son temps), à
 * présenter comme un plancher de sécurité (§7.2).
 *
 * @param {Object} props
 * @param {string[]} props.categories - RAFALE_CATEGORIES de la manche (#216,
 *   réouverture assumée de #107, contrat §3.3/§9 : union catégories ×
 *   difficultés, au moins une catégorie et une difficulté requises)
 * @param {number[]} props.difficulties - RAFALE_DIFFICULTIES (chaque valeur 1..3)
 * @param {number} props.roundTime - durée de la manche en secondes (TIME)
 * @param {number} props.questionTime - secondes par question (RAFALE_QUESTION_TIME)
 * @param {string} [props.className]
 * @param {(level: 'blocking'|'warning'|'ok'|null) => void} [props.onLevelChange] -
 *   notifié à chaque recalcul du niveau (`null` tant qu'aucun pool n'est
 *   encore résolu — filtre incomplet, chargement, ou erreur). Consommé par
 *   `GamePage.jsx` pour bloquer le bouton START en RAFALE quand le pool est
 *   vide (§7.2 : "disponibles == 0 → Bloquant — démarrage refusé"),
 *   `QuestionsPage.jsx` l'ignore (pas de bouton à bloquer en édition).
 */
export default function RafalePoolAlert({ categories = [], difficulties = [], roundTime, questionTime, className = '', onLevelChange }) {
  const [pool, setPool] = useState(null) // { AVAILABLE, USED, TOTAL } | null
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState(null)

  const hasFilter = categories.length > 0 && difficulties.length > 0 && difficulties.every(d => d >= 1 && d <= 3)
  // Clés stables (chaîne) plutôt que les tableaux eux-mêmes en dépendance
  // d'effet — `formData.rafaleCategories`/`rafaleDifficulties` (appelant
  // QuestionsPage.jsx) sont de NOUVEAUX tableaux à chaque `setFormData`, même
  // quand leur contenu est identique (ex. un re-render déclenché par un
  // autre champ du formulaire) : sans cette stabilisation, un effet par
  // référence redéclencherait un fetch réseau à chaque frappe ailleurs dans
  // le formulaire, pas seulement au changement réel du filtre.
  const categoriesKey = categories.join(',')
  const difficultiesKey = difficulties.join(',')

  useEffect(() => {
    if (!hasFilter) {
      setPool(null)
      setError(null)
      return
    }
    let cancelled = false
    setLoading(true)
    setError(null)
    const params = new URLSearchParams()
    // #216 — pluriel virgule-séparé (contrat §9, union sur le produit
    // cartésien), même convention que GET /api/rafale/questions.
    params.set('categories', categoriesKey)
    params.set('difficulties', difficultiesKey)
    fetch(`/api/rafale/pool?${params.toString()}`)
      .then(res => {
        if (!res.ok) throw new Error(`HTTP ${res.status}`)
        return res.json()
      })
      .then(data => {
        if (!cancelled) {
          setPool(data)
          setLoading(false)
        }
      })
      .catch(err => {
        if (!cancelled) {
          setError(err.message)
          setLoading(false)
        }
      })
    return () => { cancelled = true }
  }, [categoriesKey, difficultiesKey, hasFilter])

  const need = (roundTime > 0 && questionTime > 0) ? Math.ceil(roundTime / questionTime) : 0
  const level = (hasFilter && !loading && !error && pool)
    ? (pool.AVAILABLE === 0 ? 'blocking' : (pool.AVAILABLE < need ? 'warning' : 'ok'))
    : null

  useEffect(() => {
    onLevelChange?.(level)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [level])

  if (!hasFilter) {
    return (
      <div className={`rafale-pool-alert rafale-pool-alert-neutral ${className}`}>
        <span className="rafale-pool-alert-icon">ℹ️</span>
        <div>Sélectionnez au moins une catégorie et une difficulté pour estimer le pool disponible.</div>
      </div>
    )
  }

  if (loading || error || !pool) {
    return (
      <div className={`rafale-pool-alert rafale-pool-alert-neutral ${className}`}>
        <span className="rafale-pool-alert-icon">ℹ️</span>
        <div>{error ? `Erreur : ${error}` : 'Vérification du pool…'}</div>
      </div>
    )
  }

  // Retour utilisateur (2026-08-30, #198) — "réduis le texte indiquant
  // qu'il n'y a plus de questions ou qu'il n'y en a pas assez" : messages
  // blocking/warning raccourcis à l'essentiel (combien de disponibles,
  // bloquant ou juste avertissement) — l'état `ok` n'était pas visé par le
  // retour, inchangé.
  const levelMeta = {
    blocking: { icon: '✕', cls: 'rafale-pool-alert-blocking', title: '0 disponible — bloquant' },
    warning: { icon: '!', cls: 'rafale-pool-alert-warning', title: `${pool.AVAILABLE} question${pool.AVAILABLE > 1 ? 's' : ''} disponible${pool.AVAILABLE > 1 ? 's' : ''} — insuffisant` },
    ok: { icon: '✓', cls: 'rafale-pool-alert-ok', title: `${pool.AVAILABLE} question${pool.AVAILABLE > 1 ? 's' : ''} disponible${pool.AVAILABLE > 1 ? 's' : ''}` },
  }[level]

  return (
    <div className={`rafale-pool-alert ${levelMeta.cls} ${className}`}>
      <span className="rafale-pool-alert-icon">{levelMeta.icon}</span>
      <div>
        <b>{levelMeta.title}</b>
        {need > 0 && (
          <p className="rafale-pool-alert-detail">
            {level === 'blocking'
              ? 'Changez de filtre ou réinitialisez le pool.'
              : level === 'warning'
                ? `~${need} nécessaires — démarrage autorisé.`
                : `~${need} nécessaires.`}
          </p>
        )}
      </div>
    </div>
  )
}
