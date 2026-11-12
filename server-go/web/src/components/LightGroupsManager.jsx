import { useState, useMemo, useEffect } from 'react'
import Button from './Button'
import {
  ZONES, ZONE_LABEL, ZONE_ICON, MAX_GROUPS_PER_ZONE, EMPTY_DRAFT, TEST_STEPS, testStepAt,
  draftFromConfig, draftToPayload, draftsEqual, groupCount, pruneToGeneral,
  unassignedLights, locateLight, moveLight, addGroup, removeGroup, moveGroup,
  validateDraft, summarize, groupName, testRef, orderChanged,
} from '../utils/lightGroups'
import './LightGroupsManager.css'

// #225 (v11.7.0, LOT-1) — sous-onglet « Groupes » de Lumière : rassemble les ampoules de l'éclairage général en
// groupes, dans l'une des trois zones (Ambiance, Animateur, Public ; révision 4).
// Maquette : docs/mockup/v11.7.0/ui/ambiance_page__light_groups_zones.html.
// Contrat : contracts/light-effects.md §2.1, §2.4, §2.5.
//
// Props :
//   generalLights  noms des ampoules de rôle « general », dans l'ordre de la liste des ampoules
//   groups         `lighting.groups` enregistré (tableau, éventuellement absent)
//   enabled        éclairage d'ambiance activé (l'essai est refusé sinon : 409)
//   testUnavailableReason  raison d'indisponibilité de l'essai (pont injoignable…), '' si disponible
//   onSave(payload) -> Promise<{ok, message?}>
//   onTest(n | 'host<k>' | 'public<k>') -> Promise<{ok, duration_ms?, message?}>   essai en séquence (6 pas d'1 s)
//   onStopTest() -> Promise<{ok, message?}>                                        POST …/groups/test/stop
//   onChooseEffects(zone)  lien « Choisir les effets de la zone » (null : inactif)
//   onNotify({message,type}) toast de la page
//
// Glisser-déposer natif (dataTransfer), comme les fonds d'écran et les manches ; une ampoule peut passer d'un groupe à
// un autre, d'une zone à l'autre ; un groupe se réordonne dans sa zone. Alternative clavier : « Déplacer vers… » par
// ampoule, « Monter » / « Descendre » par groupe.

const DND_TYPE = 'application/x-buzz-lightgroups'

const ZONE_BLURB = {
  ambiance: 'sans effet : scène de la salle',
  animateur: 'sans effet : blanc fixe',
  public: 'sans effet : non pilotée',
}
const ZONE_NEW = {
  ambiance: '+ Nouveau groupe',
  animateur: '🎤 + Nouveau groupe animateur',
  public: '👥 + Nouveau groupe public',
}
const ZONE_HINT = {
  animateur: "La zone Animateur éclaire le pupitre, la régie, la table de l'animateur… Elle n'est entraînée ni par l'éclairage de la salle, ni par ON / OFF, ni par le Flash. Sans effet choisi, elle reste en blanc fixe.",
  public: "La zone Public rassemble les lampes placées côté public. Tant qu'aucun effet ne lui est attribué, BuzzControl ne lui envoie rien : ces lampes restent comme vous les avez réglées. Elle n'est concernée ni par ON / OFF, ni par le Flash. Sans groupe public, rien ne change.",
}

function readDrag(e) {
  try { return JSON.parse(e.dataTransfer.getData(DND_TYPE) || 'null') } catch { return null }
}

