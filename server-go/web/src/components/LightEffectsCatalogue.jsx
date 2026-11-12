import { useEffect, useMemo, useRef, useState } from 'react'
import Button from './Button'
import LightEffectPreview from './LightEffectPreview'
import { ImportResults, ConflictDialog, Modal, postImport, requestDelete } from './LightEffectsImport'
import NameIdDialog from './EffectNameDialog'
import { copyId } from '../utils/effectDocument'
import {
  MOMENTS, ZONE_LABEL, TRY_SECONDS, TRY_EVENT, formatSpeed, globalEffect, globalSpeed, visibleZones, readTryZone, saveTryZone,
} from '../utils/lightEffects'
import './LightEffectsCatalogue.css'

// #299 (v11.7.0, LOT-3) — catalogue des effets (sous-onglet Effets de Ambiance › Lumière).
// Maquette : docs/mockup/v11.7.0/ui/ambiance_page__effects_catalogue_zones.html. Contrat : contracts/light-effects.md
// §7.1 (catalogue, `steps`, `tick_ms`), §7.2 (rechargement), §7.3 (aperçu : `tick`, `step`, `duration_ms`, `chrono_s`), §7.4 (essai).
//
// Props :
//   catalog        GET /api/lighting/effects normalisé (useLightEffects)
//   lighting       section `lighting` de config.json (« utilisé : zone, moment » d'après les réglages GLOBAUX)
//   zoneGroups     { ambiance, animateur, public } groupes non vides par zone
//   onReload()     POST /api/lighting/effects/reload ; onTry(id, zone) -> {ok, duration_ms?} ; onStopTry()
//   onCreate({id, name}) : « Nouvel effet » validé ; onDuplicate(effect, {id, name}) : « Dupliquer » validé (à défaut, onCreate reçoit
//     {id, name, from}) ; onEdit(id) : « Modifier » / « Ouvrir » (#210). Sans ces rappels, les boutons correspondants ne sont pas affichés
//   onCatalogue(catalogue) : catalogue renvoyé par l'import (§11.7) ou la suppression (§11.5), appliqué par le parent
//   (à défaut : onReload()). L'import et la suppression sont des appels `fetch` faits ici.
//   tryUnavailable(zone) -> '' | raison (éclairage désactivé, pont, forçage ON/OFF/Flash, question en cours)
//   onManageGroups() ; focus : {id, n} carte à mettre en vue (lien « Voir l'aperçu » des réglages)
//
// Un seul aperçu à la fois ; un seul essai à la fois ; un essai et un test (bouton « Tester » des réglages) ne tournent
// jamais ensemble : le dernier lancé remplace l'autre (événement fenêtre TRY_EVENT).

const MOMENT_LABEL = { chrono: 'Chronomètre', calm: 'Phases calmes', score: 'Équipe qui marque' }
const MOMENT_SHORT = { chrono: 'chronomètre', calm: 'phases calmes', score: 'équipe qui marque' }
const ORIGIN_BADGE = { native: 'natif', user: 'personnalisé', override: 'natif modifié' }

// « utilisé : Animateur, chronomètre (×1,5) » d'après les réglages globaux ; les manches ne sont pas listées.
export function usedBy(lighting, zoneGroups, id) {
  const out = []
  for (const zone of visibleZones(zoneGroups)) {
    for (const m of MOMENTS) {
      if (globalEffect(lighting, zone, m.key) !== id) continue
      const sp = globalSpeed(lighting, zone, m.key)
      out.push(`${ZONE_LABEL[zone]}, ${MOMENT_SHORT[m.key]}${sp !== 1 ? ` (${formatSpeed(sp)})` : ''}`)
    }
  }
  return out
}

// Applicabilité d'un effet à une zone (table `applicable` du §7.1 ; catalogue antérieur : booléen).
const applicableTo = (e, zone) => (e.applicable && typeof e.applicable === 'object' ? e.applicable[zone] !== false : e.applicable !== false)
const downloadHref = (id) => `/api/lighting/effects/${encodeURIComponent(id)}/download`

const baseName = (file) => (typeof file === 'string' ? file.split('/').pop() : '')
const tickText = (ms) => `${String(ms / 1000).replace('.', ',')} s`

