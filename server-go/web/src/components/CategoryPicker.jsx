import { useId, useState } from 'react'
import { CATEGORIES, categoryMeta, firstFreePaletteColor } from '../utils/categoryUtils'
import CategoryBadge from './CategoryBadge'
import CategoryColorSwatch from './CategoryColorSwatch'
import CategoryManagerDialog from './CategoryManagerDialog'
import { PickerHeader, toSet } from './pickerCommon'
import './CategoryPicker.css'

/**
 * CategoryPicker — sélecteur de catégorie partagé (#221), remplace
 * CategorySelector, CategoryFilterBar et les chips `rafale-multi-chip` /
 * `ai-chip--category`.
 *
 * Modes :
 *  - `single` (défaut) : une seule catégorie, `role="radiogroup"`/`aria-checked`,
 *    re-clic sur l'active → `onChange('')`. `allowCreate` ajoute le « + Créer ».
 *  - `multi` : plusieurs catégories, `aria-pressed` + ✓, `onToggle(key)`.
 *  - `variant="filter"` (implique multi) : icônes rondes + « × » ; restreint à
 *    `availableCategories` ; `showLabel` ajoute le libellé à côté de l'icône.
 *
 * Garde #197 (INTERNE, jamais confiée à l'appelant) : `customCategories` peut
 * être la réponse BRUTE de GET /api/categories (8 miroirs isCustom:false +
 * customs) — seule la liste `[CATEGORIES + customs.filter(isCustom)]` est
 * itérée, donc aucune clé React dupliquée.
 *
 * @param {Object} props
 * @param {'single'|'multi'} [props.mode='single']
 * @param {'chips'|'filter'} [props.variant='chips']
 * @param {string|string[]|Set<string>} props.value
 * @param {(key: string) => void} [props.onChange] - single
 * @param {(key: string) => void} [props.onToggle] - multi / filter
 * @param {() => void} [props.onClear] - filter : bouton × (si sélection non vide)
 * @param {Array} [props.customCategories] - GET /api/categories (brut accepté)
 * @param {string[]} [props.availableCategories] - filter : clés proposées
 * @param {boolean} [props.allowCreate=false] - single : formulaire inline de création
 * @param {() => Promise<void>|void} [props.onRefetchCategories] - rappelé AVANT onChange(created.key)
 * @param {React.ReactNode} [props.label]
 * @param {boolean} [props.showModeBadge=true]
 * @param {boolean} [props.showLabel=true] - filter : libellé texte à côté de l'icône
 * @param {(key: string) => React.ReactNode} [props.renderExtra] - multi : contenu additionnel dans la chip
 * @param {'sm'|'md'|'lg'} [props.size='md']
 * @param {string} [props.className]
 */