export default function LightGroupsManager({
  generalLights = [], groups, enabled = true,
  disabled = false, testUnavailableReason = '', onSave, onTest, onStopTest, onChooseEffects = null, onNotify = () => {},
}) {
  const saved = useMemo(
    () => pruneToGeneral(draftFromConfig(groups), generalLights),
    [groups, generalLights]
  )
  // null = « suit l'enregistré » (dérivé dans le même rendu, jamais fixé par un effet).
  const [edited, setEdited] = useState(null)
  const draft = edited ?? saved
  const dirty = edited !== null && !draftsEqual(edited, saved)
  const [saving, setSaving] = useState(false)
  // Essai en séquence : { ref, startedAt, durationMs } tant que la séquence tourne ; `starting` = requête en vol.
  const [running, setRunning] = useState(null)
  const [starting, setStarting] = useState(null)
  const [now, setNow] = useState(() => Date.now())
  const [dragOver, setDragOver] = useState(null)

  const update = (fn) => setEdited(prev => fn(prev ?? saved))
  const errors = useMemo(() => validateDraft(draft), [draft])
  const valid = errors.global.length === 0
  const reordered = edited !== null && orderChanged(edited, saved)
  const free = unassignedLights(draft, generalLights)
  const summary = summarize(draft, generalLights)
  const total = groupCount(draft)

  // La séquence se termine d'elle-même au bout de `duration_ms` ; un tic de 250 ms met l'étape en cours à jour.
  useEffect(() => {
    if (!running) return undefined
    const tick = setInterval(() => {
      const t = Date.now()
      setNow(t)
      if (t - running.startedAt >= running.durationMs) setRunning(null)
    }, 250)
    return () => clearInterval(tick)
  }, [running])

  if (generalLights.length === 0 && total === 0) {
    return (
      <section className="lgm" aria-labelledby="lgm-title">
        <h2 id="lgm-title" className="ambiance-step-title">Groupes</h2>
        <p className="ambiance-hint">
          Aucune ampoule n'a le rôle « Éclairage général ». Attribuez ce rôle dans « Pont et ampoules »
          pour créer des groupes.
        </p>
      </section>
    )
  }

  // ---- glisser-déposer ----
  const startDrag = (payload) => (e) => {
    e.dataTransfer.setData(DND_TYPE, JSON.stringify(payload))
    e.dataTransfer.effectAllowed = 'move'
  }
  const allowDrop = (key) => (e) => {
    if (disabled) return
    e.preventDefault()
    e.dataTransfer.dropEffect = 'move'
    if (dragOver !== key) setDragOver(key)
  }
  // Dépôt sur un groupe (ou le bac) : une ampoule y est rangée ; un groupe de la même zone prend son rang.
  const dropOn = (target) => (e) => {
    e.preventDefault()
    setDragOver(null)
    if (disabled) return
    const data = readDrag(e)
    if (!data) return
    if (data.type === 'light') update(d => moveLight(d, data.name, target))
    else if (data.type === 'group' && target.kind === 'group' && data.zone === target.zone) {
      update(d => moveGroup(d, data.zone, data.index, target.index))
    }
  }
  const poolTarget = { kind: 'pool' }

  // ---- actions ----
  const handleSave = async () => {
    setSaving(true)
    try {
      const res = await onSave(draftToPayload(draft))
      if (res?.ok) { setEdited(null); onNotify({ message: 'Groupes enregistrés', type: 'success' }) }
      else onNotify({ message: res?.message || 'Enregistrement impossible', type: 'error' })
    } finally { setSaving(false) }
  }
  const handleTest = async (ref) => {
    // Un clic sur le groupe en cours d'essai l'arrête (bouton « ■ Arrêter l'essai »).
    if (running?.ref === ref) {
      setRunning(null)
      const res = await onStopTest?.()
      if (res && !res.ok) onNotify({ message: res.message || "Arrêt de l'essai impossible", type: 'warning' })
      return
    }
    setStarting(ref)
    try {
      const res = await onTest(ref)
      if (res?.ok) {
        const durationMs = Number.isFinite(res.duration_ms) ? res.duration_ms : TEST_STEPS.length * 1000
        setNow(Date.now())
        setRunning({ ref, startedAt: Date.now(), durationMs }) // remplace l'essai d'un autre groupe
      } else onNotify({ message: res?.message || 'Essai impossible', type: 'warning' })
    } finally { setStarting(null) }
  }

  // Raison d'indisponibilité de l'essai (infobulle) ; '' = disponible.
  const testDisabledReason = (empty) =>
    dirty ? "Enregistrez d'abord" : empty ? 'Groupe vide' : !enabled ? "Éclairage d'ambiance désactivé" : testUnavailableReason

  const moveSelectValue = (name) => {
    const loc = locateLight(draft, name)
    return loc.kind === 'group' ? `${loc.zone}:${loc.index}` : 'pool'
  }
  const onMoveSelect = (name) => (e) => {
    const v = e.target.value
    if (v === 'pool') { update(d => moveLight(d, name, poolTarget)); return }
    const [zone, index] = v.split(':')
    update(d => moveLight(d, name, { kind: 'group', zone, index: Number(index) }))
  }

  const renderChip = (name) => (
    <span
      key={name}
      className="lgm-chip"
      draggable={!disabled}
      onDragStart={startDrag({ type: 'light', name })}
      data-testid={`chip-${name}`}
    >
      <span className="lgm-grip" aria-hidden="true">⠿</span>
      <span className="lgm-chip-name">{name}</span>
      <select
        className="lgm-move"
        value={moveSelectValue(name)}
        onChange={onMoveSelect(name)}
        disabled={disabled}
        aria-label={`Déplacer ${name} vers…`}
      >
        <option value="pool">Hors groupe</option>
        {ZONES.flatMap(zone => draft[zone].map((_, i) => (
          <option key={`${zone}:${i}`} value={`${zone}:${i}`}>{ZONE_ICON[zone]} {ZONE_LABEL[zone]} · {groupName(i)}</option>
        )))}
      </select>
      {moveSelectValue(name) !== 'pool' && (
        <button
          type="button" className="lgm-x" disabled={disabled}
          aria-label={`Retirer ${name} du groupe`}
          onClick={() => update(d => moveLight(d, name, poolTarget))}
        >✕</button>
      )}
    </span>
  )

  const renderTest = (ref, empty) => {
    const isRunning = running?.ref === ref
    const reason = isRunning ? '' : testDisabledReason(empty)
    const step = isRunning ? testStepAt(now - running.startedAt) : 0
    const title = isRunning ? `Étape ${step + 1}/${TEST_STEPS.length} : ${TEST_STEPS[step]}` : (reason || 'Joue une séquence de 6 secondes sur les ampoules du groupe')
    return (
      <Button
        variant="secondary" size="sm"
        disabled={disabled || (!isRunning && !!reason) || (starting !== null && starting !== ref)}
        loading={starting === ref}
        title={title}
        onClick={() => handleTest(ref)}
      >
        {isRunning ? "■ Arrêter l'essai" : 'Tester le groupe'}
      </Button>
    )
  }

  const renderGroup = (zone, g, i) => (
    <div
      key={i}
      className={`lgm-grp lgm-grp-${zone}${g.lights.length === 0 ? ' is-empty' : ''}${dragOver === `${zone}${i}` ? ' is-over' : ''}`}
      onDragOver={allowDrop(`${zone}${i}`)} onDragLeave={() => setDragOver(null)}
      onDrop={dropOn({ kind: 'group', zone, index: i })}
      data-testid={`lgm-${zone}-${i + 1}`}
    >
      <div className="lgm-handle-col">
        <span
          className="lgm-handle" draggable={!disabled} title="Glisser pour changer l'ordre"
          onDragStart={startDrag({ type: 'group', zone, index: i })} aria-hidden="true"
        >⠿</span>
        <button type="button" className="lgm-mini" disabled={disabled || i === 0}
          aria-label={`Monter le groupe ${i + 1} de la zone ${ZONE_LABEL[zone]}`} onClick={() => update(d => moveGroup(d, zone, i, i - 1))}>↑</button>
        <button type="button" className="lgm-mini" disabled={disabled || i === draft[zone].length - 1}
          aria-label={`Descendre le groupe ${i + 1} de la zone ${ZONE_LABEL[zone]}`} onClick={() => update(d => moveGroup(d, zone, i, i + 1))}>↓</button>
      </div>
      <span className={`lgm-num lgm-num-${zone}`} aria-label={`Groupe ${i + 1}, zone ${ZONE_LABEL[zone]}`}>{i + 1}</span>
      <div className="lgm-body">
        <strong className="lgm-title">{groupName(i)}</strong>
        <div className="lgm-drop">
          {g.lights.length > 0
            ? g.lights.map(renderChip)
            : <span className="lgm-drop-empty">Aucune ampoule — glissez-en une ici. Un groupe vide est ignoré par les effets.</span>}
        </div>
      </div>
      <div className="lgm-actions">
        {renderTest(testRef(zone, i), g.lights.length === 0)}
        <Button variant="ghost" size="sm" disabled={disabled}
          aria-label={`Supprimer le groupe ${i + 1} de la zone ${ZONE_LABEL[zone]}`}
          onClick={() => update(d => removeGroup(d, zone, i))}>Supprimer</Button>
      </div>
    </div>
  )

  const renderZone = (zone) => {
    const list = draft[zone]
    const atMax = list.length >= MAX_GROUPS_PER_ZONE
    return (
      <section key={zone} className={`lgm-zone lgm-zone-${zone}`} aria-label={`Zone ${ZONE_LABEL[zone]}`} data-testid={`lgm-zone-${zone}`}>
        <h3 className="lgm-h lgm-zone-title">
          {ZONE_ICON[zone]} Zone {ZONE_LABEL[zone]} — {list.length} groupe{list.length > 1 ? 's' : ''}
        </h3>
        <div className="lgm-order" aria-label={`Ordre de parcours, zone ${ZONE_LABEL[zone]}`}>
          {list.length > 0 && <>Ordre de parcours :{' '}</>}
          {list.map((_, i) => (
            <span key={i}>{i > 0 && ' → '}<span className="lgm-order-item">g{i + 1}</span></span>
          ))}
          {list.length > 1 && ' → retour à g1'}
          {list.length > 0 && ' · '}{ZONE_BLURB[zone]}
          {zone !== 'ambiance' && list.length > 0 && (
            <>
              {' · '}
              {onChooseEffects && !dirty && list.every(g => g.id)
                ? <button type="button" className="lgm-link" onClick={() => onChooseEffects(zone)}>Choisir les effets de la zone</button>
                : <span className="lgm-soon" title="Enregistrez d'abord les groupes">Choisir les effets de la zone</span>}
            </>
          )}
        </div>
        {list.map((g, i) => renderGroup(zone, g, i))}
        <div className="lgm-row">
          <Button variant={zone === 'ambiance' && list.length === 0 ? 'primary' : 'ghost'} size="sm" disabled={disabled || atMax}
            onClick={() => update(d => addGroup(d, zone))}>
            {ZONE_NEW[zone]}
          </Button>
        </div>
        {atMax && <p className="ambiance-hint">{MAX_GROUPS_PER_ZONE} groupes au plus dans la zone {ZONE_LABEL[zone]}.</p>}
        {ZONE_HINT[zone] && <p className="ambiance-hint">{ZONE_HINT[zone]}</p>}
      </section>
    )
  }

  return (
    <section className="lgm" aria-labelledby="lgm-title">
      <h2 id="lgm-title" className="ambiance-step-title">Groupes</h2>
      <p className="ambiance-hint">
        Rassemblez les ampoules de l'éclairage général en groupes, dans l'une des trois zones : Ambiance (la salle),
        Animateur, Public. Les effets (chenillard…) désignent les groupes par leur numéro, du groupe 1 au dernier :
        rangez-les dans l'ordre où ils se suivent dans la salle. Dans un fichier d'effet, <code>g1</code> est le
        groupe 1 et <code>g0</code> toute la zone. Les ampoules d'équipe gardent la couleur de leur équipe et ne
        figurent pas ici.
      </p>
      {reordered && (
        <div className="lgm-warn" role="status">
          L'ordre a changé : <code>g1</code>, <code>g2</code>… désignent maintenant d'autres groupes dans les effets.
        </div>
      )}

      <div className="lgm-cols">
        <div
          className={`lgm-pool${dragOver === 'pool' ? ' is-over' : ''}`}
          onDragOver={allowDrop('pool')} onDragLeave={() => setDragOver(null)} onDrop={dropOn(poolTarget)}
          data-testid="lgm-pool"
        >
          <h3 className="lgm-h">Ampoules générales hors groupe ({free.length})</h3>
          <div>{free.map(renderChip)}</div>
          <p className="ambiance-hint">
            Glissez une ampoule vers un groupe. Une ampoule hors groupe fait partie de la zone Ambiance : elle garde
            l'éclairage de la salle mais ne participe pas aux effets répartis.
          </p>
        </div>

        <div className="lgm-main">
          {total === 0 && (
            <p className="ambiance-hint">
              Aucun groupe pour l'instant : l'éclairage se comporte comme avant. Les effets répartis (chenillard…)
              demandent des groupes. Créez un groupe, puis glissez-y des ampoules.
            </p>
          )}
          {ZONES.map(renderZone)}
        </div>
      </div>

      {total > 0 && (summary.free > 0 || summary.empty > 0) && (
        <div className="lgm-warn" role="status">
          {[
            summary.free > 0 && `${summary.free} ampoule${summary.free > 1 ? 's' : ''} générale${summary.free > 1 ? 's sont' : ' est'} hors groupe`,
            summary.empty > 0 && `${summary.empty} groupe${summary.empty > 1 ? 's' : ''} d'ambiance ${summary.empty > 1 ? 'sont vides' : 'est vide'}`,
          ].filter(Boolean).join(' et ')} : les effets répartis animeront {summary.active} groupe{summary.active > 1 ? 's' : ''} dans la zone Ambiance.
        </div>
      )}
      {errors.global.map(m => <div key={m} className="lgm-warn is-error" role="alert">{m}</div>)}

      <div className="lgm-foot">
        <Button variant="primary" onClick={handleSave} loading={saving} disabled={disabled || !dirty || !valid}>
          Enregistrer les groupes
        </Button>
        <Button variant="secondary" disabled={disabled || !dirty || saving} onClick={() => setEdited(null)}>
          Annuler les modifications
        </Button>
        {dirty && <span className="ambiance-hint">Modifications non enregistrées</span>}
      </div>
    </section>
  )
}

export { EMPTY_DRAFT }
