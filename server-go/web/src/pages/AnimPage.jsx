import { useEffect, useMemo, useRef } from 'react'
import NoSleep from 'nosleep.js'
import { useGame } from '../hooks/GameContext'
import { useCategories } from '../hooks/useCategories'
import { categoryMeta } from '../utils/categoryUtils'
import Timer from '../components/Timer'
import CategoryBadge from '../components/CategoryBadge'
import AnimTeamCard from '../components/AnimTeamCard'
import './AnimPage.css'

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
 *   - Zone B (conduite) : conteneur posé, vide ici — contenu livré par
 *     #156/F5 (les gestes de pilotage SPEEDY, contextuels à la phase).
 *   - Zone C (équipes) : carte d'équipe de base (AnimTeamCard, F4). Enrichie
 *     par #156/F6 (rang de buzz, temps de réaction, bouton de crédit) sans
 *     réécriture — voir AnimTeamCard.jsx.
 *
 * Tablette paysage, pas de Navbar régie (App.jsx ne l'affiche que sur
 * /admin/* désormais — #155/F2). Connecté sur /ws/anim (ClientTypeAnim,
 * capacité réduite) via GameProvider (App.jsx).
 */
export default function AnimPage() {
  const { status, gameState, teams, bumpers, nextQuestion } = useGame()

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

  // Zone C — mêmes équipes que /admin (au moins un joueur assigné — règle de
  // base #45, GamePage.jsx:135). Le tri par ordre de buzz et l'enrichissement
  // arrivent avec #156/F6 ; ici uniquement l'ordre courant des équipes.
  const displayTeams = useMemo(() => {
    const teamsWithPlayers = new Set(
      Object.values(bumpers)
        .filter(b => b.TEAM)
        .map(b => b.TEAM)
    )
    return Object.entries(teams)
      .filter(([name]) => teamsWithPlayers.has(name))
      .map(([name, data]) => ({ name, ...data }))
  }, [teams, bumpers])

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
          <span className="anim-next-question-label">À suivre</span>
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

      {/* Zone B — conduite (contenu livré par #156/F5) */}
      <div className="anim-zone anim-zone-conduct" />

      {/* Zone C — équipes */}
      <div className="anim-zone anim-zone-teams">
        {displayTeams.map(team => (
          <AnimTeamCard key={team.name} name={team.name} color={team.COLOR} score={team.SCORE || 0} />
        ))}
      </div>
    </div>
  )
}
