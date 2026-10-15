import './Picker.css'

/**
 * Normalise une valeur de sélection (Set, tableau, valeur seule, vide) en Set.
 * Partagé par CategoryPicker et DifficultyPicker (#221).
 */
export function toSet(value) {
  if (value instanceof Set) return value
  if (Array.isArray(value)) return new Set(value)
  if (value === null || value === undefined || value === '') return new Set()
  return new Set([value])
}

/**
 * En-tête commun des pickers : libellé + badge de mode en texte
 * (« une seule » / « plusieurs possibles » / « une seule ou aucune »),
 * jamais par la seule couleur.
 */
export function PickerHeader({ id, label, isMulti, modeText, showModeBadge = true }) {
  return (
    <div className="picker-label" id={id}>
      <span className="picker-label-text">{label}</span>
      {showModeBadge && (
        <span className={`picker-mode ${isMulti ? 'multi' : 'single'}`}>{modeText}</span>
      )}
    </div>
  )
}