export default function LightEffectsCatalogue({
  catalog, lighting = {}, zoneGroups = { ambiance: 0, animateur: 0, public: 0 }, onReload, onTry, onStopTry,
  tryUnavailable = () => '', onManageGroups = null, focus = null, onCatalogue = null, onCreate = null, onEdit = null, onDuplicate = null,
}) {
  const [previewId, setPreviewId] = useState(null)
  const [trying, setTrying] = useState(null) // { id, durationMs }
  const [starting, setStarting] = useState(null)
  const [reloading, setReloading] = useState(false)
  const timer = useRef(null)
  const fileInput = useRef(null)
  const [tryZone, setTryZone] = useState(readTryZone)
  const [importing, setImporting] = useState(false)
  const [importRows, setImportRows] = useState([])
  const [importError, setImportError] = useState('')
  const [pending, setPending] = useState(null) // { files, queue: [index…] } : fichiers en conflit, un à la fois
  const [dragging, setDragging] = useState(false)
  const [confirm, setConfirm] = useState(null) // effet à supprimer / restaurer
  const [deleteError, setDeleteError] = useState('')
  const [deleting, setDeleting] = useState(false)
  const [nameDialog, setNameDialog] = useState(null) // { kind: 'create' } | { kind: 'duplicate', effect }
  // Version du catalogue : change à chaque (re)chargement ; invalide les aperçus en cache (trames périmées).
  const versionRef = useRef(0)
  const version = useMemo(() => { versionRef.current += 1; return versionRef.current }, [catalog])

  // Un test des réglages (bouton « Tester ») qui démarre remplace l'essai du catalogue.
  useEffect(() => {
    const onOther = (e) => { if (e.detail?.source !== 'catalogue') { clearTimeout(timer.current); setTrying(null) } }
    window.addEventListener(TRY_EVENT, onOther)
    return () => { window.removeEventListener(TRY_EVENT, onOther); clearTimeout(timer.current) }
  }, [])

  useEffect(() => {
    if (!focus?.id) return
    document.getElementById(`fx-card-${focus.id}`)?.scrollIntoView?.({ block: 'center' })
  }, [focus])

  const zones = visibleZones(zoneGroups)
  const zone = zones.includes(tryZone) ? tryZone : 'ambiance'
  const chooseZone = (z) => { setTryZone(z); saveTryZone(z) }

  // « utilisé : … » sur TOUTES les zones (un réglage qui cite l'effet compte même si la zone n'a plus de groupe).
  const usedEverywhere = (id) => usedBy(lighting, { ambiance: 1, animateur: 1, public: 1 }, id)
  const applyCatalogue = (c) => { if (c) onCatalogue?.(c); else onReload?.() }

  const importFiles = async (list) => {
    const files = Array.from(list || [])
    if (!files.length || importing) return
    setImporting(true); setImportError(''); setPending(null)
    try {
      const out = await postImport(files, 'ask')
      if (!out?.ok) { setImportRows([]); setImportError(out?.message || 'Import impossible.'); return }
      applyCatalogue(out.catalogue)
      setImportRows(out.results)
      const queue = out.results.flatMap((r, i) => (r.status === 'conflict' ? [i] : []))
      if (queue.length) setPending({ files, queue })
    } finally { setImporting(false) }
  }

  // Réponse à la fenêtre de conflit : le seul fichier concerné est renvoyé avec `replace` ou `keep_both` (§11.7).
  const resolveConflict = async (choice) => {
    const idx = pending.queue[0]
    const file = pending.files[idx]
    let row
    if (choice === 'cancel') {
      row = { ...importRows[idx], status: 'cancelled' }
    } else {
      const out = await postImport([file], choice)
      if (out?.ok) applyCatalogue(out.catalogue)
      row = out?.ok && out.results[0]
        ? { ...out.results[0], file: importRows[idx].file }
        : { file: importRows[idx].file, status: 'invalid', errors: [{ message: out?.message || 'Import impossible.' }] }
    }
    setImportRows(rows => rows.map((r, i) => (i === idx ? row : r)))
    const queue = pending.queue.slice(1)
    setPending(queue.length ? { ...pending, queue } : null)
  }

  const doDelete = async () => {
    setDeleting(true); setDeleteError('')
    try {
      const out = await requestDelete(confirm.id)
      if (out?.ok) { applyCatalogue(out.catalogue); setConfirm(null) }
      else setDeleteError(out?.message || 'Suppression impossible.')
    } finally { setDeleting(false) }
  }

  const toggleTry = async (e) => {
    if (trying?.id === e.id) {
      clearTimeout(timer.current)
      setTrying(null)
      onStopTry?.()
      return
    }
    setStarting(e.id)
    try {
      // Essai sur la zone choisie, vitesse ×1, durée choisie par le serveur (§7.4 :
      // 5 s pour un effet qui ne déclare que « score », 10 s sinon) ; la réponse donne `duration_ms`.
      const res = await onTry(e.id, zone)
      if (res?.ok) {
        window.dispatchEvent(new CustomEvent(TRY_EVENT, { detail: { source: 'catalogue' } }))
        const durationMs = Number.isFinite(res.duration_ms) ? res.duration_ms : TRY_SECONDS * 1000
        clearTimeout(timer.current)
        setTrying({ id: e.id, durationMs })
        timer.current = setTimeout(() => setTrying(null), durationMs)
      }
    } finally { setStarting(null) }
  }

  const reload = async () => {
    setReloading(true)
    try { await onReload?.() } finally { setReloading(false) }
  }

  const errors = catalog?.errors || []
  const effects = catalog?.effects || []
  const nAmb = zoneGroups.ambiance ?? 0

  return (
    <section
      className={`lfx${dragging ? ' is-drop' : ''}`} aria-labelledby="lfx-title" data-testid="catalogue-dropzone"
      onDragOver={(e) => { e.preventDefault(); setDragging(true) }}
      onDragLeave={(e) => { if (!e.currentTarget.contains(e.relatedTarget)) setDragging(false) }}
      onDrop={(e) => { e.preventDefault(); setDragging(false); importFiles(e.dataTransfer?.files) }}
    >
      <div className="lfx-head">
        <h3 id="lfx-title" className="lfx-title">Catalogue des effets</h3>
        <span className="lfx-spacer" />
        {onCreate && <Button variant="primary" size="sm" onClick={() => setNameDialog({ kind: 'create' })}>＋ Nouvel effet</Button>}
        <input
          ref={fileInput} type="file" accept=".yaml,.yml" multiple hidden data-testid="lfx-import-input"
          onChange={(e) => { importFiles(e.target.files); e.target.value = '' }}
        />
        <Button variant="secondary" size="sm" onClick={() => fileInput.current?.click()} loading={importing}>⭱ Importer…</Button>
        <Button variant="secondary" size="sm" onClick={reload} loading={reloading}>↻ Recharger les fichiers</Button>
      </div>
      <p className="ambiance-hint">
        Un effet est une liste de pas qui donnent, tick après tick, une couleur à des groupes (<code>g1</code>, <code>g2</code>… ;
        <code>g0</code> = toute la zone). Un même effet peut être joué sur la zone Ambiance, Animateur ou Public.
        {onCreate ? <> Créez un effet avec « Nouvel effet », ou partez d'un effet existant avec « Dupliquer ».</> : null}
        {' '}Chaque effet est aussi un fichier texte du dossier <code>{catalog?.dir || 'data/files/effects'}</code> du serveur, que l'on peut
        télécharger, importer (bouton « Importer… » ou glisser-déposer sur le catalogue) ou modifier à la main : il apparaît ici en quelques
        secondes.
      </p>
      {dragging && <div className="lfx-drop" role="note">Déposez ici des fichiers d'effet (<code>.yaml</code>)</div>}

      {importError && <div className="lfx-err" role="alert">{importError}</div>}
      <ImportResults rows={importRows} onClose={() => { setImportRows([]); setPending(null) }} />

      {errors.length > 0 && (
        <div className="lfx-err" role="alert">
          <b>{errors.length === 1 ? "1 fichier n'a pas pu être chargé." : `${errors.length} fichiers n'ont pas pu être chargés.`}</b>{' '}
          Les autres effets ne sont pas touchés.
          <ul>
            {errors.map((er, i) => (
              <li key={`${er.file}-${i}`}>
                <code>{er.file}</code>{er.line ? `, ligne ${er.line}` : ''} : {er.message}
              </li>
            ))}
          </ul>
        </div>
      )}

      {effects.length === 0 && <p className="ambiance-hint">Aucun effet disponible.</p>}

      <div className="lfx-grid">
        {effects.map((e) => {
          const used = usedBy(lighting, zoneGroups, e.id)
          const applicable = e.applicable && typeof e.applicable === 'object' ? e.applicable.ambiance !== false : e.applicable !== false
          const min = e.min_groups || 0
          const isTrying = trying?.id === e.id
          const zoneOk = applicableTo(e, zone) && (min <= 0 || (zoneGroups[zone] ?? 0) >= min) // le serveur juge, l'interface double la règle des groupes
          const unavailable = isTrying ? ''
            : !zoneOk ? `Demande ${min} groupe${min > 1 ? 's' : ''} ; la zone ${ZONE_LABEL[zone]} en a ${zoneGroups[zone] ?? 0}`
            : tryUnavailable(zone)
          const simGroups = applicable ? (nAmb > 0 ? nAmb : 6) : Math.min(12, Math.max(6, min))
          const moment = e.moments?.includes('chrono') ? 'chrono' : (e.moments?.[0] || 'chrono')
          const playing = previewId === e.id
          return (
            <article key={e.id} id={`fx-card-${e.id}`} className={`lfx-fx${applicable ? '' : ' is-na'}`} data-testid={`fx-${e.id}`}>
              <h4 className="lfx-fx-title">
                {e.name}
                <span className={`lfx-b lfx-b-${e.origin}`}>{ORIGIN_BADGE[e.origin] || e.origin}</span>
                {(e.moments || []).map(m => <span key={m} className="lfx-m">{MOMENT_LABEL[m] || m}</span>)}
                {used.length > 0 && <span className="lfx-b lfx-b-use">utilisé : {used.join(' · ')}</span>}
              </h4>
              {e.description && <p className="lfx-desc">{e.description}</p>}
              {!applicable && (
                <div className="lfx-warn">
                  Demande {min} groupe{min > 1 ? 's' : ''} ; la zone Ambiance en a {nAmb}.{' '}
                  {onManageGroups && <button type="button" className="lfx-link" onClick={onManageGroups}>Gérer les groupes</button>}
                </div>
              )}
              <LightEffectPreview
                id={e.id} groups={simGroups} moment={moment} version={version}
                playing={playing} onUnavailable={() => setPreviewId(null)}
              />
              {!applicable && playing && <p className="lfx-sim">simulation avec {simGroups} groupes</p>}
              <div className="lfx-actions">
                <Button variant="secondary" size="sm" onClick={() => setPreviewId(playing ? null : e.id)}>
                  {playing ? '■ Arrêter' : '▶ Aperçu'}
                </Button>
                <span className="lfx-tryz">
                  <Button
                    variant="secondary" size="sm"
                    disabled={!!unavailable || starting === e.id}
                    title={unavailable || `Joue l'effet sur la zone ${ZONE_LABEL[zone]}, à la vitesse ×1, puis l'éclairage revient à son état normal`}
                    onClick={() => toggleTry(e)}
                  >
                    {isTrying ? "■ Arrêter l'essai" : '💡 Essayer sur la salle'}
                  </Button>
                  <select
                    className="lfx-zone" aria-label="Zone où jouer l'essai" title="Zone où jouer l'essai"
                    value={zone} disabled={isTrying} onChange={(ev) => chooseZone(ev.target.value)}
                  >
                    {zones.map(z => <option key={z} value={z}>{ZONE_LABEL[z]}</option>)}
                  </select>
                </span>
                <span className="lfx-sep" />
                {onEdit && (e.origin === 'user'
                  ? <Button variant="primary" size="sm" onClick={() => onEdit(e.id)}>✎ Modifier</Button>
                  : <Button variant="secondary" size="sm" onClick={() => onEdit(e.id)}>👁 Ouvrir</Button>)}
                {(onDuplicate || onCreate) && <Button variant="secondary" size="sm" onClick={() => setNameDialog({ kind: 'duplicate', effect: e })}>⧉ Dupliquer</Button>}
                <a className="btn btn-secondary btn-sm lfx-dl" href={downloadHref(e.id)} download={`${e.id}.yaml`}>⭳ Télécharger</a>
                {e.origin === 'user' && (
                  <Button variant="danger" size="sm" onClick={() => { setDeleteError(''); setConfirm(e) }}>🗑 Supprimer</Button>
                )}
                {e.origin === 'override' && (
                  <Button variant="danger" size="sm" onClick={() => { setDeleteError(''); setConfirm(e) }}>↺ Revenir à l'effet d'origine</Button>
                )}
              </div>
              <div className="lfx-file">
                Fichier :{' '}
                {e.origin === 'override'
                  ? <><code>{baseName(e.file)}</code> remplace <code>natifs/{baseName(e.file)}</code>. Supprimez-le pour revenir à l'effet d'origine.</>
                  : <code>{e.file}</code>}
                {e.steps > 0 && ` · ${e.steps} pas`}
                {e.tick_ms > 0 && ` · tick de ${tickText(e.tick_ms)}`}
                {min > 0 && ` · demande ${min} groupe${min > 1 ? 's' : ''} dans la zone`}
              </div>
            </article>
          )
        })}
      </div>

      {pending && (
        <ConflictDialog
          conflict={{ result: importRows[pending.queue[0]] }}
          usedText={usedEverywhere(importRows[pending.queue[0]]?.id).join(' · ')}
          onChoose={resolveConflict}
        />
      )}
      {nameDialog && (
        <NameIdDialog
          title={nameDialog.kind === 'create' ? 'Nouvel effet' : `Dupliquer « ${nameDialog.effect.name} »`}
          initialName={nameDialog.kind === 'create' ? '' : `${nameDialog.effect.name} (copie)`}
          initialId={nameDialog.kind === 'create' ? '' : copyId(nameDialog.effect.id)}
          takenIds={effects.map(x => x.id)} confirmLabel="Créer"
          onCancel={() => setNameDialog(null)}
          onConfirm={(info) => { const d = nameDialog; setNameDialog(null); if (d.kind === 'create') onCreate?.(info)
            else if (onDuplicate) onDuplicate(d.effect, info)
            else onCreate?.({ ...info, from: d.effect.id }) }}
        />
      )}
      {confirm && (() => {
        const restore = confirm.origin === 'override'
        const used = usedEverywhere(confirm.id)
        return (
          <Modal
            titleId="lfx-confirm-title" onCancel={() => setConfirm(null)}
            title={restore ? `Revenir au « ${confirm.name} » d'origine ?` : `Supprimer « ${confirm.name} » ?`}
          >
            {restore ? (
              <p>
                Le fichier <code>{baseName(confirm.file)}</code>, qui remplace l'effet livré, est supprimé du serveur. L'effet livré
                avec BuzzControl reprend aussitôt, partout où « {confirm.name} » est utilisé.
              </p>
            ) : (
              <p>
                {used.length > 0 && (
                  <>Cet effet est <b>utilisé : {used.join(' · ')}</b>. Après la suppression, ce moment n'aura plus d'effet
                  (les réglages des manches qui le citent non plus). </>
                )}
                Le fichier <code>{confirm.file}</code> est supprimé du serveur : téléchargez-le d'abord pour le garder.
              </p>
            )}
            {deleteError && <p className="lfx-modal-err" role="alert">{deleteError}</p>}
            <div className="lfx-modal-actions">
              <Button variant="danger" size="sm" onClick={doDelete} loading={deleting}>{restore ? "Revenir à l'effet d'origine" : 'Supprimer'}</Button>
              <a className="btn btn-secondary btn-sm lfx-dl" href={downloadHref(confirm.id)} download={`${confirm.id}.yaml`}>⭳ Télécharger</a>
              <Button variant="secondary" size="sm" onClick={() => setConfirm(null)}>Annuler</Button>
            </div>
          </Modal>
        )
      })()}
    </section>
  )
}
