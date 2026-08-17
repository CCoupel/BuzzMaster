import { motionGestures } from '../utils/motionRules'
import { getRgbColor, getContrastColor } from '../utils/colorUtils'
import './AnimMotionActions.css'

// Dispatch action WS -> handler (le champ `action` de motionRules.js est le
// nom d'action WebSocket littéral, contracts/websocket-actions.md) :
//   MEMOTION_FLIP       -> onFlipMotionCard()
//   MEMOTION_STOP_TIMER -> onStopMotionTimer()
//   MEMOTION_REVEAL     -> onRevealMotionCard()
//   MEMOTION_DONE       -> onDoneMotionCard(payload.CARD_ID, payload.WINNER_TEAM)
function dispatch(gesture, handlers) {
  if (gesture.state === 'off') return
  if (gesture.action === 'MEMOTION_FLIP') return handlers.onFlipMotionCard?.()
  if (gesture.action === 'MEMOTION_STOP_TIMER') return handlers.onStopMotionTimer?.()
  if (gesture.action === 'MEMOTION_REVEAL') return handlers.onRevealMotionCard?.()
  if (gesture.action === 'MEMOTION_DONE') return handlers.onDoneMotionCard?.(gesture.payload.CARD_ID, gesture.payload.WINNER_TEAM)
}

/**
 * AnimMotionActions — ligne L2 (« gestes propres au mode ») pendant une
 * manche MEMOTION, `/anim` (#160/F6).
 *
 * Rend EXCLUSIVEMENT ce que `utils/motionRules.js` (#160/F3) calcule —
 * aucune condition d'activation réécrite ici, même philosophie que
 * `AnimConductPanel`/L1 vis-à-vis de `phaseRules.js`. Palette de boutons
 * `anim-conduct-btn-{go|optional|danger|off}` réutilisée TELLE QUELLE
 * (AnimConductPanel.css, chargé par AnimConductPanel qui monte toujours ce
 * composant) — pas de nouvelle palette pour MEMOTION.
 *
 * `disabled` natif sur l'état `off` : un bouton éteint n'émet jamais
 * d'action, conformément au « renversement de principe » L1 (#166/#171) —
 * la ligne garde toujours le même nombre d'emplacements pour une
 * sous-phase donnée, seul l'état varie.
 *
 * En `MEMORIZE` et `GRID` (`motionGestures()` retourne `[]`), rend le
 * bandeau d'information de la maquette plutôt qu'un vide — le geste, en
 * `GRID`, c'est la carte elle-même (`AnimMotionGrid`, en L3).
 *
 * @param {Object} props
 * @param {string} props.subphase - gameState.MEMOTION_SUBPHASE
 * @param {boolean} [props.timerRunning] - gameState.timer > 0 (chrono de la carte)
 * @param {string} [props.currentTeam] - gameState.MEMOTION_CURRENT_TEAM
 * @param {Array<number>|null} [props.currentTeamColor] - gameState.MEMOTION_CURRENT_TEAM_COLOR
 * @param {string} [props.selectedCardId] - gameState.MEMOTION_SELECTED
 * @param {number} [props.cardPoints] - getMotionCardPoints(selectedCard.DIFFICULTY, question.MOTION_CONFIG)
 * @param {() => void} props.onFlipMotionCard - flipMotionCard (useGame())
 * @param {() => void} props.onStopMotionTimer - stopMotionTimer (useGame())
 * @param {() => void} props.onRevealMotionCard - revealMotionCard (useGame())
 * @param {(cardId: string, winnerTeam: string) => void} props.onDoneMotionCard - doneMotionCard (useGame())
 */
export default function AnimMotionActions({
  subphase,
  timerRunning = false,
  currentTeam = '',
  currentTeamColor = null,
  selectedCardId = '',
  cardPoints = 0,
  onFlipMotionCard,
  onStopMotionTimer,
  onRevealMotionCard,
  onDoneMotionCard,
}) {
  const handlers = { onFlipMotionCard, onStopMotionTimer, onRevealMotionCard, onDoneMotionCard }

  if (subphase === 'MEMORIZE') {
    return (
      <div className="anim-motion-actions">
        <div className="anim-motion-banner">
          <b>Mémorisation en cours.</b><br />La grille s'ouvrira automatiquement.
        </div>
      </div>
    )
  }

  if (subphase === 'GRID') {
    return (
      <div className="anim-motion-actions">
        <div className="anim-motion-banner">
          {currentTeam ? (
            <>Au tour des <b>{currentTeam}</b> — tapez la carte annoncée.</>
          ) : (
            <>Choisissez une carte.</>
          )}
        </div>
      </div>
    )
  }

  const gestures = motionGestures(subphase, {
    timerRunning,
    currentTeam,
    currentTeamColor,
    selectedCardId,
    cardPoints,
  })

  return (
    <div className="anim-motion-actions">
      {gestures.map(gesture => {
        // Bouton "équipe courante" (REVEAL) — couleur de l'équipe, texte
        // contrasté (getContrastColor), palette sémantique go/optional/off
        // sinon inchangée (aucune nouvelle couleur pour les autres gestes).
        const teamStyle = gesture.color
          ? { backgroundColor: getRgbColor(gesture.color), color: getContrastColor(gesture.color) }
          : undefined
        return (
          <button
            key={gesture.key}
            type="button"
            className={`anim-conduct-btn anim-conduct-btn-${gesture.state} anim-motion-action-btn`}
            disabled={gesture.state === 'off'}
            onClick={() => dispatch(gesture, handlers)}
            style={teamStyle}
          >
            {gesture.label}
            {gesture.subLabel && <span className="anim-conduct-btn-sub">{gesture.subLabel}</span>}
          </button>
        )
      })}
    </div>
  )
}
