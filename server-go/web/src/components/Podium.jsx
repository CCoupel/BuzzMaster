import { useMemo } from 'react'
import { motion } from 'framer-motion'
import './Podium.css'

export default function Podium({ teams, changedTeams = {}, variant = 'default' }) {
  // Sort teams by score and calculate ranks (use existing rank if present)
  const sortedTeams = useMemo(() => {
    const sorted = [...teams].sort((a, b) => b.score - a.score)

    // If teams already have ranks, use them; otherwise calculate
    if (sorted.length > 0 && sorted[0].rank !== undefined) {
      return sorted
    }

    let currentRank = 1
    return sorted.map((team, index) => {
      if (index > 0 && sorted[index - 1].score === team.score) {
        return { ...team, rank: currentRank }
      }
      currentRank = index + 1
      return { ...team, rank: currentRank }
    })
  }, [teams])

  // Group teams by podium position (handling ties)
  const podiumTeams = useMemo(() => {
    const first = sortedTeams.filter(t => t.rank === 1)
    const second = sortedTeams.filter(t => t.rank === 2)
    const third = sortedTeams.filter(t => t.rank === 3)
    return { first, second, third }
  }, [sortedTeams])

  const getRgbColor = (color) => {
    if (!color) return 'var(--gray-400)'
    if (Array.isArray(color)) return `rgb(${color.join(',')})`
    return color
  }

  const isCompact = variant === 'compact'

  return (
    <div className={`podium ${isCompact ? 'podium-compact' : ''}`}>
      {/* Second Place */}
      <div className="podium-step second">
        <div className="podium-step-teams">
          {podiumTeams.second.map(team => {
            const isChanged = changedTeams[team.name]
            return (
              <motion.div
                key={team.name}
                className={`podium-place ${isChanged ? 'changed' : ''}`}
                initial={{ y: 50, opacity: 0 }}
                animate={{ y: 0, opacity: 1 }}
                transition={{ delay: 0.3, type: 'spring', stiffness: 100 }}
                layout
              >
                <motion.div
                  className="podium-avatar"
                  style={{ backgroundColor: getRgbColor(team.color) }}
                  animate={isChanged === 'up' ? {
                    boxShadow: ['0 0 0px rgba(255,255,255,0)', '0 0 40px rgba(255,255,255,0.8)', '0 0 0px rgba(255,255,255,0)']
                  } : {}}
                  transition={{ duration: 1 }}
                >
                  {team.name.charAt(0).toUpperCase()}
                </motion.div>
                <div className="podium-name">{team.name}</div>
                <motion.div
                  className="podium-score"
                  key={team.score}
                  animate={isChanged ? { scale: [1, 1.3, 1] } : {}}
                  transition={{ duration: 0.5 }}
                >
                  {team.score} pts
                </motion.div>
              </motion.div>
            )
          })}
        </div>
        {podiumTeams.second.length > 0 && (
          <div className="podium-block second-block">2</div>
        )}
      </div>

      {/* First Place */}
      <div className="podium-step first">
        <div className="podium-step-teams">
          {podiumTeams.first.map(team => {
            const isChanged = changedTeams[team.name]
            return (
              <motion.div
                key={team.name}
                className={`podium-place ${isChanged ? 'changed' : ''}`}
                initial={{ y: 50, opacity: 0 }}
                animate={{ y: 0, opacity: 1 }}
                transition={{ delay: 0.1, type: 'spring', stiffness: 100 }}
                layout
              >
                <motion.div
                  className="podium-avatar first-avatar"
                  style={{ backgroundColor: getRgbColor(team.color) }}
                  animate={isChanged === 'up' ? {
                    boxShadow: ['0 0 60px rgba(255,215,0,0.5)', '0 0 100px rgba(255,215,0,1)', '0 0 60px rgba(255,215,0,0.5)']
                  } : {}}
                  transition={{ duration: 1 }}
                >
                  {team.name.charAt(0).toUpperCase()}
                </motion.div>
                <div className="podium-name first-name">{team.name}</div>
                <motion.div
                  className="podium-score first-score"
                  key={team.score}
                  animate={isChanged ? { scale: [1, 1.3, 1] } : {}}
                  transition={{ duration: 0.5 }}
                >
                  {team.score} pts
                </motion.div>
              </motion.div>
            )
          })}
        </div>
        <div className="podium-block first-block">1</div>
      </div>

      {/* Third Place */}
      <div className="podium-step third">
        <div className="podium-step-teams">
          {podiumTeams.third.map(team => {
            const isChanged = changedTeams[team.name]
            return (
              <motion.div
                key={team.name}
                className={`podium-place ${isChanged ? 'changed' : ''}`}
                initial={{ y: 50, opacity: 0 }}
                animate={{ y: 0, opacity: 1 }}
                transition={{ delay: 0.5, type: 'spring', stiffness: 100 }}
                layout
              >
                <motion.div
                  className="podium-avatar"
                  style={{ backgroundColor: getRgbColor(team.color) }}
                  animate={isChanged === 'up' ? {
                    boxShadow: ['0 0 0px rgba(255,255,255,0)', '0 0 40px rgba(255,255,255,0.8)', '0 0 0px rgba(255,255,255,0)']
                  } : {}}
                  transition={{ duration: 1 }}
                >
                  {team.name.charAt(0).toUpperCase()}
                </motion.div>
                <div className="podium-name">{team.name}</div>
                <motion.div
                  className="podium-score"
                  key={team.score}
                  animate={isChanged ? { scale: [1, 1.3, 1] } : {}}
                  transition={{ duration: 0.5 }}
                >
                  {team.score} pts
                </motion.div>
              </motion.div>
            )
          })}
        </div>
        {podiumTeams.third.length > 0 && (
          <div className="podium-block third-block">3</div>
        )}
      </div>
    </div>
  )
}
