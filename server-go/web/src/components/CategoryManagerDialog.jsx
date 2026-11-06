import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { CATEGORIES, categoryMeta, firstFreePaletteColor } from '../utils/categoryUtils'
import CategoryColorSwatch from './CategoryColorSwatch'
import './CategoryManagerDialog.css'

const LOCKED_MSG = 'Impossible pendant une partie'
const ACCEPT = '.png,.jpg,.jpeg,.webp'
const MAX_ICON_BYTES = 2 * 1024 * 1024
export const PUSH_GRACE_MS = 1500

/**
 * CategoryManagerDialog — « Gérer les catégories » (#285, maquette
 * admin_category_manager__rename_color_delete §02).
 *
 * Une ligne par catégorie : pastille de couleur (palette), icône (envoi, personnalisées
 * seulement — les intégrées gardent leur emoji), nom (✎ renommage en ligne), compteur de
 * questions, aperçu ambiance, 🗑 (désactivé sur les 8 intégrées).
 * Suppression d'une catégorie utilisée : DELETE sans mode ⇒ 409 `category in use` ⇒ dialogue
 * de confirmation (réaffecter / supprimer aussi) ⇒ DELETE {mode}.
 * `409 game in progress` sur toute action ⇒ « Impossible pendant une partie » et dialogue en
 * lecture seule (contrat http-endpoints.md §Catégories).
 *
 * @param {Object} props
 * @param {Array}  props.categories - liste /api/categories tenue à jour par CATEGORIES_CHANGED (useCategories)
 * @param {() => void} props.onClose
 * @param {() => Promise<void>|void} [props.onChanged] - refetch du parent, rappelé en REPLI seulement (erreur/échec partiel) :
 *   une écriture réussie est répercutée par CATEGORIES_CHANGED, sans rechargement
 * @param {(key: string) => void} [props.onDeleted] - rappelé quand une catégorie est supprimée
 */
