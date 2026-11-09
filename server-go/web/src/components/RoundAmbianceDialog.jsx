import { useEffect, useRef, useState } from 'react'
import AmbianceSettingsPanel from './AmbianceSettingsPanel'
import { setPartialPath, countOverrides } from '../utils/ambianceSettings'
import './RoundGroup.css'

/**
 * #290 (LOT-4, T21) — fenêtre « Ambiance de la manche » : héberge le composant MUTUALISÉ
 * AmbianceSettingsPanel en mode « override » (tri-états « Hérité / … », valeur globale en italique).
 * Maquette ambiance_settings_panel__round_override. Aucun catalogue (une manche n'en change pas les sons).
 * L'enregistrement transporte `AMBIANCE` dans ROUNDS_UPDATE (structure des manches) — rien n'est écrit dans le global.
 *
 * Props : roundName, value (Partial courant de la manche), onSave(partial), onCancel(),
 *   base / cues : facultatifs (sinon lus par GET /api/ambiance/settings (`global`) et GET /api/sounds).
 */
export default function RoundAmbianceDialog({ roundName, value, base: baseProp = null, cues: cuesProp = null, onSave, onCancel }) {
  const [draft, setDraft] = useState(() => value || {})
  const [base, setBase] = useState(baseProp)
  const [cues, setCues] = useState(cuesProp || [])
  const [loadError, setLoadError] = useState(null)
  const boxRef = useRef(null)
  const closeRef = useRef(null)

  useEffect(() => { closeRef.current?.focus() }, [])

  useEffect(() => {
    if (baseProp) return undefined
    let alive = true
    fetch('/api/ambiance/settings')
      .then(r => (r.ok ? r.json() : Promise.reject(new Error(`HTTP ${r.status}`))))
      .then(d => { if (alive) setBase(d?.global || null) })
      .catch(e => { if (alive) setLoadError(e.message) })
    return () => { alive = false }
  }, [baseProp])

  useEffect(() => {
    if (cuesProp) return undefined
    let alive = true
    fetch('/api/sounds')
      .then(r => (r.ok ? r.json() : Promise.reject(new Error(`HTTP ${r.status}`))))
      .then(d => { if (alive) setCues(Array.isArray(d?.cues) ? d.cues.map(c => ({ cue: c.cue, type: c.type || '' })) : []) })
      .catch(e => { if (alive) setLoadError(e.message) })
    return () => { alive = false }
  }, [cuesProp])

  const n = countOverrides(draft)
  const onChange = (path, v) => setDraft(prev => setPartialPath(prev, path, v))

  const onKeyDown = (e) => {
    if (e.key === 'Escape') { e.stopPropagation(); onCancel?.(); return }
    if (e.key !== 'Tab') return
    const f = boxRef.current?.querySelectorAll('button:not(:disabled), input:not(:disabled)')
    if (!f?.length) return
    const first = f[0]
    const last = f[f.length - 1]
    if (e.shiftKey && document.activeElement === first) { e.preventDefault(); last.focus() }
    else if (!e.shiftKey && document.activeElement === last) { e.preventDefault(); first.focus() }
  }

  return (
    <div className="round-modal" onMouseDown={(e) => { if (e.target === e.currentTarget) onCancel?.() }}>
      <div
        className="round-modal-box round-ambiance-box"
        role="dialog"
        aria-modal="true"
        aria-labelledby="round-amb-title"
        ref={boxRef}
        onKeyDown={onKeyDown}
      >
        <div className="round-amb-head">
          <h3 id="round-amb-title">Ambiance de la manche « {roundName} »</h3>
          <span className="round-ambiance-badge" role="status">
            {n === 0 ? 'Hérite tout du réglage global' : `${n} réglage${n > 1 ? 's' : ''} surchargé${n > 1 ? 's' : ''}`}
          </span>
        </div>
        <p className="round-amb-hint">
          Par défaut une manche hérite des réglages globaux (page Ambiance). Choisissez une valeur pour la
          surcharger pendant cette manche seulement.
        </p>
        {loadError && <p className="round-amb-hint" role="alert">Valeurs globales indisponibles ({loadError}).</p>}
        <AmbianceSettingsPanel
          mode="override"
          value={draft}
          base={base}
          cues={cues}
          onChange={onChange}
        />
        <div className="round-modal-actions round-amb-actions">
          <button type="button" className="round-btn" disabled={n === 0} onClick={() => onChange('*', null)}>
            Tout remettre à « Hérité »
          </button>
          <button type="button" className="round-btn" ref={closeRef} onClick={onCancel}>Annuler</button>
          <button type="button" className="round-btn primary" onClick={() => onSave?.(draft)}>Enregistrer</button>
        </div>
      </div>
    </div>
  )
}
