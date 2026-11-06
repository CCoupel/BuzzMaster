import { useEffect, useRef, useState } from 'react'
import { CATEGORY_PALETTE } from '../utils/categoryUtils'
import './CategoryColorSwatch.css'

/**
 * CategoryColorSwatch — pastille de couleur = bouton ouvrant la palette (#285,
 * maquette admin_category_manager). 12 teintes + « aucune » (`""`).
 *
 * @param {string} props.value - `#rrggbb` ou `""` (aucune)
 * @param {(color: string) => void} props.onChange
 * @param {string} [props.label='Couleur'] - nom accessible du bouton
 * @param {boolean} [props.disabled]
 */
export default function CategoryColorSwatch({ value, onChange, label = 'Couleur', disabled = false }) {
  const [open, setOpen] = useState(false)
  const ref = useRef(null)

  useEffect(() => {
    if (!open) return undefined
    const onDown = (e) => { if (ref.current && !ref.current.contains(e.target)) setOpen(false) }
    const onKey = (e) => { if (e.key === 'Escape') { e.stopPropagation(); setOpen(false) } }
    document.addEventListener('mousedown', onDown)
    document.addEventListener('keydown', onKey, true)
    return () => {
      document.removeEventListener('mousedown', onDown)
      document.removeEventListener('keydown', onKey, true)
    }
  }, [open])

  const pick = (color) => { setOpen(false); onChange?.(color) }

  return (
    <span className="cat-swatch-wrap" ref={ref}>
      <button
        type="button"
        className={`cat-swatch${value ? '' : ' cat-swatch--none'}`}
        style={{ '--c': value || 'transparent' }}
        aria-label={label}
        title={label}
        aria-haspopup="true"
        aria-expanded={open}
        disabled={disabled}
        onClick={() => setOpen(o => !o)}
      />
      {open && (
        <div className="cat-pal-pop">
          <p className="cat-pal-title">Teinte</p>
          <div className="cat-palette">
            <button
              type="button"
              className={`cat-pal-dot cat-pal-dot--none${value ? '' : ' sel'}`}
              aria-label="Aucune"
              title="Aucune (salle blanche)"
              onClick={() => pick('')}
            />
            {CATEGORY_PALETTE.map(c => (
              <button
                key={c}
                type="button"
                className={`cat-pal-dot${value?.toLowerCase() === c ? ' sel' : ''}`}
                style={{ background: c }}
                aria-label={c}
                title={c}
                onClick={() => pick(c)}
              />
            ))}
          </div>
        </div>
      )}
    </span>
  )
}
