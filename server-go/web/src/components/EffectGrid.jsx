import { useEffect, useState } from 'react'
import { NAMED_RGB } from './EffectBrush'
import {
  COLUMN_PRE, COLUMN_LAST, COLUMN_POST, MAX_STEPS, canAddGroup, canRemoveGroup, columnLabel, getCell, gridRows,
} from '../utils/effectDocument'

// #210 (v11.8.0, LOT-2) — grille pas × cibles (maquette effect_editor_r4 §01, contrat §11.8).
// Colonnes : PRE, les pas (un par tick), DERNIER, POST. Lignes : variables en haut, puis g0, g1…g<n>, et TOUJOURS g* en dernier.
// Une case = une action (couleur + intensité, ou « éteint ») ; une case vide n'écrit rien. Le pinceau est choisi à part.
//
// Props : doc, numbered (lignes de groupes numérotés), onCell(column, target), readOnly,
//   errorColumns (Set de colonnes en erreur : 'pre' | 'last' | 'post' | indice de pas), playingColumn (surlignage, F2.3),
//   onGoStep(column) facultatif (clic sur l'en-tête d'un pas : place l'aperçu), onAddStep/onDuplicateStep/onInsertStep/onMoveStep/onRemoveStep,
//   onAddGroup / onRemoveGroup

const rgbLuma = (hex) => {
  const [r, g, b] = [1, 3, 5].map(i => parseInt(hex.slice(i, i + 2), 16))
  return r * 0.3 + g * 0.59 + b * 0.11
}
const colorLabel = (c) => ({ theme: 'thème', equipe: 'équipe', urgence: 'urgence', scene: 'scène' }[c] || c)

function ActionChip({ action }) {
  if (action.off) return <span className="efx-act is-off">éteint</span>
  const bg = NAMED_RGB[action.color] || action.color
  const fg = /^#[0-9a-fA-F]{6}$/.test(bg) && rgbLuma(bg) < 150 ? '#fff' : '#111827'
  return (
    <span className="efx-act" style={{ background: bg, color: fg, opacity: 0.45 + (action.intensity ?? 100) / 180 }}>
      {colorLabel(action.color)} {action.intensity} %
    </span>
  )
}

function rowNote(row) {
  if (row.kind === 'all') return 'toute la zone'
  if (row.kind === 'last') return 'dernier groupe'
  if (row.kind === 'variable') {
    const v = row.variable
    if (v.kind === 'list') return `suite ${(v.items || []).join(' ')}`
    return `${v.start ?? ''}${v.step === null || v.step === undefined || v.step === '' ? '' : ` ; ${v.step}`}`
  }
  if (row.kind === 'extra') return 'voisin (fichier)'
  return ''
}

