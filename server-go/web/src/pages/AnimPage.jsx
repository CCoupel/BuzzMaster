import { useEffect, useMemo, useRef } from 'react'
import NoSleep from 'nosleep.js'
import { useGame } from '../hooks/GameContext'
import { useCategories } from '../hooks/useCategories'
import { categoryMeta } from '../utils/categoryUtils'
import { sortTeamsByBuzzOrder, getRankBadge, formatReactionTime } from '../utils/buzzOrder'
import { resolvePointsAward } from '../utils/pointsAward'
import Timer from '../components/Timer'
import CategoryBadge from '../components/CategoryBadge'
import AnimTeamCard from '../components/AnimTeamCard'
import AnimConductPanel from '../components/AnimConductPanel'
import './AnimPage.css'

// Phases pendant lesquelles l'ordre de buzz (rang, réordonnancement) est
// actif — même règle que GamePage.jsx (utils/buzzOrder.js).
const BUZZ_ORDER_PHASES = ['STARTED', 'PAUSED', 'REVEALED', 'STOPPED']
// Rang affiché uniquement pendant ces phases (pas en STOPPED, même choix
// que TeamCard.jsx — le badge médaille perd son sens une fois arrêté).
const RANK_BADGE_PHASES = ['STARTED', 'PAUSED', 'REVEALED']
// Crédit actif uniquement une fois la question arrêtée (GamePage.jsx:1077).
const CREDIT_PHASES = ['STOPPED', 'REVEALED']

const STATUS_LABEL = {
  connected: 'Connecté',
  connecting: 'Connexion...',
  disconnected: 'Déconnecté',
}

/**
 * AnimPage — interface animateur (`/anim`, #155/#156).
 *
 * Gabarit à 3 zones (plan _work/reports/plan-20260813-094321.md §4 F4) :
 *   - Zone A (contexte) : question courante, chronomètre + phase, statut de
 *     connexion, question suivante.
 *   - Zone B (conduite, #156/F5) : gestes de pilotage SPEEDY (AnimConductPanel),
 *     contextuels à la phase — jamais un bouton inactif "pour information".
 *   - Zone C (équipes, #156/F6) : AnimTeamCard enrichie (rang de buzz, temps
 *     de réaction, bouton de crédit) sans réécriture du composant — voir
 *     AnimTeamCard.jsx (point d'extension `children`).
 *
 * Tablette paysage, pas de Navbar régie (App.jsx ne l'affiche que sur
 * /admin/* désormais — #155/F2). Connecté sur /ws/anim (ClientTypeAnim,
 * capacité réduite) via GameProvider (App.jsx).
 */
