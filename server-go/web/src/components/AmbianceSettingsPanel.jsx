import { useMemo, useState } from 'react'
import { CUE_TABS, cueMeta } from '../utils/soundCues'
import './AmbianceSettingsPanel.css'

// #231 / #290 (D7) — composant MUTUALISÉ « Réglages d'ambiance » : édite les
// réglages de COMPORTEMENT son + lumière (contracts/ambiance-settings.md §2).
// Un seul composant, deux modes :
//   mode="global"   — page Ambiance : `value` = Settings complet ; pas d'état
//                     « Hérité » ; colonnes catalogue (fichier, durée, Remplacer,
//                     Tester…) fournies par le parent via `renderCatalogue`.
//   mode="override" — éditeur de manche (LOT-4) : `value` = Partial (clé absente
//                     = hérité), `base` = réglages effectifs hérités (affichés en
//                     italique) ; chaque réglage a un état « Hérité » ;
//                     `onChange(field, null)` retire la surcharge.
//
// Props
//   mode      'global' | 'override'
//   value     { sound?: { muted?, countdown?: { enabled?, loop? }, cues?: { [cue]: bool } }, light?: { mode? } }
//   base      même forme, complète (mode override : valeurs héritées affichées)
//   onChange  (field, value|null) — field ∈ 'sound.muted' | 'sound.countdown.enabled'
//             | 'sound.countdown.loop' | 'sound.cues.<cue>' | 'light.mode'
//   cues      [{ cue, type }] — registre ordonné (GET /api/sounds : global puis par type)
//   sections  sous-ensemble de ['sound','countdown','cues','light'] (défaut : toutes)
//   renderCatalogue(cue)   mode global : cellules <td> catalogue de la ligne
//   catalogueColumns       [<th>…] en-têtes correspondants
//   renderExtraRows(tabKey) lignes <tr> supplémentaires d'un onglet (ex. sons longs, onglet Global)
//   emptyRow               <tr> affiché quand `cues` est vide (chargement / aucun son)
//   renderFile(cue)  mode override (T27, LOT-4 Batch 2) : contenu de la colonne « Fichier » (fichier de manche /
//                    fichier global hérité, boutons) — fourni par RoundAmbianceDialog, comme renderCatalogue en global
//   files     mode override : { [cue]: fichier de manche } — sert au point « • » des onglets (réglage OU fichier surchargé)
//   disabled  fige tous les contrôles

const get = (obj, path) => path.split('.').reduce((o, k) => (o == null ? undefined : o[k]), obj)
const ALL_SECTIONS = ['sound', 'countdown', 'cues', 'light']

function Tri({ options, selected, onSelect, label, disabled }) {
  return (
    <span className="asp-tri" role="radiogroup" aria-label={label}>
      {options.map(o => (
        <button
          key={String(o.value)}
          type="button"
          role="radio"
          aria-checked={selected === o.value}
          className={`${selected === o.value ? 'is-selected' : ''} ${o.value === null ? 'is-inherit' : ''}`}
          disabled={disabled}
          onClick={() => selected !== o.value && onSelect(o.value)}
        >
          {o.label}
        </button>
      ))}
    </span>
  )
}

