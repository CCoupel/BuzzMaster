import { useState } from 'react'
import { CATEGORIES, categoryMeta } from '../utils/categoryUtils'
import CategoryBadge from './CategoryBadge'
import './CategorySelector.css'

/**
 * CategorySelector — sélecteur de catégorie UNIQUE (icônes + bouton "+" de
 * création), extrait de QuestionsPage.jsx (#95/#97/#100) pour être partagé
 * avec RafalePage.jsx (v8.0.0, #16/#197) — bugfix de cohérence UI :
 * l'éditeur du réservoir RAFALE utilisait un `<select>` texte, distinct du
 * sélecteur icônes des questions standard. Un seul composant désormais,
 * jamais deux variantes de la même fonctionnalité.
 *
 * Simple sélection (une catégorie ou aucune) — pour un multi-sélecteur
 * (RAFALE_CATEGORIES, contrat rafale.md §3.3), voir QuestionsPage.jsx, qui
 * réutilise `category-selector`/`category-btn`/`CategoryBadge` mais avec sa
 * propre logique de toggle-array (pas ce composant).
 *
 * @param {Object} props
 * @param {string} props.value - clé de la catégorie sélectionnée ('' = aucune)
 * @param {(key: string) => void} props.onChange - appelé avec la NOUVELLE
 *   valeur ('' si la catégorie déjà sélectionnée est re-cliquée — toggle,
 *   même comportement qu'avant l'extraction)
 * @param {Array} props.customCategories - GET /api/categories (useCategories())
 * @param {() => Promise<void>|void} [props.onRefetchCategories] - rappelé
 *   après création réussie d'une catégorie, avant `onChange(created.key)` —
 *   nécessaire pour que la nouvelle catégorie apparaisse dans la grille
 *   (le composant ne détient pas son propre état de catégories).
 * @param {boolean} [props.allowCreate] - affiche le bouton "+" de création
 *   inline (#97/#100). `true` par défaut — RafalePage.jsx et QuestionsPage.jsx
 *   en ont tous deux besoin (aucune raison connue de le masquer).
 * @param {'sm'|'md'|'lg'} [props.size] - taille des icônes (CategoryBadge)
 */
export default function CategorySelector({
  value,
  onChange,
  customCategories = [],
  onRefetchCategories,
  allowCreate = true,
  size = 'lg',
}) {
  const [showAddCategory, setShowAddCategory] = useState(false)
  const [newCategoryName, setNewCategoryName] = useState('')
  const [newCategoryFile, setNewCategoryFile] = useState(null)
  const [addCategoryError, setAddCategoryError] = useState('')

  const handleCreate = async () => {
    if (!newCategoryName.trim()) { setAddCategoryError('Nom invalide'); return }
    if (!newCategoryFile) { setAddCategoryError('Image requise'); return }
    try {
      const fd = new FormData()
      fd.append('name', newCategoryName.trim())
      fd.append('file', newCategoryFile)
      // Ne PAS définir Content-Type — le browser gère le boundary
      const res = await fetch('/api/categories', { method: 'POST', body: fd })
      if (res.ok) {
        const created = await res.json()
        setShowAddCategory(false)
        setNewCategoryName('')
        setNewCategoryFile(null)
        setAddCategoryError('')
        await onRefetchCategories?.()
        onChange(created.key)
      } else if (res.status === 409) {
        setAddCategoryError('Cette catégorie existe déjà')
      } else {
        setAddCategoryError('Nom invalide ou image non supportée')
      }
    } catch {
      setAddCategoryError('Erreur réseau')
    }
  }

  const selectedMeta = value ? categoryMeta(value, customCategories) : null

  return (
    <div className="category-selector-wrap">
      <div className="category-selector">
        {[...Object.keys(CATEGORIES), ...customCategories.map(c => c.key)].map(key => {
          const meta = categoryMeta(key, customCategories)
          if (!meta) return null
          return (
            <button
              key={key}
              type="button"
              className={`category-btn ${value === key ? 'active' : ''}`}
              style={{ '--cat-color': meta.color }}
              onClick={() => onChange(value === key ? '' : key)}
              title={meta.label}
            >
              <CategoryBadge catKey={key} customCategories={customCategories} size={size} chip={false} />
            </button>
          )
        })}
        {allowCreate && !showAddCategory && (
          <button
            type="button"
            className="category-btn category-btn--add"
            title="Créer une catégorie"
            onClick={() => { setShowAddCategory(true); setAddCategoryError('') }}
          >
            +
          </button>
        )}
      </div>
      {allowCreate && showAddCategory && (
        <div className="add-category-inline">
          <input
            type="text"
            className="add-category-input"
            placeholder="Nom de la catégorie..."
            value={newCategoryName}
            maxLength={50}
            onChange={(e) => { setNewCategoryName(e.target.value); setAddCategoryError('') }}
            onKeyDown={(e) => { if (e.key === 'Escape') { setShowAddCategory(false); setNewCategoryName(''); setNewCategoryFile(null) } }}
            autoFocus
          />
          <label className="add-category-file-label">
            <input
              type="file"
              accept=".png,.jpg,.jpeg,.webp"
              style={{ display: 'none' }}
              onChange={(e) => { setNewCategoryFile(e.target.files[0] || null); setAddCategoryError('') }}
            />
            <span className="add-category-file-btn">
              {newCategoryFile ? '✓ ' + newCategoryFile.name : '📁 Choisir une image…'}
            </span>
          </label>
          <div className="add-category-actions">
            <button type="button" className="add-category-validate" onClick={handleCreate}>
              Valider
            </button>
            <button
              type="button"
              className="add-category-cancel"
              onClick={() => { setShowAddCategory(false); setNewCategoryName(''); setNewCategoryFile(null); setAddCategoryError('') }}
            >
              Annuler
            </button>
          </div>
          {addCategoryError && <p className="add-category-error">{addCategoryError}</p>}
        </div>
      )}
      {selectedMeta && (
        <span className="category-label" style={{ color: selectedMeta.color }}>{selectedMeta.label}</span>
      )}
    </div>
  )
}
