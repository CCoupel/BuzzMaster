import { useId, useState } from 'react'
import { CATEGORIES, categoryMeta } from '../utils/categoryUtils'
import CategoryBadge from './CategoryBadge'
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
 *    re-clic sur l'active → `onChange('')`.
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
 * @param {boolean} [props.allowManage=false] - ajoute « ⚙ Gérer… » (dialogue de gestion : création, couleur, icône, nom, suppression)
 * @param {() => Promise<void>|void} [props.onRefetchCategories] - refetch de repli du dialogue de gestion
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
  allowManage = false,
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

  const [showManager, setShowManager] = useState(false)

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
        {allowManage && (
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