export default function AmbianceSettingsPanel({
  mode = 'global',
  value = {},
  base = null,
  onChange,
  cues = [],
  sections = ALL_SECTIONS,
  renderCatalogue = null,
  catalogueColumns = [],
  renderExtraRows = null,
  emptyRow = null,
  renderFile = null,
  files = null,
  disabled = false,
}) {
  const override = mode === 'override'
  const has = (s) => sections.includes(s)
  const [tab, setTab] = useState('')

  // Valeur courante d'un champ : en surcharge, undefined = hérité (null pour Tri).
  const cur = (field) => {
    const v = get(value, field)
    return v === undefined ? (override ? null : undefined) : v
  }
  const inherited = (field, fmt) => (override && base != null ? <span className="asp-inherit">global : {fmt(get(base, field))}</span> : null)
  const setField = (field, v) => onChange && onChange(field, v)
  const inh = override ? [{ value: null, label: 'Hérité' }] : []

  const byTab = useMemo(() => {
    const m = {}
    cues.forEach(c => { (m[c.type || ''] = m[c.type || ''] || []).push(c) })
    return m
  }, [cues])
  // Barre d'onglets seulement si le serveur expose des cues typées (sinon : l'écran v11.4.0, sans onglets).
  const typed = cues.some(c => c.type)
  const activeTab = typed ? tab : ''
  const rows = byTab[activeTab] || []
  const overridden = (key) => override && (byTab[key] || []).some(c => get(value, `sound.cues.${c.cue}`) !== undefined || !!files?.[c.cue])
  const colSpan = 2 + catalogueColumns.length
  const fileCol = override && !!renderFile

  const onoff = (v) => (v ? 'actif' : 'inactif')

  return (
    <div className="ambiance-settings-panel" data-mode={mode}>
      {has('sound') && (
        <div className="asp-row">
          <span className="asp-label">Son</span>
          <Tri
            label="Son"
            disabled={disabled}
            selected={override ? cur('sound.muted') : !!cur('sound.muted')}
            options={[...inh, { value: false, label: 'ON' }, { value: true, label: 'Coupé' }]}
            onSelect={v => setField('sound.muted', v)}
          />
          {override ? inherited('sound.muted', m => (m ? 'coupé' : 'ON'))
            : <span className="asp-inherit">Même réglage que le popover d'en-tête ; persisté.</span>}
        </div>
      )}

      {has('countdown') && (
        <>
          <div className="asp-row">
            <span className="asp-label">Décompte</span>
            <Tri
              label="Décompte actif"
              disabled={disabled}
              selected={override ? cur('sound.countdown.enabled') : cur('sound.countdown.enabled') !== false}
              options={[...inh, { value: true, label: 'Actif' }, { value: false, label: 'Inactif' }]}
              onSelect={v => setField('sound.countdown.enabled', v)}
            />
            <Tri
              label="Lecture du décompte"
              disabled={disabled}
              selected={override ? cur('sound.countdown.loop') : cur('sound.countdown.loop') !== false}
              options={[...inh, { value: false, label: 'Une fois' }, { value: true, label: 'En boucle' }]}
              onSelect={v => setField('sound.countdown.loop', v)}
            />
            {inherited('sound.countdown', c => `${c?.enabled === false ? 'inactif' : 'actif'}, ${c?.loop === false ? 'une fois' : 'en boucle'}`)}
          </div>
          {fileCol && (
            <div className="asp-row asp-file-row">
              <span className="asp-label">Fichier du décompte</span>
              <span className="asp-file">{renderFile('decompte')}</span>
            </div>
          )}
        </>
      )}

      {has('cues') && (
        <>
          <h4 className="asp-title">Bruitages</h4>
          {typed && (
            <div className="asp-subtabs" role="tablist" aria-label="Bruitages par type">
              {CUE_TABS.map(t => (
                <button
                  key={t.key || 'global'}
                  type="button"
                  role="tab"
                  aria-selected={activeTab === t.key}
                  className={`asp-subtab ${activeTab === t.key ? 'is-active' : ''}`}
                  onClick={() => setTab(t.key)}
                >
                  {t.label}
                  <span className="asp-count">{(byTab[t.key] || []).length}</span>
                  {overridden(t.key) && <span className="asp-dot" aria-label="surchargé">•</span>}
                </button>
              ))}
            </div>
          )}
          <div className="sound-table-scroll">
            <table className="sound-table">
              <thead>
                <tr>
                  <th>{override ? 'Événement' : 'Moment du jeu'}</th>
                  <th className="sound-col-center">{override ? 'Pendant cette manche' : 'Actif'}</th>
                  {override ? <th>Valeur globale</th> : catalogueColumns}
                  {fileCol && <th>Fichier</th>}
                </tr>
              </thead>
              <tbody>
                {cues.length === 0 && emptyRow}
                {!override && rows.length > 0 && activeTab === '' && (
                  <tr className="sound-section-row"><th colSpan={colSpan} scope="colgroup">Bruitages (impulsions, ≤ 5 s)</th></tr>
                )}
                {rows.map(c => {
                  const meta = cueMeta(c.cue)
                  const field = `sound.cues.${c.cue}`
                  const enabled = override ? cur(field) : cur(field) !== false
                  return (
                    <tr key={c.cue} className={!override && enabled === false ? 'is-off' : ''}>
                      <td><b>{meta.label}</b><span className="sound-sub">{meta.sub}</span></td>
                      <td className="sound-col-center">
                        {override ? (
                          <Tri
                            label={meta.label}
                            disabled={disabled}
                            selected={enabled}
                            options={[...inh, { value: true, label: 'Actif' }, { value: false, label: 'Inactif' }]}
                            onSelect={v => setField(field, v)}
                          />
                        ) : (
                          <label className="sound-switch sound-switch-sm">
                            <input
                              type="checkbox"
                              checked={enabled}
                              disabled={disabled}
                              onChange={e => setField(field, e.target.checked)}
                              aria-label={`Activer ${meta.label}`}
                            />
                            <span className="sound-switch-track" aria-hidden="true" />
                          </label>
                        )}
                      </td>
                      {override
                        ? <td className="asp-inherit">{onoff(get(base, field) !== false)}</td>
                        : renderCatalogue && renderCatalogue(c.cue)}
                      {fileCol && <td className="asp-file">{renderFile(c.cue)}</td>}
                    </tr>
                  )
                })}
                {!override && renderExtraRows && renderExtraRows(activeTab)}
              </tbody>
            </table>
          </div>
        </>
      )}

      {has('light') && (
        <div className="asp-row">
          <span className="asp-label">Mode d'éclairage{override ? '' : ' par défaut'}</span>
          <Tri
            label="Mode d'éclairage"
            disabled={disabled}
            selected={override ? cur('light.mode') : (cur('light.mode') || 'AUTO')}
            options={[...inh, { value: 'ON', label: 'ON' }, { value: 'AUTO', label: 'AUTO' }, { value: 'OFF', label: 'OFF' }]}
            onSelect={v => setField('light.mode', v)}
          />
          {inherited('light.mode', m => m || 'AUTO')}
        </div>
      )}
    </div>
  )
}
