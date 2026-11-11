import { useState, useMemo, useEffect } from 'react'
import Button from './Button'
import {
  MAX_ROOM_GROUPS, MAX_HOST_GROUPS, MAX_NAME_LENGTH, EMPTY_DRAFT, TEST_STEPS, testStepAt,
  draftFromConfig, draftToPayload, draftsEqual, groupCount, pruneToGeneral, pruneTeams,
  unassignedLights, locateLight, moveLight, addGroup, removeGroup, renameGroup, moveGroup,
  addNearTeam, removeNearTeam, availableTeams, addHost, removeHost, renameHost, moveHost,
  validateDraft, summarize,
} from '../utils/lightGroups'
import './LightGroupsManager.css'

// #225 (v11.7.0, LOT-1) — sous-onglet « Groupes » de Lumière : rassemble les ampoules de
// l'éclairage général en groupes ordonnés + bloc « Groupe animateur ».
// Maquette : docs/mockup/v11.7.0/ui/ambiance_page__light_groups.html (sections 02 à 04).
// Contrat : contracts/light-effects.md §2.1, §2.4, §2.5.
//
// Props :
//   generalLights  noms des ampoules de rôle « general », dans l'ordre de la liste des ampoules
//   teams          noms d'équipes ; teamColor(name) -> couleur CSS ou null
//   groups         `lighting.groups` enregistré (tableau, éventuellement absent)
//   enabled        éclairage d'ambiance activé (l'essai est refusé sinon : 409)
//   testUnavailableReason  raison d'indisponibilité de l'essai (pont injoignable…), '' si disponible
//   onSave(payload) -> Promise<{ok, message?}>
//   onTest(n | 'host<k>') -> Promise<{ok, duration_ms?, message?}>   essai en séquence (6 pas d'1 s), réponse immédiate
//   onStopTest() -> Promise<{ok, message?}>                          POST …/groups/test/stop
//   onChooseEffects(hostId)  lien « Choisir ses effets » d'un groupe animateur (null : inactif)
//   onNotify({message,type}) toast de la page
//
// Glisser-déposer natif (dataTransfer), comme les fonds d'écran et les manches ; alternative
// clavier : « Déplacer vers… » par ampoule, « Monter » / « Descendre » par groupe.

const DND_TYPE = 'application/x-buzz-lightgroups'

function readDrag(e) {
  try { return JSON.parse(e.dataTransfer.getData(DND_TYPE) || 'null') } catch { return null }
}

