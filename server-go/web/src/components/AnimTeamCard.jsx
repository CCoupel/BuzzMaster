import { getRgbColor, getContrastColor } from '../utils/colorUtils'
import './AnimTeamCard.css'

/**
 * AnimTeamCard — carte d'équipe de base pour la page animateur (#155, F4).
 *
 * Périmètre F4 : nom, couleur, score — c'est tout ce que /anim affiche tant
 * que le socle n'est pas enrichi par #156/F6 (rang de buzz, temps de
 * réaction, bouton de crédit). Volontairement construit pour être ÉTENDU
 * sans réécriture (plan §7 R10) : `children` est le point d'extension que F6
 * utilisera pour ajouter son contenu sans toucher au balisage nom/couleur/
 * score ci-dessous.
 *
 * @param {string} name
 * @param {Array|string} color - couleur d'équipe (array RGB ou string CSS, voir colorUtils)
 * @param {number} [score]
 * @param {React.ReactNode} [children] - point d'extension pour F6 (rang, temps, bouton de crédit, ...)
 */
export default function AnimTeamCard({ name, color, score = 0, children }) {
  const bgColor = getRgbColor(color)
  const textColor = Array.isArray(color) ? getContrastColor(color) : '#ffffff'

  return (
    <div
      className="anim-team-card"
      style={{ '--anim-team-color': bgColor, '--anim-team-text': textColor }}
    >
      <div className="anim-team-card-header">
        <span className="anim-team-card-name">{name}</span>
        <span className="anim-team-card-score">{score}</span>
      </div>
      {children && <div className="anim-team-card-extra">{children}</div>}
    </div>
  )
}
