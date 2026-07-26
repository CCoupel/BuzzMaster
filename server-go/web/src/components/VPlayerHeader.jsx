import { getRgbColor } from '../utils/colorUtils'
import './VPlayerHeader.css'

export default function VPlayerHeader({ bumper, team }) {
  const isAssigned = bumper?.TEAM && team

  // Get first letter of player name for avatar
  const initial = bumper?.NAME?.[0]?.toUpperCase() || '?'

  // Get team color (boosted, #113) or default gray
  const avatarColor = isAssigned && team?.COLOR
    ? getRgbColor(team.COLOR, 'var(--gray-500)')
    : 'var(--gray-500)'

  return (
    <div className={`vplayer-header ${isAssigned ? 'assigned' : 'waiting'}`}>
      <div className="vplayer-avatar" style={{ backgroundColor: avatarColor }}>
        <span className="avatar-initial">{initial}</span>
      </div>

      <div className="vplayer-info">
        <div className="vplayer-name">{bumper?.NAME || 'Joueur'}</div>
        {isAssigned ? (
          <>
            <div className="vplayer-team" style={{ color: avatarColor }}>
              {team.NAME}
            </div>
            <div className="vplayer-score">
              {bumper.SCORE} pts
            </div>
          </>
        ) : (
          <div className="vplayer-status">
            En attente d'assignation...
          </div>
        )}
      </div>
    </div>
  )
}
