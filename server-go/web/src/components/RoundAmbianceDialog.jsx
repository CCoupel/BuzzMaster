import { useEffect, useRef, useState } from 'react'
import AmbianceSettingsPanel from './AmbianceSettingsPanel'
import { setPartialPath, countOverrides } from '../utils/ambianceSettings'
import { useLightEffects } from '../hooks/useLightEffects'
import { soundUploadMessage } from '../utils/soundState'
import { cueMeta } from '../utils/soundCues'
import './RoundGroup.css'

/**
 * #290 (LOT-4, T21) — fenêtre « Ambiance de la manche » : héberge le composant MUTUALISÉ
 * AmbianceSettingsPanel en mode « override » (tri-états « Hérité / … », valeur globale en italique).
 * Maquette ambiance_settings_panel__round_override. Aucun catalogue (une manche n'en change pas les sons).
 * L'enregistrement transporte `AMBIANCE` dans ROUNDS_UPDATE (structure des manches) — rien n'est écrit dans le global.
 *
 * Props : roundId, roundName, value (Partial courant de la manche), onSave(partial), onCancel(),
 *   base / cues : facultatifs (sinon lus par GET /api/ambiance/settings (`global`) et GET /api/sounds).
 *
 * T27 (LOT-4 Batch 2, révision 3) — colonne « Fichier » : fichier de manche par cue (et décompte) via
 * GET/POST/DELETE /api/rounds/{id}/sounds[/{cue}] (+ /test). Effet IMMÉDIAT, indépendant d'Enregistrer / Annuler (D10) :
 * ces deux boutons ne portent que sur les tri-états et le mode d'éclairage. Sans `roundId`, pas de colonne Fichier.
 */
// `sound.files` est géré côté serveur (effet immédiat, D10) : jamais renvoyé par ROUNDS_UPDATE.
function withoutFiles(partial) {
  if (!partial?.sound || partial.sound.files === undefined) return partial
  const { files: _files, ...sound } = partial.sound // eslint-disable-line no-unused-vars
  const { sound: _s, ...rest } = partial // eslint-disable-line no-unused-vars
  return Object.keys(sound).length ? { ...rest, sound } : rest
}

