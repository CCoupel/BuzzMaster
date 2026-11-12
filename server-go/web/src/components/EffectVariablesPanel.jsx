import { useEffect, useRef, useState } from 'react'
import { MAX_VARIABLES, itemsText, parseItems } from '../utils/effectDocument'

// #210 (v11.8.0, LOT-2) — panneau « Variables de groupe » (maquette effect_editor_r4 §01).
// Une ligne par variable : nom proposé, type (« Avance régulière » / « Suite de groupes »), départ et pas (EXPRESSIONS du §12.2,
// vérifiées par le serveur), ordre, suppression. La bulle « ? » d'une variable donne une phrase et la suite obtenue pour 1, 3, 6
// et 12 groupes, LUE dans `vars` des aperçus du brouillon (§11.3) : le navigateur ne calcule jamais une position.
//
// Props : variables[], errors { [index]: string[] } (erreurs du serveur par `path`), previews[], disabled,
//   onAdd(), onKind(i, kind), onPatch(i, patch), onMove(i, delta), onRemove(i)

const START_SUGGESTIONS = ['1', 'N/2', 'N/2+1', 'N', 'g*', 'gA+1', 'gA-1']
const SIZES = [1, 3, 6, 12]
const SEQ_LEN = 7

// Suite des groupes désignés par une variable, d'après les trames d'un aperçu : « g3 g4 g5 … » (— = aucun groupe).
export function sequenceText(preview, name) {
  const frames = preview?.frames || []
  if (!frames.length) return '—'
  const out = frames.slice(0, SEQ_LEN).map(f => {
    const v = f?.vars?.[name]
    return Number.isFinite(v) && v > 0 ? `g${v}` : '—'
  })
  return `${out.join(' ')}${frames.length > SEQ_LEN ? ' …' : ''}`
}

const follows = (v) => v.kind === 'progression' && (v.step === null || v.step === undefined || String(v.step).trim() === '') && /g[A-Z]/.test(v.start || '')

// Phrase qui décrit une variable (texte seulement : aucun calcul).
export function variableSentence(v) {
  if (v.kind === 'list') return "Suit les groupes dans l'ordre donné, un par pas, puis recommence. Un groupe qui n'existe pas dans la salle est ignoré."
  if (follows(v)) return `Vaut toujours « ${v.start} » : elle suit la variable citée, pas après pas.`
  if (v.step === null || v.step === undefined || String(v.step).trim() === '') return `Reste sur le groupe « ${v.start} ».`
  return `Part du groupe « ${v.start} » et avance de « ${v.step} » à chaque pas ; après le dernier groupe, repart du premier. N = nombre de groupes de la zone ; une division ne garde que la partie entière ; hors de 1…N, on reboucle (0 = dernier groupe).`
}

