import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { useGame } from '../hooks/GameContext'
import Podium from '../components/Podium'
import './ScoresPage.css'

// #252 — bornes du montant libre (contrat : entier non nul de −999 à 999).
const FREE_LIMIT = 999
const FREE_ERROR = 'Entier de −999 à 999'

// Valide la saisie libre : '' = vide (pas de montant libre) ; sinon entier non nul dans les bornes.
function parseFreeAmount(text) {
  if (text === '') return { empty: true, valid: true, value: null }
  const n = Number(text)
  const valid = Number.isInteger(n) && n !== 0 && Math.abs(n) <= FREE_LIMIT
  return { empty: false, valid, value: valid ? n : null }
}

export default function ScoresPage() {
  const { teams, bumpers, gameState, setBumperPoints, setTeamPoints } = useGame()
  const [pointsAmount, setPointsAmount] = useState(1)
  // #252 — montant libre (« autre ») et interrupteur « Afficher sur la TV » : activé à CHAQUE
  // ouverture de la page (état de page, jamais sauvegardé : pas de correction cachée oubliée).
  const [freeText, setFreeText] = useState('')
  const [tvDisplay, setTvDisplay] = useState(true)
  const free = parseFreeAmount(freeText)
  const freeActive = !free.empty
  const invalid = freeActive && !free.valid
  const effectiveAmount = freeActive && free.valid ? free.value : pointsAmount
  const shownAmount = Math.abs(effectiveAmount) // le signe se choisit avec − / +

  // « Dernières attributions » : mêmes événements que la page Historique (/history), 5 plus récents.
  const [recent, setRecent] = useState([])
  const mountedRef = useRef(true)
  const loadRecent = useCallback(async () => {
    if (typeof fetch !== 'function') return
    try {
      const response = await fetch('/history')
      if (!response.ok) return
      const data = (await response.json()) || []
      if (!mountedRef.current) return
      setRecent(
        data
          .filter(e => e.EVENT_TYPE !== 'TEAM_DELETED')
          .sort((a, b) => b.TIMESTAMP - a.TIMESTAMP)
          .slice(0, 5)
      )
    } catch {
      // historique indisponible : la section reste vide
    }
  }, [])
  // Rafraîchissement périodique (5 s), jamais réinitialisé par les UPDATE.
  useEffect(() => {
    mountedRef.current = true
    loadRecent()
    const id = setInterval(loadRecent, 5000)
    return () => { mountedRef.current = false; clearInterval(id) }
  }, [loadRecent])
  // Et seulement quand une attribution vient d'avoir lieu (SCORE_CHANGE.SEQ), pas à chaque UPDATE.
  const scoreSeq = gameState?.SCORE_CHANGE?.SEQ
  const firstSeqRef = useRef(true)
  useEffect(() => {
    if (firstSeqRef.current) { firstSeqRef.current = false; return }
    loadRecent()
  }, [scoreSeq, loadRecent])

  // Sort teams by score (descending) with rank calculation
  const sortedTeams = useMemo(() => {
    const sorted = Object.entries(teams)
      .map(([name, data]) => ({
        name,
        score: data.SCORE || 0,
        color: data.COLOR,
      }))
      .sort((a, b) => b.score - a.score)

    let currentRank = 1
    let lastScore = null
    let lastRank = 1
    return sorted.map((team, index) => {
      if (index > 0 && team.score === lastScore) {
        return { ...team, rank: lastRank }
      }
      currentRank = index + 1
      lastScore = team.score
      lastRank = currentRank
      return { ...team, rank: currentRank }
    })
  }, [teams])

  // Calculate max scores for progress bars
  const maxTeamScore = useMemo(() => Math.max(...sortedTeams.map(t => t.score), 1), [sortedTeams])

  // Sort players by score with rank calculation
  const sortedPlayers = useMemo(() => {
    const sorted = Object.entries(bumpers)
      .map(([mac, data]) => ({
        mac,
        name: data.NAME || mac.slice(-6),
        team: data.TEAM,
        score: data.SCORE || 0,
        teamColor: teams[data.TEAM]?.COLOR,
      }))
      .sort((a, b) => b.score - a.score)

    let currentRank = 1
    let lastScore = null
    let lastRank = 1
    return sorted.map((player, index) => {
      if (index > 0 && player.score === lastScore) {
        return { ...player, rank: lastRank }
      }
      currentRank = index + 1
      lastScore = player.score
      lastRank = currentRank
      return { ...player, rank: currentRank }
    })
  }, [bumpers, teams])

  const maxPlayerScore = useMemo(() => Math.max(...sortedPlayers.map(p => p.score), 1), [sortedPlayers])

  // Top 3 players for podium
  const topPlayers = useMemo(() => {
    return sortedPlayers.slice(0, 3).map(player => ({
      ...player,
      name: player.name,
      color: player.teamColor,
    }))
  }, [sortedPlayers])

  const getRgbColor = (color) => {
    if (!color) return 'var(--gray-400)'
    if (Array.isArray(color)) return `rgb(${color.join(',')})`
    return color
  }

  const getMedalEmoji = (rank) => {
    if (rank === 1) return '🥇'
    if (rank === 2) return '🥈'
    if (rank === 3) return '🥉'
    return `#${rank}`
  }

  // #252 : DISPLAY:false seulement quand l'interrupteur est coupé (sinon aucun champ : v11.1).
  const handleTeamPoints = (teamName, amount) => {
    if (invalid) return
    setTeamPoints(teamName, amount, { display: tvDisplay })
  }

  const handlePlayerPoints = (playerMac, amount) => {
    if (invalid) return
    setBumperPoints(playerMac, amount, { display: tvDisplay })
  }

  const silentTitle = tvDisplay ? '' : ' sans affichage TV'
  const renderControls = (onChange, id) => (
    <div className="ranking-controls">
      <button
        className="control-btn remove"
        onClick={() => onChange(id, -shownAmount)}
        disabled={invalid}
        title={`Retirer ${shownAmount} point(s)${silentTitle}`}
        aria-label={`Retirer ${shownAmount} point(s)${silentTitle}`}
      >
        −{shownAmount}{!tvDisplay && <span className="silent-mark" aria-hidden="true">🔇</span>}
      </button>
      <button
        className="control-btn add"
        onClick={() => onChange(id, shownAmount)}
        disabled={invalid}
        title={`Donner ${shownAmount} point(s)${silentTitle}`}
        aria-label={`Donner ${shownAmount} point(s)${silentTitle}`}
      >
        +{shownAmount}{!tvDisplay && <span className="silent-mark" aria-hidden="true">🔇</span>}
      </button>
    </div>
  )

  return (
    <div className="scores-page page">
      <header className="page-header">
        <h1 className="page-title">Gestion des Scores</h1>
        <div className="points-selector">
          <span className="points-label">Points:</span>
          <div className="points-buttons">
            {[1, 2, 5, 10].map(val => (
              <button
                key={val}
                className={`points-btn ${!freeActive && pointsAmount === val ? 'active' : ''}`}
                onClick={() => { setPointsAmount(val); setFreeText('') }}
              >
                {val}
              </button>
            ))}
          </div>
          <span className="free-points">
            <input
              type="number"
              step="1"
              min={-FREE_LIMIT}
              max={FREE_LIMIT}
              value={freeText}
              onChange={(e) => setFreeText(e.target.value)}
              className={`free-points-input${freeActive && free.valid ? ' active' : ''}`}
              placeholder="autre"
              aria-label="Nombre de points libre (négatif autorisé)"
              aria-invalid={invalid}
              title="Nombre libre, de −999 à 999"
            />
            {invalid && <span className="free-points-error" role="alert">{FREE_ERROR}</span>}
          </span>
        </div>
        <div className="tv-toggle">
          <span className="tv-toggle-label">📺 Afficher sur la TV</span>
          <button
            type="button"
            className={`tv-switch${tvDisplay ? ' on' : ''}`}
            role="switch"
            aria-checked={tvDisplay}
            aria-label="Afficher les attributions sur la TV"
            onClick={() => setTvDisplay(v => !v)}
          />
          <span className="tv-state">{tvDisplay ? 'animation + son + LED' : 'correction cachée'}</span>
        </div>
      </header>
      {!tvDisplay && (
        <div className="hidden-banner" role="status">
          🔇 Correction cachée : les points sont enregistrés sans animation, sans son ni LED. Le classement de la TV est mis à jour sans animation.
        </div>
      )}

      <div className="scores-grid">
        {/* Teams Section */}
        <section className="scores-section teams-section">
          <h2 className="section-title">Equipes</h2>

          <div className="section-content">
            {/* Teams Podium */}
            <div className="podium-container">
              {sortedTeams.length >= 1 && (
                <Podium teams={sortedTeams} variant="compact" />
              )}
            </div>

            {/* Teams List */}
            <div className="ranking-list">
              <AnimatePresence mode="popLayout">
                {sortedTeams.map((team) => {
                  const rgbColor = getRgbColor(team.color)
                  const isTied = sortedTeams.filter(t => t.rank === team.rank).length > 1

                  const barWidth = (team.score / maxTeamScore) * 100

                  return (
                    <motion.div
                      key={team.name}
                      className="ranking-item"
                      initial={{ opacity: 0, y: 20 }}
                      animate={{ opacity: 1, y: 0 }}
                      exit={{ opacity: 0, y: -20 }}
                      layout
                      style={{ '--team-color': rgbColor }}
                    >
                      <div className="ranking-rank">
                        <span className="rank-medal">{getMedalEmoji(team.rank)}</span>
                        {isTied && <span className="tied-badge">ex</span>}
                      </div>
                      <div className="ranking-badge" style={{ backgroundColor: rgbColor }}>
                        {team.name.charAt(0).toUpperCase()}
                      </div>
                      <div className="ranking-info">
                        <span className="ranking-name">{team.name}</span>
                        <div className="ranking-bar">
                          <motion.div
                            className="ranking-bar-fill"
                            initial={{ width: 0 }}
                            animate={{ width: `${barWidth}%` }}
                            transition={{ duration: 0.5 }}
                          />
                        </div>
                      </div>
                      <div className="ranking-score">{team.score}</div>
                      {renderControls(handleTeamPoints, team.name)}
                    </motion.div>
                  )
                })}
              </AnimatePresence>
            </div>
          </div>
        </section>

        {/* Players Section */}
        <section className="scores-section players-section">
          <h2 className="section-title">Joueurs</h2>

          <div className="section-content">
            {/* Players Podium */}
            <div className="podium-container">
              {topPlayers.length >= 1 && (
                <Podium teams={topPlayers} variant="compact" />
              )}
            </div>

            {/* Players List */}
            <div className="ranking-list">
              <AnimatePresence mode="popLayout">
                {sortedPlayers.map((player) => {
                  const rgbColor = getRgbColor(player.teamColor)
                  const isTied = sortedPlayers.filter(p => p.rank === player.rank).length > 1
                  const barWidth = (player.score / maxPlayerScore) * 100

                  return (
                    <motion.div
                      key={player.mac}
                      className="ranking-item player-item"
                      initial={{ opacity: 0, y: 20 }}
                      animate={{ opacity: 1, y: 0 }}
                      exit={{ opacity: 0, y: -20 }}
                      layout
                      style={{ '--team-color': rgbColor }}
                    >
                      <div className="ranking-rank">
                        <span className="rank-medal">{getMedalEmoji(player.rank)}</span>
                        {isTied && <span className="tied-badge">ex</span>}
                      </div>
                      <div className="ranking-badge" style={{ backgroundColor: rgbColor }}>
                        {player.name.charAt(0).toUpperCase()}
                      </div>
                      <div className="ranking-info">
                        <span className="ranking-name">{player.name}</span>
                        <span className="ranking-team">{player.team || 'Sans equipe'}</span>
                        <div className="ranking-bar">
                          <motion.div
                            className="ranking-bar-fill"
                            initial={{ width: 0 }}
                            animate={{ width: `${barWidth}%` }}
                            transition={{ duration: 0.5 }}
                          />
                        </div>
                      </div>
                      <div className="ranking-score">{player.score}</div>
                      {renderControls(handlePlayerPoints, player.mac)}
                    </motion.div>
                  )
                })}
              </AnimatePresence>
            </div>
          </div>
        </section>
      </div>

      {/* #252 — Dernières attributions : 5 plus récentes, même source que la page Historique */}
      <section className="scores-section recent-awards">
        <h2 className="section-title">Dernières attributions</h2>
        {recent.length === 0 ? (
          <p className="recent-awards-empty">Aucune attribution pour l'instant.</p>
        ) : (
          <ul className="recent-awards-list">
            {recent.map((e, i) => {
              const points = e.POINTS || 0
              const who = e.WINNER_TYPE === 'TEAM' ? e.TEAM_NAME : (e.PLAYER_NAME || e.TEAM_NAME)
              const hiddenEntry = e.DISPLAY === false
              return (
                <li key={`${e.TIMESTAMP}-${i}`} className={hiddenEntry ? 'hidden-entry' : undefined}>
                  {points === 0 ? '0 pt' : `${points > 0 ? '+' : '−'}${Math.abs(points)}`} {who}
                  {e.QUESTION_ID ? ` (question ${e.QUESTION_ID})` : ''}
                  {hiddenEntry && <span className="silent-mark" title="Correction cachée : sans affichage TV"> 🔇 correction cachée</span>}
                </li>
              )
            })}
          </ul>
        )}
      </section>
    </div>
  )
}
