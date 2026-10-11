import { useEffect, useRef } from 'react'
import './RoundGroup.css'

/**
 * Fenêtre de suppression d'une manche (#245, maquette (f″)) : deux choix explicites — « Conserver les N entrées »
 * (fin de « Sans manche ») ou « Supprimer aussi les N entrées » (définitif, médias compris) — et « Annuler ».
 * Le focus va sur « Conserver » ; Échap annule ; Tab reste dans la fenêtre.
 */
export default function RoundDeleteDialog({ name, questions = 0, entractes = 0, onKeep, onDelete, onCancel }) {
  const keepRef = useRef(null)
  const boxRef = useRef(null)
  const total = questions + entractes
  useEffect(() => { keepRef.current?.focus() }, [])

  const onKeyDown = (e) => {
    if (e.key === 'Escape') { e.stopPropagation(); onCancel?.(); return }
    if (e.key !== 'Tab') return
    const f = boxRef.current?.querySelectorAll('button')
    if (!f?.length) return
    const first = f[0]
    const last = f[f.length - 1]
    if (e.shiftKey && document.activeElement === first) { e.preventDefault(); last.focus() }
    else if (!e.shiftKey && document.activeElement === last) { e.preventDefault(); first.focus() }
  }

  const q = `${questions} question${questions > 1 ? 's' : ''}`
  const en = `${entractes} entracte${entractes > 1 ? 's' : ''}`
  return (
    <div className="round-modal" onMouseDown={(e) => { if (e.target === e.currentTarget) onCancel?.() }}>
      <div className="round-modal-box" role="dialog" aria-modal="true" aria-labelledby="round-del-title" ref={boxRef} onKeyDown={onKeyDown}>
        <h3 id="round-del-title">Supprimer la manche « {name} » ?</h3>
        {total === 0 ? (
          <>
            <p>Elle est vide.</p>
            <button type="button" className="round-choice danger" ref={keepRef} onClick={onKeep}>
              <b>Supprimer la manche</b>
            </button>
          </>
        ) : (
          <>
            <p>
              Elle contient {q}{entractes ? ` et ${en}` : ''}. Que faire de {total > 1 ? `ces ${total} entrées` : 'cette entrée'} ?
            </p>
            <button type="button" className="round-choice" ref={keepRef} onClick={onKeep}>
              <b>Conserver les {total} entrées</b>
              <span>Elles passent à la fin de « Sans manche » (en tête de liste).</span>
            </button>
            <button type="button" className="round-choice danger" onClick={onDelete}>
              <b>Supprimer aussi les {total} entrées</b>
              <span>Définitif : questions, entractes et leurs médias.</span>
            </button>
          </>
        )}
        <div className="round-modal-actions"><button type="button" className="round-btn" onClick={onCancel}>Annuler</button></div>
      </div>
    </div>
  )
}