export default function LightGroupsManager({
  generalLights = [], teams = [], teamColor = () => null, groups, enabled = true,
  disabled = false, testUnavailableReason = '', onSave, onTest, onStopTest, onChooseEffects = null, onNotify = () => {},
}) {
  const saved = useMemo(
    () => pruneTeams(pruneToGeneral(draftFromConfig(groups), generalLights), teams),
    [groups, generalLights, teams]
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
  const valid = errors.global.length === 0 && Object.keys(errors.byGroup).length === 0
  const free = unassignedLights(draft, generalLights)
  const summary = summarize(draft, generalLights)
  const total = groupCount(draft)
  const atMaxRoom = draft.groups.length >= MAX_ROOM_GROUPS
  const atMaxHost = draft.hosts.length >= MAX_HOST_GROUPS

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
        <h2 id="lgm-title" className="ambiance-step-title">Groupes d'ambiance</h2>
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
  const dropOn = (target, groupIndex) => (e) => {
    e.preventDefault()
    setDragOver(null)
    if (disabled) return
    const data = readDrag(e)
    if (!data) return
    if (data.type === 'light' && !String(target).endsWith('card')) update(d => moveLight(d, data.name, groupIndex))
    if (data.type === 'group' && target === 'groupcard') update(d => moveGroup(d, data.index, groupIndex.index))
    if (data.type === 'host' && target === 'hostcard') update(d => moveHost(d, data.index, groupIndex.index))
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
    return loc.kind === 'group' ? `g${loc.index}` : loc.kind === 'host' ? `h${loc.index}` : 'pool'
  }
  const onMoveSelect = (name) => (e) => {
    const v = e.target.value
    update(d => moveLight(d, name, v === 'pool' ? poolTarget : { kind: v[0] === 'h' ? 'host' : 'group', index: Number(v.slice(1)) }))
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
        {draft.groups.map((g, i) => <option key={i} value={`g${i}`}>{i + 1} · {g.name || `Groupe ${i + 1}`}</option>)}
        {draft.hosts.map((g, i) => <option key={`h${i}`} value={`h${i}`}>🎤{i + 1} · {g.name || `Animateur ${i + 1}`}</option>)}
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

  return (
    <section className="lgm" aria-labelledby="lgm-title">
      <h2 id="lgm-title" className="ambiance-step-title">Groupes d'ambiance</h2>
      <p className="ambiance-hint">
        Rassemblez les ampoules de l'éclairage général en groupes. Les effets de salle (chenillard,
        remplissage, vague…) animent les groupes dans l'ordre de cette liste, du groupe 1 au dernier :
        rangez-les dans l'ordre où ils se suivent dans la salle. Les ampoules d'équipe gardent la couleur
        de leur équipe et ne figurent pas ici.
      </p>

      {draft.groups.length > 0 && (
        <div className="lgm-order" aria-label="Ordre de parcours">
          Ordre de parcours :{' '}
          {draft.groups.map((g, i) => (
            <span key={i}>
              {i > 0 && ' → '}<span className="lgm-order-item">{i + 1} {g.name}</span>
            </span>
          ))}
          {draft.groups.length > 1 && ' → retour au 1'}
        </div>
      )}

      <div className="lgm-cols">
        <div
          className={`lgm-pool${dragOver === 'pool' ? ' is-over' : ''}`}
          onDragOver={allowDrop('pool')} onDragLeave={() => setDragOver(null)} onDrop={dropOn('pool', poolTarget)}
          data-testid="lgm-pool"
        >
          <h3 className="lgm-h">Ampoules générales hors groupe ({free.length})</h3>
          <div>{free.map(renderChip)}</div>
          <p className="ambiance-hint">
            Glissez une ampoule vers un groupe. Une ampoule hors groupe garde l'éclairage de la salle mais
            ne participe pas aux effets répartis.
          </p>
        </div>

        <div className="lgm-main">
          {total === 0 && (
            <p className="ambiance-hint">
              Aucun groupe pour l'instant : l'éclairage se comporte comme avant. Les effets répartis
              (chenillard, remplissage, vague, scintillement) demandent des groupes.
            </p>
          )}

          {draft.groups.map((g, i) => {
            const teamOptions = availableTeams(draft, teams)
            const err = errors.byGroup[i]
            return (
              <div
                key={i}
                className={`lgm-grp${g.lights.length === 0 ? ' is-empty' : ''}${dragOver === `grp${i}` ? ' is-over' : ''}`}
                onDragOver={allowDrop(`grp${i}`)} onDragLeave={() => setDragOver(null)}
                onDrop={(e) => {
                  const data = readDrag(e)
                  if (data?.type === 'group') dropOn('groupcard', { index: i })(e)
                  else dropOn('group', { kind: 'group', index: i })(e)
                }}
                data-testid={`lgm-group-${i + 1}`}
              >
                <div className="lgm-handle-col">
                  <span
                    className="lgm-handle" draggable={!disabled} title="Glisser pour changer l'ordre"
                    onDragStart={startDrag({ type: 'group', index: i })} aria-hidden="true"
                  >⠿</span>
                  <button type="button" className="lgm-mini" disabled={disabled || i === 0}
                    aria-label={`Monter le groupe ${i + 1}`} onClick={() => update(d => moveGroup(d, i, i - 1))}>↑</button>
                  <button type="button" className="lgm-mini" disabled={disabled || i === draft.groups.length - 1}
                    aria-label={`Descendre le groupe ${i + 1}`} onClick={() => update(d => moveGroup(d, i, i + 1))}>↓</button>
                </div>
                <span className="lgm-num" aria-label={`Groupe ${i + 1}`}>{i + 1}</span>
                <div className="lgm-body">
                  <input
                    className={`lgm-name${err ? ' is-invalid' : ''}`} value={g.name} maxLength={MAX_NAME_LENGTH + 10}
                    disabled={disabled} aria-label={`Nom du groupe ${i + 1}`} aria-invalid={!!err}
                    onChange={e => update(d => renameGroup(d, i, e.target.value))}
                  />
                  {err && <span className="lgm-err" role="alert">{err}</span>}
                  <div className="lgm-drop">
                    {g.lights.length > 0
                      ? g.lights.map(renderChip)
                      : <span className="lgm-drop-empty">Aucune ampoule — glissez-en une ici. Un groupe vide est ignoré par les effets.</span>}
                  </div>
                  <div className="lgm-line">
                    Équipes à proximité{' '}
                    <span
                      className="lgm-help" role="img" tabIndex={0} aria-label="Aide : équipes à proximité"
                      title="Quand une équipe marque, l'effet « Vague » part du groupe qui la cite ; sans indication, il part du groupe 1. Une équipe n'est citée que dans un groupe."
                    >?</span> :{' '}
                    {g.near_teams.length === 0 && <span className="lgm-none">aucune</span>}
                    {g.near_teams.map(t => (
                      <span key={t} className="lgm-chip lgm-chip-team">
                        <span className="lgm-dot" style={{ backgroundColor: teamColor(t) || 'var(--gray-300)' }} aria-hidden="true" />
                        {t}
                        <button type="button" className="lgm-x" disabled={disabled}
                          aria-label={`Retirer l'équipe ${t} du groupe ${i + 1}`}
                          onClick={() => update(d => removeNearTeam(d, i, t))}>✕</button>
                      </span>
                    ))}
                    <select
                      className="lgm-select" value="" disabled={disabled || teamOptions.length === 0}
                      aria-label={`Ajouter une équipe au groupe ${i + 1}`}
                      onChange={e => { if (e.target.value) update(d => addNearTeam(d, i, e.target.value)) }}
                    >
                      <option value="">+ Ajouter une équipe</option>
                      {teamOptions.map(t => <option key={t} value={t}>{t}</option>)}
                    </select>
                  </div>
                </div>
                <div className="lgm-actions">
                  {renderTest(i + 1, g.lights.length === 0)}
                  <Button variant="ghost" size="sm" disabled={disabled}
                    aria-label={`Supprimer le groupe ${i + 1}`}
                    onClick={() => update(d => removeGroup(d, i))}>Supprimer</Button>
                </div>
              </div>
            )
          })}

          <div className="lgm-row">
            <Button variant="ghost" size="sm" disabled={disabled || atMaxRoom} onClick={() => update(addGroup)}>
              + Nouveau groupe
            </Button>
          </div>
          {atMaxRoom && <p className="ambiance-hint">{MAX_ROOM_GROUPS} groupes de salle au plus.</p>}
          {draft.groups.length === 0 && draft.hosts.length === 0 && (
            <p className="ambiance-hint">Créez un groupe, nommez-le, puis glissez-y des ampoules.</p>
          )}

          <div className="lgm-host-wrap">
            <h3 className="lgm-h">Groupes animateur ({draft.hosts.length})</h3>
            {draft.hosts.map((g, i) => {
              const err = errors.byGroup[`h${i}`]
              return (
                <div
                  key={i}
                  className={`lgm-grp lgm-host${g.lights.length === 0 ? ' is-empty' : ''}${dragOver === `host${i}` ? ' is-over' : ''}`}
                  onDragOver={allowDrop(`host${i}`)} onDragLeave={() => setDragOver(null)}
                  onDrop={(e) => {
                    const data = readDrag(e)
                    if (data?.type === 'host') dropOn('hostcard', { index: i })(e)
                    else dropOn('host', { kind: 'host', index: i })(e)
                  }}
                  data-testid={`lgm-host-${i + 1}`}
                >
                  <div className="lgm-handle-col">
                    <span
                      className="lgm-handle" draggable={!disabled} title="Glisser pour changer l'ordre"
                      onDragStart={startDrag({ type: 'host', index: i })} aria-hidden="true"
                    >⠿</span>
                    <button type="button" className="lgm-mini" disabled={disabled || i === 0}
                      aria-label={`Monter le groupe animateur ${i + 1}`} onClick={() => update(d => moveHost(d, i, i - 1))}>↑</button>
                    <button type="button" className="lgm-mini" disabled={disabled || i === draft.hosts.length - 1}
                      aria-label={`Descendre le groupe animateur ${i + 1}`} onClick={() => update(d => moveHost(d, i, i + 1))}>↓</button>
                  </div>
                  <span className="lgm-num lgm-num-host" aria-label={`Groupe animateur ${i + 1}`}>🎤{i + 1}</span>
                  <div className="lgm-body">
                    <input
                      className={`lgm-name${err ? ' is-invalid' : ''}`} value={g.name} maxLength={MAX_NAME_LENGTH + 10}
                      disabled={disabled} aria-label={`Nom du groupe animateur ${i + 1}`} aria-invalid={!!err}
                      onChange={e => update(d => renameHost(d, i, e.target.value))}
                    />
                    {err && <span className="lgm-err" role="alert">{err}</span>}
                    <div className="lgm-drop">
                      {g.lights.length > 0
                        ? g.lights.map(renderChip)
                        : <span className="lgm-drop-empty">Aucune ampoule — glissez-en une ici.</span>}
                    </div>
                    <div className="lgm-line">
                      Blanc fixe par défaut ·{' '}
                      {onChooseEffects && g.id && !dirty
                        ? <button type="button" className="lgm-link" onClick={() => onChooseEffects(g.id)}>Choisir ses effets</button>
                        : <span className="lgm-soon" title="Enregistrez d'abord les groupes">Choisir ses effets</span>}
                    </div>
                  </div>
                  <div className="lgm-actions">
                    {renderTest(`host${i + 1}`, g.lights.length === 0)}
                    <Button variant="ghost" size="sm" disabled={disabled}
                      aria-label={`Supprimer le groupe animateur ${i + 1}`}
                      onClick={() => update(d => removeHost(d, i))}>Supprimer</Button>
                  </div>
                </div>
              )
            })}
            <div className="lgm-row">
              <Button variant="ghost" size="sm" disabled={disabled || atMaxHost} onClick={() => update(addHost)}>
                🎤 + Nouveau groupe animateur
              </Button>
            </div>
            {atMaxHost && <p className="ambiance-hint">{MAX_HOST_GROUPS} groupes animateur au plus.</p>}
            <p className="ambiance-hint">
              Les groupes animateur éclairent le pupitre, la régie, la table de l'animateur… Ils ne sont entraînés ni
              par les effets de salle, ni par ON / OFF, ni par le Flash : chacun reste en blanc fixe, sauf si vous lui
              choisissez un effet par moment (sous-onglet Effets). {MAX_HOST_GROUPS} groupes animateur au plus.
            </p>
          </div>
        </div>
      </div>

      {total > 0 && (summary.free > 0 || summary.empty > 0) && (
        <div className="lgm-warn" role="status">
          {[
            summary.free > 0 && `${summary.free} ampoule${summary.free > 1 ? 's' : ''} générale${summary.free > 1 ? 's sont' : ' est'} hors groupe`,
            summary.empty > 0 && `${summary.empty} groupe${summary.empty > 1 ? 's sont vides' : ' est vide'}`,
          ].filter(Boolean).join(' et ')} : les effets répartis animeront {summary.active} groupe{summary.active > 1 ? 's' : ''}.
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
