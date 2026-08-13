import { useEffect, useMemo, useRef } from 'react'
import NoSleep from 'nosleep.js'
import { useGame } from '../hooks/GameContext'
import { useCategories } from '../hooks/useCategories'
import { categoryMeta } from '../utils/categoryUtils'
import { sortTeamsByBuzzOrder, getRankBadge, formatReactionTime } from '../utils/buzzOrder'
import { resolvePointsAward, resolvePointsTarget, calcQcmTeamAward } from '../utils/pointsAward'
import { QCM_COLORS } from '../constants/colors'
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
 * AnimPage — interface animateur (`/anim`, #155/#156/#157).
 *
 * Gabarit à 3 zones (plan _work/reports/plan-20260813-094321.md §4 F4) :
 *   - Zone A (contexte) : question courante, chronomètre + phase, statut de
 *     connexion, question suivante.
 *   - Zone B (conduite, #156/F5) : gestes de pilotage SPEEDY (AnimConductPanel),
 *     contextuels à la phase — jamais un bouton inactif "pour information".
 *   - Zone C (équipes, #156/F6) : AnimTeamCard enrichie (rang de buzz, temps
 *     de réaction, bouton de crédit) sans réécriture du composant — voir
 *     AnimTeamCard.jsx (point d'extension `children`). #157 (mode QCM,
 *     plan _work/reports/plan-20260813-151543.md) y ajoute la couleur de
 *     réponse choisie par équipe et rend le montant de crédit PAR ÉQUIPE
 *     (chaque équipe a sa propre pénalité d'indices) au lieu d'un montant
 *     unique — le verrou de buzz étant PAR ÉQUIPE (`engine.go:1404-1409`,
 *     tous types sauf MEMORY/MEMOTION), une équipe = un buzzer = une
 *     couleur, donc pas de vue par joueur à construire.
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
    creditPoints,
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
    // MAJEUR-1 — creditPoints (CREDIT_POINTS) est l'équivalent serveur de
    // pointsInput sur /admin, potentiellement ajusté depuis question.POINTS
    // (ex. manche bonus) : c'est la valeur à jour, pas la valeur brute de la
    // question. En pratique ce paramètre n'a aujourd'hui aucun effet côté
    // serveur (StartPayload.POINTS n'est pas décodé, cf. rapport backend
    // MAJEUR-1) mais autant transmettre la bonne valeur plutôt que réintroduire
    // le même écart que celui corrigé pour le crédit.
    startGame(time, creditPoints || 1)
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
  const isQcmWithHints = gameState.question?.TYPE === 'QCM' && gameState.question?.QCM_HINTS_ENABLED

  // Bumpers groupés par équipe — évite un filtre O(bumpers) répété par
  // équipe à chaque rendu ; consommé par le crédit QCM (T3, #157) et
  // l'affichage de la réponse QCM (T4, #157).
  const bumpersByTeam = useMemo(() => {
    const grouped = {}
    Object.entries(bumpers).forEach(([mac, bumper]) => {
      if (!bumper.TEAM) return
      if (!grouped[bumper.TEAM]) grouped[bumper.TEAM] = []
      // `mac` ajouté au bumper brut : garde ANSWER_COLOR/HINTS_AT_BUZZ/TIME
      // directement accessibles (forme attendue par calcQcmTeamAward) tout
      // en gardant l'identifiant nécessaire à setBumperPoints.
      grouped[bumper.TEAM].push({ ...bumper, mac })
    })
    return grouped
  }, [bumpers])

  // Crédit — cible (équipe/joueur) globale, POINTS_TARGET ne dépend pas de
  // l'équipe (GamePage.jsx:404-411). Base = creditPoints (CREDIT_POINTS,
  // MAJEUR-1) — l'équivalent serveur de pointsInput sur /admin, PAS
  // question.POINTS brut : /admin crédite pointsInput, potentiellement
  // ajusté après sélection (ex. manche bonus), et SET_CREDIT_POINTS/
  // CREDIT_POINTS existent précisément pour que /anim voie cet ajustement.
  const creditTarget = resolvePointsTarget(gameState.question)

  // #157/T3 — le MONTANT, lui, est par équipe en QCM avec indices activés
  // (chaque équipe a sa propre pénalité, celle de SON buzzer) : calcul
  // mutualisé par calcQcmTeamAward (#157/T1). Hors QCM (ou QCM sans
  // indices), resolvePointsAward retombe sur le montant de base pour
  // toutes les équipes — comportement inchangé par rapport à avant T3.
  const getTeamAward = (teamName) => {
    const basePoints = creditPoints || 1
    if (isQcmWithHints) {
      return calcQcmTeamAward(gameState.question, basePoints, bumpersByTeam[teamName] || [], gameState.qcmInvalidated?.length || 0)
    }
    return { amount: resolvePointsAward(gameState.question, basePoints, {}).amount, hasCorrectAnswer: null }
  }

  // En PLAYER, crédite le bumper le plus rapide de l'équipe. Correction
  // #157/T2 : le verrou de buzz n'est PAS global, il est PAR ÉQUIPE
  // (engine.go:1404-1409, "only ONE player per team can buzz") — et
  // s'applique à tous les types de question SAUF MEMORY/MEMOTION, donc à
  // QCM et ARDOISE aussi, pas seulement SPEEDY. Conséquence : une équipe a
  // au plus UN bumper avec TIME > 0, quel que soit le type — "le plus
  // rapide de l'équipe" est donc sans ambiguïté le même joueur que /admin
  // créditerait en cliquant sur son buzzer, pas une approximation propre à
  // SPEEDY.
  const handleCredit = (teamName) => {
    if (!creditEnabled) return
    const { amount } = getTeamAward(teamName)
    if (creditTarget === 'TEAM') {
      setTeamPoints(teamName, amount)
      return
    }
    const fastestBumper = (bumpersByTeam[teamName] || [])
      .filter(b => (b.TIME ?? 0) > 0)
      .sort((a, b) => a.TIME - b.TIME)[0]
    if (fastestBumper) setBumperPoints(fastestBumper.mac, amount)
  }

  // #157/T4 — réponse QCM de l'équipe (zone C) : couleur choisie + joueur,
  // dès que l'équipe a buzzé (pas de garde de phase — contrairement au
  // crédit). Marqueur de justesse séparé, gardé côté rendu à REVEALED
  // uniquement (décision D1 : l'animateur ne doit pas lire la réponse sur
  // sa tablette avant REVEALED). Rien de tout cela hors QCM.
  const getTeamQcmAnswer = (teamName) => {
    if (gameState.question?.TYPE !== 'QCM') return null
    const buzzedBumper = (bumpersByTeam[teamName] || []).find(b => (b.TIME ?? 0) > 0)
    if (!buzzedBumper?.ANSWER_COLOR) return null
    const colorInfo = QCM_COLORS[buzzedBumper.ANSWER_COLOR] || null
    return {
      colorInfo,
      playerName: buzzedBumper.NAME || '',
      isCorrect: buzzedBumper.ANSWER_COLOR === gameState.question?.QCM_CORRECT,
    }
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

      {/* Zone C — équipes (#156/F6 : ordre de buzz, rang, temps, crédit ;
          #157/T3-T4 : montant par équipe en QCM, couleur de réponse) */}
      <div className="anim-zone anim-zone-teams">
        {displayTeams.map((team, index) => {
          const rank = index + 1
          const rankBadge = showRankBadge ? getRankBadge(rank) : null
          const reactionTime = showBuzzOrder ? formatReactionTime(team.TIME, gameState.gameTime) : null
          const qcmAnswer = getTeamQcmAnswer(team.name)
          const teamCreditAmount = creditEnabled ? getTeamAward(team.name).amount : null
          const hasExtra = rankBadge || reactionTime || qcmAnswer || creditEnabled
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
                  {/* #157/T4 — couleur choisie dès le buzz ; justesse (✓/✗)
                      uniquement en REVEALED (décision D1, plan §4) — rien de
                      tout cela hors QCM (qcmAnswer est null hors QCM ou tant
                      que l'équipe n'a pas buzzé). */}
                  {qcmAnswer && (
                    <span className="anim-team-qcm-answer">
                      {qcmAnswer.colorInfo && (
                        <span
                          className="anim-team-qcm-color"
                          style={{ backgroundColor: qcmAnswer.colorInfo.color }}
                          title={qcmAnswer.colorInfo.label}
                        >
                          {qcmAnswer.colorInfo.letter}
                        </span>
                      )}
                      {qcmAnswer.playerName && (
                        <span className="anim-team-qcm-player">{qcmAnswer.playerName}</span>
                      )}
                      {gameState.phase === 'REVEALED' && (
                        <span className={`anim-team-qcm-correct ${qcmAnswer.isCorrect ? 'correct' : 'incorrect'}`}>
                          {qcmAnswer.isCorrect ? '✓' : '✗'}
                        </span>
                      )}
                    </span>
                  )}
                  {creditEnabled && (
                    <button
                      className="anim-team-credit-btn"
                      onClick={() => handleCredit(team.name)}
                      title={`Créditer ${teamCreditAmount} pts à ${team.name}`}
                    >
                      +{teamCreditAmount} pts
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
