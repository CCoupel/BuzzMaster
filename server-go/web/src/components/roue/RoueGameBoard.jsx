/**
 * RoueGameBoard — plateau de jeu ROUE (TV, tablette, admin, animateur), contrat roue.md §4 `ROUE_BOARD`.
 *
 * Caractères : espace = pas de case ; `_` = lettre cachée ; `*` = case cachée éclairée (étape 4) ; tout autre
 * caractère = lettre visible. Le retournement (étape 5) : une case qui passe de `*` à une lettre se retourne,
 * à son tour dans l'ordre de lecture, une toutes les `flipIntervalMs` ; un client qui monte directement sur un
 * plateau déjà révélé affiche les lettres sans animation.
 *
 * `colorOf(letter)` (optionnel, animateur/admin) : couleur d'une lettre trouvée (équipe qui l'a proposée).
 * `reveal` (optionnel, admin/anim) : phrase complète — les cases cachées s'affichent en grisé.
 */
import { useRef } from 'react'
import { boardTopOffset } from './boardLayout'
import './RoueGameBoard.css'

export default function RoueGameBoard({ board, flipIntervalMs = 600, size = 'tv', colorOf, solution = null }) {
  const cols = board?.COLS || 14
  const rows = board?.ROWS || []
  const prev = useRef(null)
  const flips = useRef({}) // clé de case → rang de retournement ; conservée tant que le plateau ne change pas (l'animation ne doit pas être coupée par un re-rendu)
  const before = prev.current
  prev.current = rows
  if (before && before !== rows) {
    const next = {}
    let n = 0
    rows.forEach((r, ri) => [...r].forEach((ch, i) => { if (ch !== '*' && ch !== '_' && ch !== ' ' && before[ri]?.[i] === '*') next[`${ri}:${i}`] = n++ }))
    if (n > 0) flips.current = next
    else if (rows.some(r => r.includes('*')) || before.join('') !== rows.join('')) flips.current = {}
  }
  const cells = []
  // Phrase centrée verticalement dans les 4 lignes (#310) ; `ri` = ligne affichée, `sri` = ligne du serveur (retournements, solution).
  const top = boardTopOffset(rows.length)
  const lines = [...Array(top).fill(''), ...rows]
  while (lines.length < 4) lines.push('')
  lines.slice(0, Math.max(4, rows.length + top)).forEach((r, ri) => {
    const sri = ri - top
    const pad = Math.floor((cols - r.length) / 2)
    for (let c = 0; c < cols; c++) {
      const idx = c - pad
      const ch = c < pad ? undefined : r[idx]
      const key = `${ri}-${c}`
      if (!ch || ch === ' ') { cells.push(<div key={key} className="rgb-cell" />); continue }
      const flipRank = flips.current[`${sri}:${idx}`]
      const style = {}
      let cls = 'rgb-cell'
      let content = null
      if (ch === '_') {
        cls += ' tile'
        const hidden = solution?.[sri]?.[idx]
        if (hidden && hidden !== ' ') content = <span className="rgb-ghost">{hidden}</span>
      } else if (ch === '*') cls += ' lit'
      else {
        cls += ' shown'
        content = ch
        if (colorOf) { const col = colorOf(ch); if (col) { style['--rgb-c'] = col; cls += ' mine' } }
        if (flipRank !== undefined) { cls += ' flip'; style['--rgb-i'] = flipRank }
      }
      cells.push(<div key={key} className={cls} style={style}>{content}</div>)
    }
  })
  return (
    <div className={`rgb-board rgb-${size}`} style={{ '--rgb-cols': cols, '--rgb-flip': `${flipIntervalMs}ms` }}
      role="img" aria-label="Plateau de la phrase">
      {cells}
    </div>
  )
}
