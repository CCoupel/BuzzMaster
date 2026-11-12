import { useEffect, useMemo, useState } from 'react'
import Button from './Button'
import EffectBrush, { DEFAULT_BRUSH, brushToAction } from './EffectBrush'
import EffectGrid from './EffectGrid'
import EffectSimulator from './EffectSimulator'
import EffectTextTab from './EffectTextTab'
import EffectTryBar from './EffectTryBar'
import EffectVariablesPanel from './EffectVariablesPanel'
import { Modal } from './LightEffectsImport'
import { useEffectDraft, DEFAULT_PREVIEW_GROUPS } from '../hooks/useEffectDraft'
import { ZONE_LABEL, ZONES } from '../utils/lightEffects'
import {
  addGroup, addStep, addVariable, duplicateStep, highestNumbered, initialNumberedRows, insertStepBefore, locateError, moveStep,
  moveVariable, parseTickSeconds, removeGroup, removeStep, removeVariable, setCell, setVariableKind, starRowUsed, tickSecondsText,
  toggleMoment, updateVariable, withFields, columnLabel,
} from '../utils/effectDocument'
import './LightEffectEditor.css'

// #210 (v11.8.0, LOT-2, F2.2) — éditeur d'effet pas à pas : fiche, grille, variables, pinceau, onglet Texte.
// Maquette : docs/mockup/v11.8.0/ui/ambiance_page__effect_editor.html (révision 4), SANS simulateur ni essai (F2.3).
// Contrat : contracts/light-effects.md §11.1 à §11.4, §11.8, §12. Le serveur est le seul analyseur / validateur / interprète.
//
// Props :
//   id            identifiant de l'effet à ouvrir (GET …/source) — ou de l'effet neuf quand `initialDoc` est donné
//   initialDoc    document d'un nouvel effet ou d'une copie (avec `create`) ; pas de GET
//   create        le premier enregistrement crée le fichier (`PUT … create: true`)
//   zoneGroups    { ambiance, animateur, public } : groupes par zone — rangées « zone réelle » du simulateur et zones de l'essai
//   tryUnavailable(zone) -> '' | raison ; onStopTry() : « Essayer sur la salle » (brouillon, §11.10)
//   onDirtyChange(bool) : modifications non enregistrées (la page garde l'éditeur monté)
//   onClose()     « ← Retour au catalogue » (après la question « modifications non enregistrées » le cas échéant)
//   onSaved(catalogue)  catalogue renvoyé par PUT, à appliquer par le parent
//   onDuplicate(source, doc) facultatif : bouton « Dupliquer » ; `doc` = le document tel qu'il est à l'écran

const MOMENT_LABELS = [['chrono', 'Chronomètre'], ['calm', 'Phases calmes'], ['score', 'Équipe qui marque']]
const ORIGIN = { native: ['natif', ''], override: ['natif modifié', 'over'], user: ['personnalisé', 'user'] }

const downloadHref = (id) => `/api/lighting/effects/${encodeURIComponent(id)}/download`

// Regroupe les erreurs du serveur : par variable (champ), par colonne, et le reste (bandeau).
function placeErrors(errors) {
  const byVar = {}
  const columns = new Set()
  const lines = []
  for (const er of errors) {
    const loc = locateError(er.path)
    if (loc.kind === 'variable') (byVar[loc.index] ||= []).push(er.message)
    else {
      if (loc.kind === 'column') columns.add(loc.column)
      const where = loc.kind === 'column' ? `${columnLabel(loc.column)} : ` : ''
      lines.push(`${where}${er.message}`)
    }
  }
  return { byVar, columns, lines }
}