export default function RoundAmbianceDialog({ roundId = null, roundName, value, base: baseProp = null, cues: cuesProp = null, onSave, onCancel }) {
  const [draft, setDraft] = useState(() => value || {})
  const [base, setBase] = useState(baseProp)
  const [cues, setCues] = useState(cuesProp || [])
  const [loadError, setLoadError] = useState(null)
  const boxRef = useRef(null)
  const closeRef = useRef(null)
  // Fichiers de manche : { [cue]: { path, present, duration_seconds } } ; `ver` = jeton anti-cache de « Écouter ici »
  // (le nom de fichier est fixe, GET ne fournit pas de mtime : on re-jette un jeton à chaque (re)chargement).
  const [files, setFiles] = useState({})
  const [ver, setVer] = useState(() => Date.now())
  const [globalSounds, setGlobalSounds] = useState({})
  const [busyCue, setBusyCue] = useState(null)
  const [notice, setNotice] = useState(null) // { text, type }
  const fileRef = useRef({})

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
      .then(d => {
        if (!alive) return
        setCues(Array.isArray(d?.cues) ? d.cues.map(c => ({ cue: c.cue, type: c.type || '' })) : [])
        const m = {}
        ;(Array.isArray(d?.cues) ? d.cues : []).forEach(c => { m[c.cue] = c })
        ;(Array.isArray(d?.long) ? d.long : []).forEach(l => { m[l.name] = l })
        setGlobalSounds(m)
      })
      .catch(e => { if (alive) setLoadError(e.message) })
    return () => { alive = false }
  }, [cuesProp])

  const loadFiles = async () => {
    if (!roundId) return
    try {
      const res = await fetch(`/api/rounds/${roundId}/sounds`)
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      const d = await res.json()
      const m = {}
      ;(Array.isArray(d?.files) ? d.files : []).forEach(f => { m[f.cue] = f })
      setFiles(m)
      setVer(Date.now())
    } catch (e) {
      setLoadError(e.message)
    }
  }
  useEffect(() => { loadFiles() }, [roundId]) // eslint-disable-line react-hooks/exhaustive-deps

  const errText = async (res, fallback) => {
    try { const b = await res.json(); return b?.message || fallback } catch { return fallback }
  }
  const replaceFile = async (cue, file) => {
    setBusyCue(cue); setNotice(null)
    const fd = new FormData()
    fd.append('file', file)
    try {
      const res = await fetch(`/api/rounds/${roundId}/sounds/${cue}`, { method: 'POST', body: fd })
      if (!res.ok) { setNotice({ text: await errText(res, `Fichier refusé (HTTP ${res.status}).`), type: 'error' }); return }
      let body = {}
      try { body = await res.json() } catch { /* corps vide */ }
      await loadFiles()
      setNotice({ text: soundUploadMessage('Fichier de la manche remplacé', body), type: body.warning ? 'warning' : 'success' })
    } catch (e) {
      setNotice({ text: 'Erreur : ' + e.message, type: 'error' })
    } finally {
      setBusyCue(null)
    }
  }
  const revertFile = async (cue) => {
    setBusyCue(cue); setNotice(null)
    try {
      const res = await fetch(`/api/rounds/${roundId}/sounds/${cue}`, { method: 'DELETE' })
      if (!res.ok && res.status !== 404) { setNotice({ text: await errText(res, `Erreur (HTTP ${res.status}).`), type: 'error' }); return }
      await loadFiles()
      setNotice({ text: 'Fichier global rétabli pour cette manche.', type: 'success' })
    } catch (e) {
      setNotice({ text: 'Erreur : ' + e.message, type: 'error' })
    } finally {
      setBusyCue(null)
    }
  }
  const testFile = async (cue) => {
    setBusyCue(cue); setNotice(null)
    try {
      const res = await fetch(`/api/rounds/${roundId}/sounds/${cue}/test`, { method: 'POST' })
      if (!res.ok) { setNotice({ text: await errText(res, `Test impossible (HTTP ${res.status}).`), type: 'error' }); return }
      let body = {}
      try { body = await res.json() } catch { /* corps vide */ }
      setNotice({ text: body?.played === 'round' ? 'Fichier de la manche joué sur l\'enceinte.' : 'Fichier global joué sur l\'enceinte.', type: 'success' })
    } catch (e) {
      setNotice({ text: 'Erreur : ' + e.message, type: 'error' })
    } finally {
      setBusyCue(null)
    }
  }
  const listenHere = (cue) => {
    try {
      const a = new Audio(`/files/sounds/rounds/${roundId}/${cue}.wav?v=${ver}`)
      const p = a.play()
      if (p && typeof p.catch === 'function') p.catch(() => setNotice({ text: 'Lecture locale impossible.', type: 'error' }))
    } catch {
      setNotice({ text: 'Lecture locale impossible.', type: 'error' })
    }
  }
  const dur = (sec) => (typeof sec === 'number' ? `${sec.toFixed(2).replace('.', ',')} s` : '')

  const renderFile = (cue) => {
    const own = files[cue]
    const g = globalSounds[cue]
    const busy = busyCue === cue
    return (
      <>
        {own ? (
          <span>
            <span className="asp-file-name">{own.path}</span>
            {own.present !== false && own.duration_seconds != null && <span> {dur(own.duration_seconds)}</span>}
            <span className="asp-file-badge">manche</span>
            {own.present === false && <span className="asp-file-missing"> — fichier introuvable, le son global est joué</span>}
          </span>
        ) : (
          <span className="asp-inherit">
            {cue}.wav{g ? ` · ${g.custom ? 'personnalisé' : 'défaut'}${g.duration_seconds != null ? ` · ${dur(g.duration_seconds)}` : ''}` : ''}
          </span>
        )}
        <div className="asp-file-acts">
          <label>
            <input
              type="file"
              accept=".wav,audio/wav,audio/x-wav"
              style={{ display: 'none' }}
              ref={el => { fileRef.current[cue] = el }}
              onChange={e => {
                const f = e.target.files?.[0]
                e.target.value = ''
                if (f) replaceFile(cue, f)
              }}
            />
            <button type="button" disabled={busy} onClick={() => fileRef.current[cue]?.click()}>Remplacer…</button>
          </label>
          {own && <button type="button" className="is-ghost" disabled={busy} onClick={() => revertFile(cue)}>Revenir au fichier global</button>}
          {!(own && own.present === false) && <button type="button" disabled={busy} onClick={() => testFile(cue)}>Tester</button>}
          {own && own.present !== false && <button type="button" onClick={() => listenHere(cue)}>Écouter ici</button>}
        </div>
      </>
    )
  }

  const nFiles = Object.keys(files).length
  const resetAll = async () => {
    if (nFiles > 0) {
      const ok = window.confirm(`Tout remettre à « Hérité » ? Les ${nFiles} fichier${nFiles > 1 ? 's' : ''} propre${nFiles > 1 ? 's' : ''} à la manche seront supprimé${nFiles > 1 ? 's' : ''} (immédiat, définitif).`)
      if (!ok) return
      setNotice(null)
      const failed = []
      for (const cue of Object.keys(files)) {
        try {
          const res = await fetch(`/api/rounds/${roundId}/sounds/${cue}`, { method: 'DELETE' })
          if (!res.ok && res.status !== 404) failed.push(cue)   // 404 = déjà sans fichier : l'objectif est atteint
        } catch {
          failed.push(cue)
        }
      }
      await loadFiles()   // l'état réel du serveur, succès partiel compris
      if (failed.length) {
        const names = failed.map(c => (c === 'decompte' ? 'Décompte' : cueMeta(c).label)).join(', ')
        setNotice({
          text: `Erreur : ${failed.length} fichier${failed.length > 1 ? 's' : ''} de la manche non supprimé${failed.length > 1 ? 's' : ''} (${names}). Les réglages n'ont pas été remis à « Hérité » : réessayez.`,
          type: 'error',
        })
        return
      }
    }
    onChange('*', null)
  }

  // #223 (v11.7.0) — catalogue d'effets lumière + présence du groupe animateur (sélecteurs par moment).
  const { catalog, hostGroups } = useLightEffects({ withHost: true })
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
            {n === 0 && nFiles === 0
              ? 'Hérite tout du réglage global'
              : nFiles === 0
                ? `${n} réglage${n > 1 ? 's' : ''} surchargé${n > 1 ? 's' : ''}`
                : `${n} réglage${n > 1 ? 's' : ''} et ${nFiles} fichier${nFiles > 1 ? 's' : ''} surchargé${n + nFiles > 1 ? 's' : ''}`}
          </span>
        </div>
        <p className="round-amb-hint">
          Par défaut une manche hérite des réglages globaux (page Ambiance). Choisissez une valeur pour la
          surcharger pendant cette manche seulement.
          {roundId && (
            <> Les fichiers (Remplacer…, Revenir au fichier global) s'appliquent <b>immédiatement</b>,
            indépendamment de Enregistrer / Annuler (qui ne portent que sur les réglages et le mode d'éclairage).</>
          )}
        </p>
        {loadError && <p className="round-amb-hint" role="alert">Valeurs globales indisponibles ({loadError}).</p>}
        <AmbianceSettingsPanel
          mode="override"
          value={draft}
          base={base}
          cues={cues}
          onChange={onChange}
          files={files}
          catalog={catalog}
          hostGroups={hostGroups}
          renderFile={roundId ? renderFile : null}
        />
        {notice && (
          <p className={`round-amb-hint round-amb-notice is-${notice.type}`} role="status">{notice.text}</p>
        )}
        <div className="round-modal-actions round-amb-actions">
          <button type="button" className="round-btn" disabled={n === 0 && nFiles === 0} onClick={resetAll}>
            Tout remettre à « Hérité »
          </button>
          <button type="button" className="round-btn" ref={closeRef} onClick={onCancel}>Annuler</button>
          <button type="button" className="round-btn primary" onClick={() => onSave?.(withoutFiles(draft))}>Enregistrer</button>
        </div>
      </div>
    </div>
  )
}