export default function EffectGrid({
  doc, numbered, onCell, readOnly = false, errorColumns = new Set(), playingColumn = null, onGoStep,
  onAddStep, onDuplicateStep, onInsertStep, onMoveStep, onRemoveStep, onAddGroup, onRemoveGroup,
}) {
  const [menu, setMenu] = useState(null) // indice du pas dont le menu est ouvert

  useEffect(() => {
    if (menu === null) return undefined
    const onDown = (e) => { if (!e.target.closest?.('.efx-stepmenu, .efx-stepmenu-btn')) setMenu(null) }
    const onKey = (e) => { if (e.key === 'Escape') setMenu(null) }
    document.addEventListener('mousedown', onDown)
    document.addEventListener('keydown', onKey)
    return () => { document.removeEventListener('mousedown', onDown); document.removeEventListener('keydown', onKey) }
  }, [menu])

  const rows = gridRows(doc, numbered)
  const columns = [COLUMN_PRE, ...doc.steps.map((_, i) => i), COLUMN_LAST, COLUMN_POST]
  const span = columns.length + 2
  const isSpecial = (c) => typeof c === 'string'
  const cls = (c) => `${errorColumns.has(c) ? ' is-err' : ''}${playingColumn === c ? ' is-playing' : ''}`
  const firstVar = rows.findIndex(r => r.kind === 'variable')
  const firstAll = rows.findIndex(r => r.kind === 'all')

  return (
    <div className="efx-gridwrap" data-testid="effect-grid">
      <table className="efx-grid">
        <thead>
          <tr>
            <th scope="col"><span className="efx-sr">Cible</span></th>
            {columns.map((c, idx) => (
              <ColumnHeader
                key={String(c)} column={c} special={isSpecial(c)} className={cls(c)} readOnly={readOnly}
                menuOpen={menu === c} stepCount={doc.steps.length}
                onGo={onGoStep ? () => onGoStep(c) : undefined}
                onToggleMenu={() => setMenu(m => (m === c ? null : c))}
                onAction={(fn) => { setMenu(null); fn(c) }}
                handlers={{ onDuplicateStep, onInsertStep, onMoveStep, onRemoveStep }}
                addAfter={!isSpecial(c) && (idx + 1 >= columns.length || isSpecial(columns[idx + 1])) && !readOnly
                  ? <th key="add" className="efx-addstep"><button type="button" title="Ajouter un pas" aria-label="Ajouter un pas" onClick={onAddStep} disabled={doc.steps.length >= MAX_STEPS}>＋</button></th>
                  : null}
              />
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((row, ri) => (
            <RowFragment key={row.target} first={ri === firstVar ? 'var' : ri === firstAll ? 'all' : ''} span={span}>
              <tr className={`efx-row efx-row-${row.kind}`}>
                <th scope="row">{row.target}<small>{rowNote(row)}</small></th>
                {columns.map((c, idx) => {
                  const action = getCell(doc, c, row.target)
                  const lastOfSteps = !isSpecial(c) && (idx + 1 >= columns.length || isSpecial(columns[idx + 1]))
                  return (
                    <CellFragment key={String(c)} extra={lastOfSteps && !readOnly}>
                      <td className={`efx-cell${cls(c)}`}>
                        <button
                          type="button" disabled={readOnly} data-testid={`cell-${String(c)}-${row.target}`}
                          aria-label={`${columnLabel(c)}, ${row.target} : ${action ? (action.off ? 'éteint' : `${colorLabel(action.color)} ${action.intensity} %`) : 'vide'}`}
                          onClick={() => onCell(c, row.target)}
                        >{action ? <ActionChip action={action} /> : null}</button>
                      </td>
                    </CellFragment>
                  )
                })}
              </tr>
            </RowFragment>
          ))}
          {!readOnly && (
            <tr className="efx-grouprow">
              <th colSpan={span}>
                <button type="button" className="efx-mini" onClick={onAddGroup} disabled={!canAddGroup(numbered)}>＋ Ajouter un groupe</button>{' '}
                <button type="button" className="efx-mini" onClick={onRemoveGroup} disabled={!canRemoveGroup(numbered)}>－ Retirer un groupe</button>{' '}
                <span className="efx-hint">{numbered + 1} ligne{numbered ? 's' : ''} de groupes dans la grille · la dernière ligne est toujours g*</span>
              </th>
            </tr>
          )}
        </tbody>
      </table>
    </div>
  )
}

// Bandeaux « Variables » / « Fond et groupes fixes » insérés avant la première ligne concernée.
function RowFragment({ first, span, children }) {
  return (
    <>
      {first === 'var' && <tr className="efx-band"><th colSpan={span}>Variables — repeignent le fond et les groupes</th></tr>}
      {first === 'all' && <tr className="efx-band"><th colSpan={span}>Fond et groupes fixes</th></tr>}
      {children}
    </>
  )
}

// Cellule vide supplémentaire sous le « ＋ » d'ajout de pas.
function CellFragment({ extra, children }) {
  return <>{children}{extra && <td className="efx-addcell" aria-hidden="true" />}</>
}

function ColumnHeader({ column, special, className, readOnly, menuOpen, stepCount, onGo, onToggleMenu, onAction, handlers, addAfter }) {
  const label = columnLabel(column)
  return (
    <>
      <th scope="col" className={`efx-colh${special ? ' is-special' : ''}${className}`}>
        {onGo
          ? <button type="button" className="efx-colgo" title="Placer l'aperçu sur ce pas" onClick={onGo}>{label}</button>
          : <span>{label}</span>}
        {!special && !readOnly && (
          <>
            <button type="button" className="efx-stepmenu-btn" aria-label={`Menu du ${label}`} aria-expanded={menuOpen} onClick={onToggleMenu}>▾</button>
            {menuOpen && (
              <div className="efx-stepmenu" role="menu" aria-label={`Menu du ${label}`}>
                <button type="button" role="menuitem" onClick={() => onAction(handlers.onDuplicateStep)}>Dupliquer</button>
                <button type="button" role="menuitem" onClick={() => onAction(handlers.onInsertStep)}>Insérer un pas avant</button>
                <button type="button" role="menuitem" disabled={column === 0} onClick={() => onAction((c) => handlers.onMoveStep(c, -1))}>Déplacer vers la gauche</button>
                <button type="button" role="menuitem" disabled={column === stepCount - 1} onClick={() => onAction((c) => handlers.onMoveStep(c, 1))}>Déplacer vers la droite</button>
                <button type="button" role="menuitem" disabled={stepCount <= 1} onClick={() => onAction(handlers.onRemoveStep)}>Supprimer</button>
              </div>
            )}
          </>
        )}
      </th>
      {addAfter}
    </>
  )
}