export default function LightEffectEditor({
  id, initialDoc = null, create = false, zoneGroups = { ambiance: 0, animateur: 0, public: 0 }, tryUnavailable, onStopTry,
  onDirtyChange, onClose, onSaved, onDuplicate = null,
}) {
  // Tailles simulées : 1, 3, 6, 12 + le nombre de groupes de chaque zone réelle qui n'y est pas déjà (8 au plus, §11.3).
  const simRows = useMemo(() => {
    const rows = DEFAULT_PREVIEW_GROUPS.map(n => ({ n, label: `${n} groupe${n > 1 ? 's' : ''}` }))
    const extra = new Map()
    for (const z of ZONES) {
      const n = zoneGroups?.[z] ?? 0
      if (n > 0 && !DEFAULT_PREVIEW_GROUPS.includes(n)) extra.set(n, [...(extra.get(n) || []), ZONE_LABEL[z]])
    }
    for (const [n, names] of extra) rows.push({ n, real: true, label: `${names.length > 1 ? 'Zones' : 'Zone'} ${names.join(', ')} — ${n} groupe${n > 1 ? 's' : ''}` })
    return rows.slice(0, 8)
  }, [zoneGroups?.ambiance, zoneGroups?.animateur, zoneGroups?.public]) // eslint-disable-line react-hooks/exhaustive-deps
  const previewGroups = useMemo(() => simRows.map(r => r.n), [simRows])
  const [moment, setMoment] = useState('')
  const [seek, setSeek] = useState(null)
  const [playingColumn, setPlayingColumn] = useState(null)
  const d = useEffectDraft({ id, initialDoc, create, groups: previewGroups, moment })
  const { doc, text, status, readOnly, gridEditable, source } = d

  const [brush, setBrush] = useState(DEFAULT_BRUSH)
  const [numbered, setNumbered] = useState(5)
  const [tickText, setTickText] = useState('')
  const [dialog, setDialog] = useState(null) // 'leave' | 'removeGroup'
  const [notice, setNotice] = useState('')

  // Nombre de lignes de groupes : max(5, plus grand g<k> utilisé) à l'ouverture ; jamais moins que ce que le document écrit.
  useEffect(() => {
    if (doc) setNumbered(n => Math.max(n, highestNumbered(doc)))
  }, [doc])
  useEffect(() => { if (status === 'ready' && doc) setNumbered(initialNumberedRows(doc)) }, [status]) // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => {
    if (doc && !Number.isNaN(parseTickSeconds(tickText)) && parseTickSeconds(tickText) === doc.tick_ms) return
    if (doc) setTickText(tickSecondsText(doc.tick_ms))
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [doc?.tick_ms])

  useEffect(() => { onDirtyChange?.(!!d.dirty) }, [d.dirty]) // eslint-disable-line react-hooks/exhaustive-deps
  const placed = useMemo(() => placeErrors(d.draft?.from === 'text' ? [] : d.errors), [d.draft, d.errors])
  const textErrors = d.draft?.from === 'text' ? d.errors : []
  const editable = !readOnly
  const gridLocked = readOnly || !gridEditable

  if (status === 'loading') return <section className="efx" aria-busy="true"><p className="efx-hint" role="status">Ouverture de l'effet…</p></section>
  if (status === 'error') {
    return (
      <section className="efx">
        <div className="efx-err" role="alert">{d.loadError}</div>
        <Button variant="secondary" size="sm" onClick={onClose}>← Retour au catalogue</Button>
      </section>
    )
  }

  const edit = (fn) => d.setDoc(fn)
  const onCell = (col, target) => { if (editable && !gridLocked) edit(x => setCell(x, col, target, brushToAction(target, brush))) }
  const back = () => { if (d.dirty) setDialog('leave'); else onClose?.() }
  const switchTab = (to) => {
    const out = d.setTab(to)
    if (!out.ok) {
      setNotice(out.reason === 'invalid' ? 'Corrigez les erreurs du texte avant de revenir à la grille.'
        : out.reason === 'not-editable' ? "Ce texte ne peut pas s'afficher dans la grille : modifiez-le dans l'onglet Texte."
        : 'Vérification du texte en cours : réessayez dans un instant.')
    } else setNotice('')
  }
  const save = async () => {
    const out = await d.save()
    if (out.ok) onSaved?.(out.catalogue)
    return out
  }
  const askRemoveGroup = () => { if (starRowUsed(doc)) setDialog('removeGroup'); else doRemoveGroup() }
  const doRemoveGroup = () => { edit(x => removeGroup(x, numbered)); setNumbered(n => Math.max(0, n - 1)); setDialog(null) }

  const [originLabel, originCls] = ORIGIN[source?.origin] || ['personnalisé', 'user']
  const hasErrors = d.errors.length > 0
  const converted = d.converted

  return (
    <section className="efx" aria-label="Éditeur d'effet" data-testid="effect-editor">
      <div className="efx-head">
        <Button variant="secondary" size="sm" onClick={back}>← Retour au catalogue</Button>
        <h3 className="efx-title">{doc.name || doc.id}</h3>
        <span className={`efx-b ${originCls}`}>{originLabel}</span>
        {d.dirty && <span className="efx-b dirty" role="status">modifications non enregistrées</span>}
        <span className="efx-grow" />
        {d.persisted && <a className="btn btn-secondary btn-sm" href={downloadHref(doc.id)} download={`${doc.id}.yaml`}>⭳ Télécharger</a>}
        {onDuplicate && <Button variant="secondary" size="sm" onClick={() => onDuplicate(source, doc)}>⧉ Dupliquer</Button>}
        {editable && <Button variant="primary" size="sm" disabled={!d.canSave} loading={d.saving} onClick={save}>Enregistrer</Button>}
      </div>

      {readOnly && (
        <div className="efx-info" role="status">
          <b>« {doc.name} » est un effet livré avec BuzzControl : il ne se modifie pas.</b> Dupliquez-le pour en faire le vôtre.
          {onDuplicate && <> <Button variant="secondary" size="sm" onClick={() => onDuplicate(source, doc)}>⧉ Dupliquer</Button></>}
        </div>
      )}
      {d.saveError && <div className="efx-err" role="alert">{d.saveError}</div>}

      <div className="efx-form">
        <label className="efx-wide">Nom
          <input value={doc.name} readOnly={!editable} onChange={(e) => edit(x => withFields(x, { name: e.target.value }))} />
        </label>
        <label>Identifiant
          <input value={doc.id} readOnly title="Choisi à la création. Pour en changer : Dupliquer." />
        </label>
        <label>Durée d'un tick (secondes)
          <input
            value={tickText} readOnly={!editable} inputMode="decimal"
            onChange={(e) => {
              setTickText(e.target.value)
              const ms = parseTickSeconds(e.target.value)
              if (!Number.isNaN(ms)) edit(x => withFields(x, { tick_ms: ms }))
            }}
          />
        </label>
        <label className="efx-full">Description
          <input value={doc.description || ''} readOnly={!editable} onChange={(e) => edit(x => withFields(x, { description: e.target.value }))} />
        </label>
        <div className="efx-wide efx-field">Proposé pour les moments
          <span className="efx-chips" role="group" aria-label="Proposé pour les moments">
            {MOMENT_LABELS.map(([key, label]) => (
              <button
                key={key} type="button" className={`efx-chip${doc.moments.includes(key) ? ' is-on' : ''}`} aria-pressed={doc.moments.includes(key)}
                disabled={!editable} onClick={() => edit(x => toggleMoment(x, key))}
              >{label}</button>
            ))}
          </span>
        </div>
        <label>Groupes demandés dans la zone (minimum)
          <input
            type="number" min="0" max="12" value={doc.min_groups ?? 0} readOnly={!editable}
            onChange={(e) => edit(x => withFields(x, { min_groups: parseInt(e.target.value, 10) || 0 }))}
          />
        </label>
        <div className="efx-field">À la fin des pas
          <span className="efx-chips" role="group" aria-label="À la fin des pas">
            <button type="button" className={`efx-chip${doc.loop ? ' is-on' : ''}`} aria-pressed={!!doc.loop} disabled={!editable} onClick={() => edit(x => withFields(x, { loop: true }))}>Recommencer (boucle)</button>
            <button type="button" className={`efx-chip${!doc.loop ? ' is-on' : ''}`} aria-pressed={!doc.loop} disabled={!editable} onClick={() => edit(x => withFields(x, { loop: false }))}>S'arrêter</button>
          </span>
        </div>
      </div>

      <div className="efx-tabs" role="tablist" aria-label="Édition de l'effet">
        <button type="button" role="tab" aria-selected={d.tab === 'grid'} className={`efx-tab${d.tab === 'grid' ? ' is-active' : ''}`} onClick={() => switchTab('grid')}>Grille</button>
        <button type="button" role="tab" aria-selected={d.tab === 'text'} className={`efx-tab${d.tab === 'text' ? ' is-active' : ''}`} onClick={() => switchTab('text')}>Texte</button>
      </div>
      {notice && <div className="efx-warn" role="alert">{notice}</div>}

      {d.tab === 'grid' && (
        <div data-testid="effect-grid-tab">
          {converted.length > 0 && (
            <div className="efx-info" role="status">
              <b>Ce fichier utilisait des voisins ({[...new Set(converted.map(c => c.from))].map(f => <code key={f}>{f} </code>)}).</b>{' '}
              Dans la grille, ils sont remplacés par les variables {converted.map((c, i) => <span key={c.to}>{i ? ', ' : ''}<code>{c.to}</code> (= <code>{c.from}</code>)</span>)} :
              le rendu est le même. Le fichier ne sera réécrit qu'à l'enregistrement.
            </div>
          )}
          {!gridEditable && !readOnly && (
            <div className="efx-info" role="status">
              <b>La grille est en lecture seule pour cet effet</b>
              {d.blocked.length > 0 && <> (cibles en cause : {d.blocked.map(b => <code key={b}>{b} </code>)})</>} : modifiez-le dans l'onglet Texte.
            </div>
          )}
          {d.hasComments && editable && (
            <div className="efx-warn">
              <b>Ce fichier contient des commentaires.</b> Ils seront perdus si vous modifiez l'effet dans la grille. Pour les garder, modifiez-le dans l'onglet Texte.
            </div>
          )}
          {d.customOrder && editable && (
            <div className="efx-warn">
              <b>Dans ce fichier, certaines actions ne sont pas dans l'ordre de la grille</b> (fond, groupes, puis variables). Un pas que vous modifiez dans la grille sera réordonné.
            </div>
          )}

          <div className="efx-cols">
            <div>
              <EffectVariablesPanel
                variables={doc.variables} errors={placed.byVar} previews={d.previews} disabled={gridLocked}
                onAdd={() => edit(addVariable)} onKind={(i, k) => edit(x => setVariableKind(x, i, k))}
                onPatch={(i, p) => edit(x => updateVariable(x, i, p))} onMove={(i, delta) => edit(x => moveVariable(x, i, delta))}
                onRemove={(i) => edit(x => removeVariable(x, i))}
              />
              {editable && !gridLocked && <EffectBrush brush={brush} onChange={setBrush} />}
            </div>
            <div>
              <EffectGrid
                doc={doc} numbered={numbered} readOnly={gridLocked} onCell={onCell} errorColumns={placed.columns}
                onAddStep={() => edit(addStep)} onDuplicateStep={(c) => edit(x => duplicateStep(x, c))}
                onInsertStep={(c) => edit(x => insertStepBefore(x, c))} onMoveStep={(c, delta) => edit(x => moveStep(x, c, delta))}
                onRemoveStep={(c) => edit(x => removeStep(x, c))}
                onGoStep={(c) => setSeek({ column: c, nonce: Date.now() })} playingColumn={playingColumn}
                onAddGroup={() => { edit(x => addGroup(x, numbered)); setNumbered(n => n + 1) }} onRemoveGroup={askRemoveGroup}
              />
              <p className="efx-hint">
                <b>Colonnes</b> : <b>PRE</b> (joué une fois au début), les <b>pas</b> de la boucle (un par tick), <b>DERNIER</b> (fin de chaque tour),
                <b> POST</b> (fin du moment). Menu d'un pas (▾ sur son en-tête) : dupliquer, insérer avant, déplacer, supprimer.<br />
                <b>Lignes</b> : les variables en haut, puis <code>g0</code> = toute la zone, puis les groupes ; <b>la dernière ligne est <code>g*</code></b>, le dernier
                groupe de la zone quel que soit son nombre de groupes. <b>Qui repeint qui</b> : dans un pas, le fond (<code>g0</code>) est posé d'abord, puis les groupes
                numérotés, puis <code>g*</code>, puis les variables.
              </p>
              {hasErrors && d.draft?.from !== 'text' && (
                <div className="efx-err" role="alert" data-testid="effect-errors">
                  {placed.lines.map((l, i) => <div key={i}>{l}</div>)}
                  {Object.entries(placed.byVar).map(([i, msgs]) => <div key={`v${i}`}><b>{doc.variables[i]?.name || `variable ${Number(i) + 1}`}</b> : {msgs.join(' ; ')}</div>)}
                </div>
              )}
              {d.pending && <p className="efx-hint" role="status">Vérification par le serveur…</p>}
            </div>
          </div>
        </div>
      )}

      {d.tab === 'text' && (
        <EffectTextTab
          text={text} onChange={d.setText} readOnly={!editable} errors={textErrors} pending={d.pending} hasComments={d.hasComments}
        />
      )}

      <EffectSimulator
        previews={d.lastValidPreviews} rows={simRows} minGroups={doc.min_groups || 0} tickMs={doc.tick_ms || 1000}
        moments={doc.moments} moment={moment && doc.moments.includes(moment) ? moment : (doc.moments[0] || '')} onMoment={setMoment}
        seek={seek} onColumn={setPlayingColumn} stale={d.errors.length > 0}
      />
      <EffectTryBar
        body={d.lastSource === 'text' ? { yaml: text } : { effect: doc }} valid={!!d.draft?.ok && !d.pending}
        zoneGroups={zoneGroups} minGroups={doc.min_groups || 0} tryUnavailable={tryUnavailable} onStopTry={onStopTry}
        seconds={doc.moments.length === 1 && doc.moments[0] === 'score' ? 5 : 10}
      />

      {dialog === 'leave' && (
        <Modal titleId="efx-leave-title" title="Modifications non enregistrées" onCancel={() => setDialog(null)}>
          <p>« {doc.name} » a été modifié.</p>
          <div className="efx-modal-actions">
            <Button variant="primary" size="sm" disabled={!d.canSave} onClick={async () => { const out = await save(); if (out.ok) { setDialog(null); onClose?.() } else setDialog(null) }}>Enregistrer</Button>
            <Button variant="secondary" size="sm" onClick={() => { setDialog(null); onClose?.() }}>Abandonner les modifications</Button>
            <Button variant="secondary" size="sm" onClick={() => setDialog(null)}>Rester</Button>
          </div>
        </Modal>
      )}
      {dialog === 'removeGroup' && (
        <Modal titleId="efx-rmg-title" title="Retirer un groupe ?" onCancel={() => setDialog(null)}>
          <p>La ligne <code>g*</code> n'est pas vide : ses actions seront supprimées, et la ligne <code>g{numbered}</code> deviendra <code>g*</code>.</p>
          <div className="efx-modal-actions">
            <Button variant="danger" size="sm" onClick={doRemoveGroup}>Retirer le groupe</Button>
            <Button variant="secondary" size="sm" onClick={() => setDialog(null)}>Annuler</Button>
          </div>
        </Modal>
      )}
    </section>
  )
}