export default function CategoryPicker({
  mode = 'single',
  variant = 'chips',
  value,
  onChange,
  onToggle,
  onClear,
  customCategories = [],
  availableCategories,
  allowCreate = false,
  onRefetchCategories,
  label,
  showModeBadge = true,
  showLabel = true,
  renderExtra,
  size = 'md',
  className,
}) {
  const labelId = useId()
  const isFilter = variant === 'filter'
  const isMulti = isFilter || mode === 'multi'
  const canCreate = allowCreate && !isMulti

  const [showAddCategory, setShowAddCategory] = useState(false)
  const [newCategoryName, setNewCategoryName] = useState('')
  const [newCategoryFile, setNewCategoryFile] = useState(null)
  const [addCategoryError, setAddCategoryError] = useState('')
  const [newCategoryColor, setNewCategoryColor] = useState('')
  const [showManager, setShowManager] = useState(false)

  const resetCreateForm = () => {
    setShowAddCategory(false)
    setNewCategoryName('')
    setNewCategoryFile(null)
    setAddCategoryError('')
    setNewCategoryColor('')
  }

  const openCreateForm = () => {
    setNewCategoryColor(firstFreePaletteColor(customCategories))
    setShowAddCategory(true)
    setAddCategoryError('')
  }

  const handleCreate = async () => {
    if (!newCategoryName.trim()) { setAddCategoryError('Nom invalide'); return }
    if (!newCategoryFile) { setAddCategoryError('Image requise'); return }
    try {
      const fd = new FormData()
      fd.append('name', newCategoryName.trim())
      fd.append('file', newCategoryFile)
      if (newCategoryColor) fd.append('color', newCategoryColor)
      // Ne PAS définir Content-Type — le browser gère le boundary
      const res = await fetch('/api/categories', { method: 'POST', body: fd })
      if (res.ok) {
        const created = await res.json()
        resetCreateForm()
        await onRefetchCategories?.()
        onChange?.(created.key)
      } else if (res.status === 409) {
        let body = {}
        try { body = await res.json() } catch { body = {} }
        setAddCategoryError(body.error === 'game in progress' ? 'Impossible pendant une partie' : 'Cette catégorie existe déjà')
      } else {
        setAddCategoryError('Nom invalide ou image non supportée')
      }
    } catch {
      setAddCategoryError('Erreur réseau')
    }
  }

  // Garde #197 : voir la doc du composant. Un seul chemin pour construire la liste.
  // #285 : categoryMeta reçoit la liste BRUTE (surcharges nom/couleur des intégrées).
  const trueCustomCategories = customCategories.filter(c => c.isCustom)
  const allKeys = [...Object.keys(CATEGORIES), ...trueCustomCategories.map(c => c.key)]
  const keys = isFilter && availableCategories ? availableCategories : allKeys

  if (isFilter && keys.length === 0) return null

  const selected = toSet(value)
  const resolvedLabel = label ?? (isFilter ? 'Filtrer' : isMulti ? 'Catégories' : 'Catégorie')
  const iconOnly = isFilter && !showLabel

  return (
    <div className={`category-picker-wrap${className ? ` ${className}` : ''}`}>
      <PickerHeader
        id={labelId}
        label={resolvedLabel}
        isMulti={isMulti}
        modeText={isMulti ? 'plusieurs possibles' : 'une seule'}
        showModeBadge={showModeBadge}
      />
      <div
        className="category-picker"
        role={isMulti ? 'group' : 'radiogroup'}
        aria-labelledby={labelId}
      >
        {keys.map(key => {
          const meta = categoryMeta(key, customCategories)
          if (!meta) return null
          const active = selected.has(key)
          const a11y = isMulti
            ? { 'aria-pressed': active }
            : { role: 'radio', 'aria-checked': active }
          return (
            <button
              key={key}
              type="button"
              className={`cat-chip${active ? ' active' : ''}${iconOnly ? ' cat-chip--icon-only' : ''}`}
              style={{ '--cat-color': meta.color }}
              onClick={() => (isMulti ? onToggle?.(key) : onChange?.(active ? '' : key))}
              title={meta.label}
              {...a11y}
            >
              <span className="cat-chip-icon">
                <CategoryBadge catKey={key} customCategories={customCategories} size={size} chip={false} />
              </span>
              {!iconOnly && <span className="cat-chip-label">{meta.label}</span>}
              {isMulti && renderExtra?.(key)}
              {isMulti && active && !iconOnly && <span className="check">✓</span>}
            </button>
          )
        })}
        {canCreate && !showAddCategory && (
          <button
            type="button"
            className="cat-chip cat-chip--add"
            title="Créer une catégorie"
            onClick={openCreateForm}
          >
            + Créer
          </button>
        )}
        {allowCreate && (
          <button
            type="button"
            className="cat-chip cat-chip--add cat-chip--manage"
            title="Gérer les catégories : couleur, icône, nom, suppression"
            onClick={() => setShowManager(true)}
          >
            ⚙ Gérer…
          </button>
        )}
        {isFilter && selected.size > 0 && (
          <button type="button" className="picker-reset" onClick={onClear} title="Réinitialiser les filtres">
            ×
          </button>
        )}
      </div>
      {canCreate && showAddCategory && (
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
          <span className="add-category-color">
            <CategoryColorSwatch value={newCategoryColor} onChange={setNewCategoryColor} label="Couleur de la catégorie" />
            <span className="add-category-color-hint">Couleur</span>
          </span>
          <div className="add-category-actions">
            <button type="button" className="add-category-validate" onClick={handleCreate}>
              Valider
            </button>
            <button type="button" className="add-category-cancel" onClick={resetCreateForm}>
              Annuler
            </button>
          </div>
          {addCategoryError && <p className="add-category-error">{addCategoryError}</p>}
        </div>
      )}
      {showManager && (
        <CategoryManagerDialog
          categories={customCategories}
          onClose={() => setShowManager(false)}
          onChanged={onRefetchCategories}
          onDeleted={(key) => {
            if (!selected.has(key)) return
            if (isMulti) onToggle?.(key)
            else onChange?.('')
          }}
        />
      )}
    </div>
  )
}
