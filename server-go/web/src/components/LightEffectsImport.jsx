import { useEffect, useRef } from 'react'
import Button from './Button'

// #305 (v11.8.0, LOT-1) — import d'effets : panneau de résultat par fichier et fenêtre de conflit.
// Maquette : docs/mockup/v11.8.0/ui/ambiance_page__effects_catalogue_import_export.html §02. Contrat : contracts/light-effects.md §11.7.

// POST /api/lighting/effects/import (§11.7) : multipart, champ `files` répété, `on_conflict` = ask | replace | keep_both.
// -> { ok: true, results, catalogue } | { ok: false, message }.
export async function postImport(files, onConflict = 'ask') {
  const form = new FormData()
  for (const f of files) form.append('files', f, f.name)
  form.append('on_conflict', onConflict)
  try {
    const res = await fetch('/api/lighting/effects/import', { method: 'POST', body: form })
    // Les refus (400, 413) sont du texte brut (http.Error) : on affiche le message du serveur tel quel (§11.7).
    const raw = await res.text().catch(() => '')
    let data = {}
    try { data = raw ? JSON.parse(raw) : {} } catch { /* texte brut */ }
    if (!res.ok) {
      const fallback = res.status === 413 ? 'Envoi trop gros (2 Mo au plus).' : `Import impossible (HTTP ${res.status}).`
      return { ok: false, message: data?.error || data?.message || raw.trim() || fallback }
    }
    return { ok: true, results: Array.isArray(data?.results) ? data.results : [], catalogue: data?.catalogue }
  } catch (e) {
    return { ok: false, message: `Import impossible : ${e.message}` }
  }
}

// DELETE /api/lighting/effects/{id} (§11.5) : supprime l'effet utilisateur, ou le fichier qui remplace un natif.
export async function requestDelete(id) {
  try {
    const res = await fetch(`/api/lighting/effects/${encodeURIComponent(id)}`, { method: 'DELETE' })
    const data = await res.json().catch(() => ({}))
    if (res.ok) return { ok: true, catalogue: data?.catalogue }
    const message = res.status === 409 ? 'Un effet livré ne peut pas être supprimé.'
      : res.status === 404 ? 'Effet inconnu — rechargez le catalogue.'
      : `Suppression impossible (HTTP ${res.status}).`
    return { ok: false, status: res.status, message }
  } catch (e) {
    return { ok: false, message: `Suppression impossible : ${e.message}` }
  }
}

// Une ligne de résultat : { ok, text, errors[] } d'après le `status` du serveur (§11.7).
export function describeResult(r) {
  const name = r.name ? `« ${r.name} »` : ''
  switch (r.status) {
    case 'imported': return { ok: true, text: `${name} importé.` }
    case 'replaced': return { ok: true, text: `${name} remplacé.` }
    case 'renamed': return { ok: true, text: `${name} importé sous l'identifiant ${r.new_id || r.id}.`, code: r.new_id || r.id }
    case 'cancelled': return { ok: null, text: 'ignoré.' }
    case 'native':
      return r.errors?.length
        ? { ok: false, text: 'refusé.', errors: r.errors }
        : { ok: false, text: `refusé. « ${r.id} » est l'identifiant d'un effet livré : changez la clé id du fichier.` }
    case 'invalid': return { ok: false, text: 'refusé.', errors: r.errors }
    case 'too_big': return { ok: false, text: 'refusé. Fichier trop gros (64 Ko au plus).' }
    case 'limit': return { ok: false, text: 'refusé. Le dossier contient déjà 100 effets : supprimez-en avant d\'importer.' }
    case 'conflict': return { ok: false, text: 'en attente de votre choix.' }
    default: return { ok: false, text: `refusé (${r.status || 'résultat inconnu'}).` }
  }
}

export function ImportResults({ rows, onClose }) {
  if (!rows?.length) return null
  return (
    <section className="lfx-res" aria-label="Résultat de l'import" data-testid="import-result">
      <div className="lfx-res-head">
        <h4 className="lfx-res-title">Résultat de l'import</h4>
        <span className="lfx-spacer" />
        <Button variant="secondary" size="sm" onClick={onClose}>Fermer</Button>
      </div>
      <ul className="lfx-res-list">
        {rows.map((r, i) => {
          const d = describeResult(r)
          const cls = d.ok === true ? 'ok' : d.ok === null ? 'skip' : 'ko'
          const mark = d.ok === true ? '✓' : d.ok === null ? '○' : '✕'
          return (
            <li key={`${r.file}-${i}`} className={`lfx-res-${cls}`} data-status={r.status}>
              {mark} <code>{r.file}</code> — {d.text}
              {d.errors?.length > 0 && (
                <> {d.errors.map((er, j) => <span key={j} className="lfx-res-err">{er.line ? `Ligne ${er.line} : ` : ''}{er.message}</span>)}</>
              )}
            </li>
          )
        })}
      </ul>
    </section>
  )
}

// Fenêtre modale commune (conflit, suppression). Échap = annuler ; le focus part sur le premier bouton.
export function Modal({ titleId, title, onCancel, children }) {
  const ref = useRef(null)
  useEffect(() => { ref.current?.querySelector('button')?.focus() }, [])
  return (
    <div className="lfx-modal" onMouseDown={(e) => { if (e.target === e.currentTarget) onCancel?.() }}>
      <div
        className="lfx-modal-box" role="dialog" aria-modal="true" aria-labelledby={titleId} ref={ref}
        onKeyDown={(e) => { if (e.key === 'Escape') { e.stopPropagation(); onCancel?.() } }}
      >
        <h4 id={titleId} className="lfx-modal-title">{title}</h4>
        {children}
      </div>
    </div>
  )
}

// « Un effet du même identifiant existe déjà » : un fichier à la fois (§11.7 : renvoi du seul fichier concerné).
export function ConflictDialog({ conflict, usedText, onChoose }) {
  const { result: r } = conflict
  const label = r.existing?.name || r.name || r.id
  return (
    <Modal titleId="lfx-conflict-title" title={`« ${label} » existe déjà`} onCancel={() => onChoose('cancel')}>
      <p>
        Le fichier <code>{r.file}</code> porte l'identifiant <code>{r.id}</code>, déjà utilisé par un de vos effets
        {usedText ? <> (utilisé : {usedText})</> : null}.
      </p>
      <div className="lfx-modal-actions">
        <Button variant="primary" size="sm" onClick={() => onChoose('replace')}>Remplacer</Button>
        <Button variant="secondary" size="sm" onClick={() => onChoose('keep_both')}>Garder les deux</Button>
        <Button variant="secondary" size="sm" onClick={() => onChoose('cancel')}>Annuler</Button>
      </div>
      <p className="lfx-modal-note">« Garder les deux » importe le fichier sous un nouvel identifiant (<code>{r.id}-2</code>…). « Annuler » ignore ce fichier.</p>
    </Modal>
  )
}