export default function EffectVariablesPanel({ variables, errors = {}, previews = [], disabled = false, onAdd, onKind, onPatch, onMove, onRemove }) {
  const [open, setOpen] = useState(null) // 'general' | indice de variable
  const root = useRef(null)

  // La bulle se ferme au clic suivant (hors d'elle) ou à Échap.
  useEffect(() => {
    if (open === null) return undefined
    const onDown = (e) => { if (!e.target.closest?.('.efx-pop, .efx-q')) setOpen(null) }
    const onKey = (e) => { if (e.key === 'Escape') setOpen(null) }
    document.addEventListener('mousedown', onDown)
    document.addEventListener('keydown', onKey)
    return () => { document.removeEventListener('mousedown', onDown); document.removeEventListener('keydown', onKey) }
  }, [open])

  const toggle = (key) => setOpen(o => (o === key ? null : key))
  const previewFor = (n) => previews.find(p => p.groups === n)

  return (
    <section className="efx-panel" ref={root} aria-label="Variables de groupe" data-testid="effect-variables">
      <div className="efx-head efx-rel">
        <h3 className="efx-h3">Variables de groupe</h3>
        <button type="button" className="efx-q" aria-label="À quoi servent les variables ?" aria-expanded={open === 'general'} onClick={() => toggle('general')}>?</button>
        <span className="efx-grow" />
        <button type="button" className="efx-mini" onClick={onAdd} disabled={disabled || variables.length >= MAX_VARIABLES}>+ Ajouter</button>
        {open === 'general' && (
          <div className="efx-pop" role="dialog" aria-label="À quoi servent les variables ?">
            <p>Une variable désigne un groupe qui <b>change à chaque pas</b>. Chaque variable a sa ligne en haut de la grille.</p>
            <p>
              <b>Avance régulière</b> — un <b>départ</b> et un <b>pas</b>. Exemples : départ <code>1</code>, pas <code>+1</code> → g1, g2, g3… ;
              départ <code>N/2</code> → part du milieu ; départ <code>N</code> ou <code>g*</code>, pas <code>-1</code> → part du dernier groupe et recule.
            </p>
            <p>
              <b>N</b> est le nombre de groupes de la zone où l'effet est joué. On peut écrire <code>N/2</code>, <code>N/2+1</code>, <code>N-1</code>,
              <code>(N+1)/2</code>… Une division ne garde que la partie entière (5 groupes : <code>N/2</code> = 2). Un résultat hors de 1…N reboucle :
              <code>N+1</code> = g1, <code>0</code> = le dernier groupe.
            </p>
            <p>
              <b>Suivre une autre variable</b> : départ <code>gA+1</code> <b>sans pas</b> = toujours le groupe après <code>gA</code>.
              <b> Suite de groupes</b> : une liste (g1, g4, g3), un groupe par pas.
            </p>
          </div>
        )}
      </div>

      <div className={`efx-vars${variables.length > 8 ? ' is-scroll' : ''}`}>
        {variables.length === 0 && <p className="efx-hint">Aucune variable. « + Ajouter » en crée une (24 au plus).</p>}
        {variables.map((v, i) => {
          const errs = errors[i] || []
          return (
            <div key={v.name} className="efx-var efx-rel" data-testid={`var-${v.name}`}>
              <div className="efx-l1">
                <span className="efx-nm">{v.name}</span>
                <select
                  className="efx-vk" aria-label={`Type de ${v.name}`} value={v.kind} disabled={disabled}
                  onChange={(e) => onKind(i, e.target.value)}
                >
                  <option value="progression">Avance régulière</option>
                  <option value="list">Suite de groupes</option>
                </select>
                {v.kind === 'progression' ? (
                  <>
                    <label className="efx-vf">départ
                      <input
                        className="efx-vs" list="efx-starts" aria-label={`Départ de ${v.name}`} disabled={disabled}
                        title="Groupe de départ : un numéro, N (nombre de groupes), N/2, g*, ou une autre variable (gA+1)"
                        value={v.start ?? ''} onChange={(e) => onPatch(i, { start: e.target.value })}
                      />
                    </label>
                    <label className="efx-vf">pas
                      <input
                        className="efx-vi" aria-label={`Pas de ${v.name}`} disabled={disabled} placeholder="suit"
                        title="Avance à chaque pas ; vide = suit le départ" value={v.step ?? ''}
                        onChange={(e) => onPatch(i, { step: e.target.value.trim() === '' ? null : e.target.value })}
                      />
                    </label>
                  </>
                ) : (
                  <input
                    className="efx-vl" aria-label={`Groupes suivis par ${v.name}`} disabled={disabled}
                    title="Groupes suivis, dans l'ordre" defaultValue={itemsText(v.items)} key={itemsText(v.items)}
                    onBlur={(e) => onPatch(i, { items: parseItems(e.target.value) })}
                    onKeyDown={(e) => { if (e.key === 'Enter') onPatch(i, { items: parseItems(e.target.value) }) }}
                  />
                )}
                <span className="efx-grow" />
                <button type="button" className="efx-q" aria-label={`Ce que fait ${v.name}`} aria-expanded={open === i} onClick={() => toggle(i)}>?</button>
                <button type="button" className="efx-mini" aria-label={`Monter ${v.name}`} title="Monter" disabled={disabled || i === 0} onClick={() => onMove(i, -1)}>▲</button>
                <button type="button" className="efx-mini" aria-label={`Descendre ${v.name}`} title="Descendre" disabled={disabled || i === variables.length - 1} onClick={() => onMove(i, 1)}>▼</button>
                <button type="button" className="efx-mini" aria-label={`Supprimer ${v.name}`} title="Supprimer la variable" disabled={disabled} onClick={() => onRemove(i)}>✕</button>
              </div>
              {errs.length > 0 && <div className="efx-var-err" role="alert">{errs.join(' ; ')}</div>}
              {open === i && (
                <div className="efx-pop" role="dialog" aria-label={`Ce que fait ${v.name}`}>
                  <p><b>{v.name}</b> — {variableSentence(v)}</p>
                  <div className="efx-seq">
                    {SIZES.map(n => (
                      <div key={n}><b>{n} groupe{n > 1 ? 's' : ''} :</b> {previewFor(n) ? sequenceText(previewFor(n), v.name) : '…'}</div>
                    ))}
                  </div>
                </div>
              )}
            </div>
          )
        })}
      </div>
      <datalist id="efx-starts">{START_SUGGESTIONS.map(s => <option key={s} value={s} />)}</datalist>
    </section>
  )
}