export default function AnimPage() {
  const {
    status,
    gameState,
    teams,
    bumpers,
    nextQuestion,
    startGame,
    stopGame,
    pauseGame,
    continueGame,
    revealAnswer,
    selectQuestion,
    setTeamPoints,
    setBumperPoints,
  } = useGame()

  // Veille écran — reprend le motif PlayerDisplay.jsx:912-921 : wake lock
  // natif (HTTPS) si disponible, repli NoSleep.js sinon. Le serveur n'expose
  // aucun TLS, donc ce repli est le chemin NOMINAL sur une tablette
  // animateur, pas une option de secours.
  const wakeLockRef = useRef(null)
  const noSleepRef = useRef(null)
  useEffect(() => {
    const startWakeLock = async () => {
      if ('wakeLock' in navigator) {
        try {
          wakeLockRef.current = await navigator.wakeLock.request('screen')
          return
        } catch (_) {
          // Repli NoSleep.js ci-dessous.
        }
      }
      if (!noSleepRef.current) noSleepRef.current = new NoSleep()
      if (!noSleepRef.current.isEnabled) noSleepRef.current.enable()
    }
    startWakeLock()

    return () => {
      if (wakeLockRef.current) {
        wakeLockRef.current.release()
        wakeLockRef.current = null
      }
      if (noSleepRef.current?.isEnabled) {
        noSleepRef.current.disable()
        noSleepRef.current = null
      }
    }
  }, [])

  const { categories: apiCategories } = useCategories()
  const customCategories = useMemo(() => apiCategories.filter(c => c.isCustom), [apiCategories])

  const question = gameState.question
  const categoryInfo = question?.CATEGORY ? categoryMeta(question.CATEGORY, customCategories) : null
  const nextCategoryInfo = nextQuestion?.CATEGORY ? categoryMeta(nextQuestion.CATEGORY, customCategories) : null

  // Zone B — mêmes conditions d'activation que /admin (GamePage.jsx:373-378),
  // seule la présentation change (AnimConductPanel, #156/F5).
  const isPlaying = gameState.phase === 'STARTED' || gameState.phase === 'PAUSED'
  const canReveal = gameState.phase === 'STOPPED' && gameState.question?.STATUS === 'STOPPED'
  const canStart = gameState.phase === 'READY'

  const handleStart = () => {
    const time = parseInt(gameState.question?.TIME) || 30
    const points = parseInt(gameState.question?.POINTS) || 1
    startGame(time, points)
  }

  // Zone C — mêmes équipes que /admin (au moins un joueur assigné — règle de
  // base #45, GamePage.jsx:135), triées par ordre de buzz pendant les phases
  // actives (utils/buzzOrder.js — même règle que GamePage.jsx, #156/F6).
  const displayTeams = useMemo(() => {
    const teamsWithPlayers = new Set(
      Object.values(bumpers)
        .filter(b => b.TEAM)
        .map(b => b.TEAM)
    )
    const list = Object.entries(teams)
      .filter(([name]) => teamsWithPlayers.has(name))
      .map(([name, data]) => ({ name, ...data }))
    return sortTeamsByBuzzOrder(list, gameState.phase)
  }, [teams, bumpers, gameState.phase])

  const showRankBadge = RANK_BADGE_PHASES.includes(gameState.phase)
  const showBuzzOrder = BUZZ_ORDER_PHASES.includes(gameState.phase)
  const creditEnabled = CREDIT_PHASES.includes(gameState.phase)

  // Crédit — montant unique via l'utilitaire partagé (F1), sans saisie ;
  // cible équipe ou joueur selon POINTS_TARGET (GamePage.jsx:404-411).
  // Calculé une seule fois : le montant ne dépend pas de l'équipe cliquée
  // (F5/F6 visent le mode SPEEDY — pas de pénalité QCM par-joueur ni de
  // score MEMORY ici, resolvePointsAward retombe donc sur le montant de
  // base), donc le même { amount, target } vaut pour le bouton de chaque
  // équipe — affiché et appliqué à l'identique.
  const { amount: creditAmount, target: creditTarget } = resolvePointsAward(
    gameState.question,
    parseInt(gameState.question?.POINTS) || 1,
    {}
  )
  // En PLAYER, crédite le bumper le plus rapide de l'équipe (dans le mode
  // SPEEDY, un seul bumper par équipe a TIME > 0 — le buzz est verrouillé
  // globalement dès le premier appui — donc c'est le même joueur que
  // /admin créditerait en cliquant sur son buzzer).
  const handleCredit = (teamName) => {
    if (!creditEnabled) return
    if (creditTarget === 'TEAM') {
      setTeamPoints(teamName, creditAmount)
      return
    }
    const fastestBumper = Object.entries(bumpers)
      .filter(([, b]) => b.TEAM === teamName && (b.TIME ?? 0) > 0)
      .sort((a, b) => a[1].TIME - b[1].TIME)[0]
    if (fastestBumper) setBumperPoints(fastestBumper[0], creditAmount)
  }

  return (
    <div className="anim-page">
      {/* Zone A — contexte */}
      <div className="anim-zone anim-zone-context">
        <span className={`anim-connection-status ${status}`}>
          <span className="anim-status-dot" />
          {STATUS_LABEL[status] || status}
        </span>

        <div className="anim-question-info">
          {question ? (
            <>
              {categoryInfo && (
                <CategoryBadge catKey={question.CATEGORY} customCategories={customCategories} size="lg" />
              )}
              <div className="anim-question-text">
                <span className="anim-question-id">#{question.ID}</span>
                <span className="anim-question-type">{question.TYPE || 'SPEEDY'}</span>
              </div>
            </>
          ) : (
            <span className="anim-question-empty">Aucune question en cours</span>
          )}
        </div>

        <Timer
          currentTime={gameState.timer}
          totalTime={gameState.totalTime}
          phase={gameState.phase}
          size="md"
        />

        <div className="anim-next-question">
          <span className="anim-next-question-label">Suivante</span>
          {nextQuestion?.ID ? (
            <span className="anim-next-question-text">
              {nextCategoryInfo && <CategoryBadge catKey={nextQuestion.CATEGORY} customCategories={customCategories} size="sm" />}
              {nextQuestion.TYPE || 'SPEEDY'}
            </span>
          ) : (
            <span className="anim-next-question-text anim-next-question-empty">—</span>
          )}
        </div>
      </div>

      {/* Zone B — conduite SPEEDY (#156/F5) */}
      <div className="anim-zone anim-zone-conduct">
        <AnimConductPanel
          phase={gameState.phase}
          isPlaying={isPlaying}
          canStart={canStart}
          canReveal={canReveal}
          nextQuestion={nextQuestion}
          onStart={handleStart}
          onPause={pauseGame}
          onContinue={continueGame}
          onStop={stopGame}
          onReveal={revealAnswer}
          onSelectNext={selectQuestion}
        />
      </div>

      {/* Zone C — équipes (#156/F6 : ordre de buzz, rang, temps, crédit) */}
      <div className="anim-zone anim-zone-teams">
        {displayTeams.map((team, index) => {
          const rank = index + 1
          const rankBadge = showRankBadge ? getRankBadge(rank) : null
          const reactionTime = showBuzzOrder ? formatReactionTime(team.TIME, gameState.gameTime) : null
          const hasExtra = rankBadge || reactionTime || creditEnabled
          return (
            <AnimTeamCard key={team.name} name={team.name} color={team.COLOR} score={team.SCORE || 0}>
              {hasExtra && (
                <>
                  {(rankBadge || reactionTime) && (
                    <span className="anim-team-buzz-info">
                      {rankBadge && <span className="anim-team-rank">{rankBadge}</span>}
                      {reactionTime && <span className="anim-team-reaction-time">{reactionTime}</span>}
                    </span>
                  )}
                  {creditEnabled && (
                    <button
                      className="anim-team-credit-btn"
                      onClick={() => handleCredit(team.name)}
                      title={`Créditer ${creditAmount} pts à ${team.name}`}
                    >
                      +{creditAmount} pts
                    </button>
                  )}
                </>
              )}
            </AnimTeamCard>
          )
        })}
      </div>
    </div>
  )
}
