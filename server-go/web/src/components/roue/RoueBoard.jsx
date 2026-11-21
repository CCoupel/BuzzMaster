/**
 * RoueBoard — plateau de la phrase à deviner (v12.0.0, #145, maquette M3).
 *
 * Affichage pur : reçoit les lignes déjà mises en forme par le serveur
 * (`POST /api/roue/preview`, contrat roue.md §3.1/§11.2). Aucune analyse de la
 * phrase ici — un seul évaluateur, côté serveur. Les lignes sont centrées sur
 * `cols` colonnes ; les voyelles sont teintées (non proposables, dévoilées à la
 * résolution).
 */
import './RoueBoard.css'

const VOWELS = 'AEIOUY'

export default function RoueBoard({ rows = [], cols = 14, maxRows = 4 }) {
  const lines = rows.slice(0, maxRows)
  while (lines.length < maxRows) lines.push('')
  return (
    <div className="roue-board-wrap">
      <div
        className="roue-board"
        role="img"
        aria-label={rows.length ? `Plateau : ${rows.join(', ')}` : 'Plateau vide'}
        style={{ gridTemplateColumns: `repeat(${cols}, minmax(0, 1fr))`, '--roue-cols': cols }}
      >
        {lines.map((row, ri) => {
          const pad = Math.floor((cols - row.length) / 2)
          return Array.from({ length: cols }, (_, c) => {
            const ch = row[c - pad]
            if (!ch || ch === ' ' || c < pad) return <div key={`${ri}-${c}`} className="roue-cell" />
            const kind = VOWELS.includes(ch) ? 'vowel' : 'letter'
            return <div key={`${ri}-${c}`} className={`roue-cell ${kind}`}>{ch}</div>
          })
        })}
      </div>
      <div className="roue-board-legend">
        <i className="roue-legend-letter" />consonne ou signe
        <i className="roue-legend-vowel" />voyelle (non proposable, dévoilée à la fin)
      </div>
    </div>
  )
}
