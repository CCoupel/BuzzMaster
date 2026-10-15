import { useId } from 'react'
import { DIFFICULTY_LEVELS } from '../utils/categoryUtils'
import './DifficultyPicker.css'

function toSet(value) {
  if (value instanceof Set) return value
  if (Array.isArray(value)) return new Set(value)
  if (value === null || value === undefined) return new Set()
  return new Set([value])
}

/**
 * DifficultyPicker — sélecteur de difficulté (★) partagé (#221).
 *
 * Trois modes :
 *  - `single`            : une seule valeur, radio strict (re-clic sur l'active = no-op)
 *  - `single + allowNone`: une seule ou aucune (re-clic désélectionne → onChange(null))
 *  - `multi`             : plusieurs valeurs, `aria-pressed` + ✓
 *
 * Le mode est annoncé en texte (badge « une seule » / « plusieurs possibles » /
 * « une seule ou aucune »), jamais par la seule couleur.
 *
 * @param {Object} props
 * @param {'single'|'multi'} [props.mode='single']
 * @param {number|null|number[]|Set<number>} props.value
 * @param {(d: number|null) => void} [props.onChange] - single ; null seulement si allowNone
 * @param {(d: number) => void} [props.onToggle] - multi
 * @param {boolean} [props.allowNone=false] - single : re-clic désélectionne
 * @param {number[]} [props.levels] - défaut DIFFICULTY_LEVELS
 * @param {boolean} [props.ghost=false] - affiche les étoiles vides (★☆☆)
 * @param {React.ReactNode} [props.label] - défaut « Difficulté » / « Difficultés »
 * @param {boolean} [props.showModeBadge=true]
 * @param {(d: number) => string} [props.chipTitle] - infobulle d'une chip (défaut « Difficulte N ») ;
 *   à surcharger quand deux pickers cohabitent sur la même page (filtre + formulaire)
 * @param {string} [props.className]
 */
export default function DifficultyPicker({
  mode = 'single',
  value,
  onChange,
  onToggle,
  allowNone = false,
  levels = DIFFICULTY_LEVELS,
  ghost = false,
  label,
  showModeBadge = true,
  chipTitle = (d) => `Difficulte ${d}`,
  className,
}) {
  const labelId = useId()
  const isMulti = mode === 'multi'
  const selected = toSet(value)
  const maxLevel = Math.max(...levels)
  const resolvedLabel = label ?? (isMulti ? 'Difficultés' : 'Difficulté')
  const modeText = isMulti ? 'plusieurs possibles' : (allowNone ? 'une seule ou aucune' : 'une seule')

  const handleClick = (d) => {
    if (isMulti) {
      onToggle?.(d)
    } else if (selected.has(d)) {
      if (allowNone) onChange?.(null)
    } else {
      onChange?.(d)
    }
  }

  return (
    <div className={`difficulty-picker-wrap${className ? ` ${className}` : ''}`}>
      <div className="picker-label" id={labelId}>
        <span className="picker-label-text">{resolvedLabel}</span>
        {showModeBadge && (
          <span className={`picker-mode ${isMulti ? 'multi' : 'single'}`}>{modeText}</span>
        )}
      </div>
      <div
        className="difficulty-picker"
        role={isMulti ? 'group' : 'radiogroup'}
        aria-labelledby={labelId}
      >
        {levels.map(d => {
          const active = selected.has(d)
          const a11y = isMulti ? { 'aria-pressed': active } : { role: 'radio', 'aria-checked': active }
          return (
            <button
              key={d}
              type="button"
              className={`diff-chip${active ? ' active' : ''}`}
              onClick={() => handleClick(d)}
              title={chipTitle(d)}
              {...a11y}
            >
              {isMulti && active && <span className="check">✓</span>}
              {'★'.repeat(d)}
              {ghost && maxLevel > d && <span className="ghost">{'☆'.repeat(maxLevel - d)}</span>}
            </button>
          )
        })}
      </div>
    </div>
  )
}
