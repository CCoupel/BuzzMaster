import Timer from './Timer'
import './RafaleTimers.css'

/**
 * RafaleTimers — double timer du mode RAFALE (contrat rafale.md §2.2/§4).
 *
 * Deux tickers tournent simultanément en RAFALE : le timer de MANCHE
 * (réutilise le timer global existant — CURRENT_TIME/totalTime, UPDATE_TIMER)
 * et le timer de QUESTION (~3s, RAFALE_QUESTION_TIME, action RAFALE_TICK).
 * Ce composant ne recalcule rien — il monte `Timer.jsx` en DEUX instances
 * (mono-valeur, non modifié) plutôt que de le rendre multi-valeur.
 *
 * @param {number} roundTime - temps restant de la manche (gameState.timer)
 * @param {number} roundTotal - durée totale de la manche (gameState.totalTime)
 * @param {number} questionTime - temps restant de la question courante (gameState.RAFALE_QUESTION_TIME)
 * @param {number} questionTotal - durée totale de la question (question.RAFALE_QUESTION_TIME, défaut 3)
 * @param {string} phase - gameState.phase, transmis tel quel aux deux Timer
 * @param {'sm'|'md'|'lg'|'xl'} size - taille des deux Timer (identique)
 * @param {boolean} showBar - transmis aux deux Timer
 * @param {string} className
 */
export default function RafaleTimers({
  roundTime = 0,
  roundTotal = 0,
  questionTime = 0,
  questionTotal = 0,
  phase = 'STOPPED',
  size = 'md',
  showBar = true,
  className = '',
}) {
  return (
    <div className={`rafale-timers ${className}`}>
      <div className="rafale-timer rafale-timer-round">
        <span className="rafale-timer-label">Manche</span>
        <Timer
          currentTime={roundTime}
          totalTime={roundTotal}
          phase={phase}
          size={size}
          showBar={showBar}
          showPhase={false}
        />
      </div>
      <div className="rafale-timer rafale-timer-question">
        <span className="rafale-timer-label">Question</span>
        <Timer
          currentTime={questionTime}
          totalTime={questionTotal}
          phase={phase}
          size={size}
          showBar={showBar}
          showPhase={false}
        />
      </div>
    </div>
  )
}