export default function CategoryManagerDialog({ categories = [], onClose, onChanged, onDeleted }) {
  const [message, setMessage] = useState('')
  const [messageKind, setMessageKind] = useState('ok') // 'ok' | 'err'
  const [locked, setLocked] = useState(false)
  const [busy, setBusy] = useState(false)
  const [renaming, setRenaming] = useState(null) // { key, value }
  const [newName, setNewName] = useState('')
  const [newFile, setNewFile] = useState(null)
  const [newColor, setNewColor] = useState(() => firstFreePaletteColor(categories))
  const [createError, setCreateError] = useState('')
  const [confirm, setConfirm] = useState(null) // { key, name, questions, rafale, history, mode, ack }

  const source = categories

  // Intégrées dans l'ordre de la table (même si l'API est indisponible), puis personnalisées.
  const rows = useMemo(() => {
    const byKey = new Map(source.map(c => [c.key, c]))
    const builtins = Object.entries(CATEGORIES).map(([key, base]) => {
      const api = byKey.get(key)
      return {
        key,
        name: api?.name ?? base.label,
        color: api ? (api.color || '') : base.color,
        isCustom: false,
        emoji: base.icon,
        imageURL: '',
        questionCount: api?.questionCount ?? 0,
        rafaleCount: api?.rafaleCount ?? 0,
      }
    })
    const customs = source.filter(c => c.isCustom).map(c => ({
      key: c.key,
      name: c.name,
      color: c.color || '',
      isCustom: true,
      emoji: '',
      imageURL: categoryMeta(c.key, source)?.imageURL ?? c.imageURL, // ?v=<TS> après remplacement d'icône
      questionCount: c.questionCount ?? 0,
      rafaleCount: c.rafaleCount ?? 0,
    }))
    return [...builtins, ...customs]
  }, [source])

  // Repli : resynchronise la liste via le parent (GET) quand l'état serveur est incertain (500, échec partiel).
  const resync = useCallback(async () => { await onChanged?.() }, [onChanged])

  // Repli de la diffusion CATEGORIES_CHANGED : après une écriture réussie, la liste (prop `categories`) doit être
  // renouvelée par le push ; sinon (message perdu) on resynchronise par GET après PUSH_GRACE_MS.
  const resyncTimer = useRef(null)
  const onChangedRef = useRef(onChanged)
  useEffect(() => { onChangedRef.current = onChanged }, [onChanged])
  const armResync = () => {
    clearTimeout(resyncTimer.current)
    resyncTimer.current = setTimeout(() => { onChangedRef.current?.() }, PUSH_GRACE_MS)
  }
  // Un nouveau tableau `categories` = le push (ou un refetch) est arrivé : plus besoin du repli.
  useEffect(() => { clearTimeout(resyncTimer.current) }, [categories])
  useEffect(() => () => clearTimeout(resyncTimer.current), [])

  useEffect(() => {
    const onKey = (e) => { if (e.key === 'Escape' && !renaming && !confirm) onClose?.() }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [onClose, renaming, confirm])

  const say = (text, kind = 'ok') => { setMessage(text); setMessageKind(kind) }

  /**
   * Appel d'écriture commun. Retourne { ok, data } ; un 409 `game in progress` verrouille le dialogue,
   * un autre 409 est rendu à l'appelant (`conflict`).
   */
  const request = async (url, opts, { quiet = false } = {}) => {
    setBusy(true)
    try {
      const res = await fetch(url, opts)
      if (res.status === 409) {
        let body = {}
        try { body = await res.json() } catch { body = {} }
        if (body.error === 'game in progress') {
          setLocked(true)
          say(LOCKED_MSG, 'err')
          return { ok: false }
        }
        return { ok: false, conflict: body }
      }
      if (!res.ok) {
        let body = {}
        try { body = await res.json() } catch { body = {} }
        if (!quiet) say(body.error ? `Erreur : ${body.error}` : `Erreur ${res.status}`, 'err')
        return { ok: false, status: res.status, error: body.error }
      }
      let data = null
      try { data = await res.json() } catch { data = null }
      return { ok: true, data }
    } catch {
      say('Erreur réseau', 'err')
      return { ok: false }
    } finally {
      setBusy(false)
    }
  }

  const putJSON = async (row, payload, okText) => {
    const r = await request(`/api/categories/${row.key}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    })
    if (r.ok) { say(okText); armResync() }
    return r.ok
  }

  const changeColor = (row, color) => putJSON(row, { color }, `Couleur de « ${row.name} » mise à jour.`)

  const commitRename = async () => {
    if (!renaming) return
    const row = rows.find(r => r.key === renaming.key)
    const value = renaming.value.trim()
    if (!row || !value || value === row.name) { setRenaming(null); return }
    setRenaming(null)
    await putJSON(row, { name: value }, `« ${row.name} » renommée en « ${value} ».`)
  }

  const uploadIcon = async (row, file) => {
    if (!file) return
    const fd = new FormData()
    fd.append('file', file)
    const r = await request(`/api/categories/${row.key}/icon`, { method: 'POST', body: fd })
    if (r.ok) {
      say(`Icône de « ${row.name} » mise à jour.`)
      armResync()
    }
  }

  // Création (nom + couleur + image) — POST /api/categories (contrat §Catégories). Liste mise à jour par CATEGORIES_CHANGED.
  const createCategory = async () => {
    const name = newName.trim()
    if (!name) { setCreateError('Nom invalide'); return }
    if (!newFile) { setCreateError('Image requise'); return }
    if (newFile.size > MAX_ICON_BYTES) { setCreateError('Image trop lourde (2 Mio max)'); return }
    setCreateError('')
    const fd = new FormData()
    fd.append('name', name)
    fd.append('file', newFile)
    if (newColor) fd.append('color', newColor)
    // Ne PAS définir Content-Type — le navigateur gère le boundary
    const r = await request('/api/categories', { method: 'POST', body: fd }, { quiet: true })
    if (r.ok) {
      say(`Catégorie « ${name} » créée.`)
      setNewName('')
      setNewFile(null)
      setNewColor(firstFreePaletteColor([...source, { color: newColor }]))
      armResync()
    } else if (r.conflict) {
      setCreateError('Cette catégorie existe déjà')
    } else if (r.status === 413) {
      setCreateError('Image trop lourde (2 Mio max)')
    } else if (r.error === 'reserved name') {
      setCreateError('Nom réservé par le système, choisissez-en un autre')
    } else if (r.status) {
      setCreateError('Nom invalide ou image non supportée (PNG, JPG, WebP)')
    } else if (!locked) {
      setCreateError('Erreur réseau')
    }
  }

  const askDelete = async (row) => {
    const r = await request(`/api/categories/${row.key}`, { method: 'DELETE' })
    if (r.ok) {
      say(`Catégorie « ${row.name} » supprimée.`)
      armResync()
      onDeleted?.(row.key)
    } else if (r.conflict?.error === 'category in use') {
      setConfirm({
        key: row.key,
        name: row.name,
        questions: r.conflict.questions ?? 0,
        rafale: r.conflict.rafale ?? 0,
        history: r.conflict.history ?? { events: 0, points: 0 },
        mode: 'reassign',
        ack: false,
      })
    } else if (r.conflict) {
      say(`Erreur : ${r.conflict.error || 'conflit'}`, 'err')
    }
  }

  const needsAck = confirm && (confirm.history.events > 0 || confirm.mode === 'delete')

  const runDelete = async () => {
    const c = confirm
    const r = await request(`/api/categories/${c.key}`, {
      method: 'DELETE',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ mode: c.mode }),
    })
    if (r.ok) {
      setConfirm(null)
      const backup = r.data?.history?.backup
      say(`Catégorie « ${c.name} » supprimée${backup ? ` — historique sauvegardé (${backup})` : ''}.`)
      armResync()
      onDeleted?.(c.key)
    } else if (r.conflict) {
      setConfirm(null)
      say(`Erreur : ${r.conflict.error || 'conflit'}`, 'err')
    } else {
      // 409 game in progress (dialogue verrouillé) ou 500 (éventuellement partiel) : on resynchronise la liste.
      setConfirm(null)
      await resync()
    }
  }

  const ackText = confirm
    ? confirm.mode === 'delete'
      ? `Je comprends que ${confirm.questions} question(s)/carte(s) et ${confirm.rafale} question(s) du réservoir seront supprimées définitivement${confirm.history.events > 0 ? ' et que l\'historique sera réécrit' : ''}.`
      : 'Je comprends que l\'historique sera réécrit.'
    : ''

  const frozen = locked || busy

  return createPortal(
    <div className="cm-overlay" onMouseDown={(e) => { if (e.target === e.currentTarget) onClose?.() }}>
      <div className="cm-dialog" role="dialog" aria-modal="true" aria-labelledby="cm-title">
        <h3 id="cm-title" className="cm-title">
          <span>Gérer les catégories</span>
          <button type="button" className="cm-close" aria-label="Fermer" onClick={onClose}>✕</button>
        </h3>

        {locked && (
          <div className="cm-locked" role="alert">
            <span>{LOCKED_MSG} — lecture seule.</span>
            <button type="button" className="cm-btn" onClick={() => { setLocked(false); setMessage('') }}>Réessayer</button>
          </div>
        )}
        {!locked && message && (
          <div className={`cm-toast${messageKind === 'err' ? ' cm-toast--err' : ''}`} role="status">{message}</div>
        )}

        <div className="cm-scroll">
          <table className="cm-table">
            <thead>
              <tr>
                <th>Couleur</th><th>Icône</th><th>Nom</th><th>Questions</th><th>Ambiance</th><th />
              </tr>
            </thead>
            <tbody>
              {rows.map(row => (
                <tr key={row.key}>
                  <td>
                    <CategoryColorSwatch
                      value={row.color}
                      label={`Couleur de ${row.name}`}
                      disabled={frozen}
                      onChange={(c) => changeColor(row, c)}
                    />
                  </td>
                  <td>
                    {row.isCustom ? (
                      <label className="cm-icon-btn" title="Remplacer l'icône (PNG, JPG, WebP, 2 Mio max)">
                        <img src={row.imageURL} alt="" />
                        <input
                          type="file"
                          accept={ACCEPT}
                          disabled={frozen}
                          aria-label={`Icône de ${row.name}`}
                          onChange={(e) => { uploadIcon(row, e.target.files?.[0]); e.target.value = '' }}
                        />
                      </label>
                    ) : (
                      <span className="cm-emoji" title="Les catégories intégrées gardent leur emoji">{row.emoji} <small>emoji</small></span>
                    )}
                  </td>
                  <td>
                    {renaming?.key === row.key ? (
                      <span className="cm-inline-edit">
                        <input
                          type="text"
                          value={renaming.value}
                          maxLength={50}
                          autoFocus
                          aria-label={`Nouveau nom de ${row.name}`}
                          onChange={(e) => setRenaming({ key: row.key, value: e.target.value })}
                          onKeyDown={(e) => {
                            if (e.key === 'Enter') { e.preventDefault(); commitRename() }
                            if (e.key === 'Escape') { e.stopPropagation(); setRenaming(null) }
                          }}
                        />
                        <button type="button" className="cm-ibtn" onClick={commitRename}>OK</button>
                        <button type="button" className="cm-ibtn" aria-label="Annuler le renommage" onClick={() => setRenaming(null)}>✕</button>
                      </span>
                    ) : (
                      <>
                        <span className="cm-name">{row.name}</span>
                        {!row.isCustom && <span className="cm-lock" title="Catégorie intégrée"> 🔒 intégrée</span>}
                      </>
                    )}
                  </td>
                  <td title={`${row.rafaleCount} dans le réservoir RAFALE`}>{row.questionCount}</td>
                  <td>
                    <span className="cm-hue">
                      <span className="cm-bulb" style={{ color: row.color || '#ffffff' }} />
                      {row.color ? '' : 'blanc'}
                    </span>
                  </td>
                  <td className="cm-actions-cell">
                    <button
                      type="button"
                      className="cm-ibtn"
                      aria-label={`Renommer ${row.name}`}
                      title="Renommer"
                      disabled={frozen}
                      onClick={() => setRenaming({ key: row.key, value: row.name })}
                    >✎</button>
                    <button
                      type="button"
                      className="cm-ibtn cm-ibtn--danger"
                      aria-label={`Supprimer ${row.name}`}
                      title={row.isCustom ? 'Supprimer' : 'Catégorie intégrée — non supprimable'}
                      disabled={!row.isCustom || frozen}
                      onClick={() => askDelete(row)}
                    >🗑</button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        <div className="cm-create">
          <input
            type="text"
            className="cm-create-name"
            placeholder="Nom de la catégorie…"
            aria-label="Nom de la nouvelle catégorie"
            value={newName}
            maxLength={50}
            disabled={frozen}
            onChange={(e) => { setNewName(e.target.value); setCreateError('') }}
            onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); createCategory() } }}
          />
          <label className="cm-create-file">
            <input
              type="file"
              accept={ACCEPT}
              disabled={frozen}
              aria-label="Image de la nouvelle catégorie"
              onChange={(e) => { setNewFile(e.target.files?.[0] || null); setCreateError('') }}
            />
            <span className="cm-btn">{newFile ? `✓ ${newFile.name}` : '📁 Choisir une image…'}</span>
          </label>
          <span className="cm-create-color">
            <CategoryColorSwatch value={newColor} onChange={setNewColor} label="Couleur de la nouvelle catégorie" disabled={frozen} />
            <small>Couleur</small>
          </span>
          <button type="button" className="cm-btn cm-btn--primary" disabled={frozen} onClick={createCategory}>Créer la catégorie</button>
          {createError && <p className="cm-create-error" role="alert">{createError}</p>}
        </div>

        <p className="cm-hint">
          Couleur : cliquer la pastille (elle pilote aussi l'éclairage d'ambiance). Icône : cliquer l'image des
          catégories personnalisées. Renommer ne change que le nom affiché ; la clé technique et les questions
          existantes ne sont pas modifiées.
        </p>

        {confirm && (
          <div className="cm-confirm">
            <h4>
              {`Supprimer « ${confirm.name} » — ${confirm.questions} question(s) et ${confirm.rafale} question(s) du réservoir RAFALE l'utilisent`}
            </h4>
            <label>
              <input
                type="radio"
                name="cm-del-mode"
                checked={confirm.mode === 'reassign'}
                onChange={() => setConfirm({ ...confirm, mode: 'reassign', ack: false })}
              />
              <span>
                <b>Garder les questions, sans catégorie</b>
                <small>Les questions et cartes restent jouables et passent en « Sans catégorie » (réservoir : à recatégoriser pour être tirées).</small>
              </span>
            </label>
            <label>
              <input
                type="radio"
                name="cm-del-mode"
                checked={confirm.mode === 'delete'}
                onChange={() => setConfirm({ ...confirm, mode: 'delete', ack: false })}
              />
              <span>
                <b>Supprimer aussi les questions</b>
                <small>
                  {`Les ${confirm.questions} question(s) et carte(s) MEMOTION de la catégorie (et leurs médias) et les ${confirm.rafale} question(s) du réservoir sont supprimées définitivement.`}
                </small>
              </span>
            </label>
            {confirm.history.events > 0 && (
              <div className="cm-hist">
                📊 {confirm.history.events} événement(s) d'historique totalisent{' '}
                <b>{Number(confirm.history.points).toLocaleString('fr-FR')} points</b> dans cette catégorie : ils seront
                regroupés dans « Sans catégorie » (historique et palmarès réécrits ; sauvegarde automatique du fichier d'historique).
              </div>
            )}
            {needsAck && (
              <label>
                <input
                  type="checkbox"
                  checked={confirm.ack}
                  onChange={(e) => setConfirm({ ...confirm, ack: e.target.checked })}
                />
                <span>{ackText}</span>
              </label>
            )}
            <div className="cm-actions">
              <button type="button" className="cm-btn" onClick={() => setConfirm(null)}>Annuler</button>
              <button
                type="button"
                className="cm-btn cm-btn--danger"
                disabled={(needsAck && !confirm.ack) || frozen}
                onClick={runDelete}
              >Supprimer la catégorie</button>
            </div>
          </div>
        )}
      </div>
    </div>,
    document.body,
  )
}
