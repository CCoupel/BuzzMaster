import {
  soundReplayButtonState,
  soundPauseResumeButtonState,
  soundPauseResumeCommand,
  soundStopButtonState,
} from '../utils/phaseRules'
import './AnimSoundActions.css'

/**
 * AnimSoundActions — ligne L2 : trois gestes de conduite sur le média
 * sonore attaché à la question courante (Rejouer / Pause↔Reprendre / Stop),
 * v11.1 #219, contrat websocket-actions.md §QUESTION_SOUND. Montée
 * uniquement quand `phaseRules.showSoundRow(phase, question)` est vrai
 * (AnimConductPanel.jsx/GamePage.jsx) — maquette question-sound-219.html
 * §02 : "quand la question n'a pas de son, la rangée n'est pas rendue du
 * tout — pas grisée, absente".
 *
 * États dérivés EXCLUSIVEMENT de `GAME.QUESTION_SOUND_STATE`
 * (IDLE/PLAYING/PAUSED), diffusé par le serveur — jamais un état React
 * local (R4, plan-20260922-103848.md §11 : dérive Go/JS déjà matérialisée
 * 2× sur v7.0.0). Réutilisée TELLE QUELLE par `/anim`
 * (AnimConductPanel.jsx, L2) ET `/admin` (GamePage.jsx, miroir) — même
 * discipline de mutualisation que AnimRafaleActions/AnimMotionActions.
 *
 * Palette `anim-conduct-btn-{optional|off}` réutilisée TELLE QUELLE
 * (AnimConductPanel.css) — les trois gestes sont des contrôles de conduite
 * optionnels, ni une action "attendue" (vert) ni destructive (rouge) :
 * même sémantique que le bouton PAUSE de L1.
 *
 * @param {Object} props
 * @param {string} props.soundState - gameState.QUESTION_SOUND_STATE
 * @param {(command: 'PLAY'|'PAUSE'|'RESUME'|'STOP') => void} props.onCommand - questionSound() (useGame())
 */
export default function AnimSoundActions({ soundState, onCommand }) {
  const replayState = soundReplayButtonState()
  const pauseResumeState = soundPauseResumeButtonState(soundState)
  const pauseResumeCommand = soundPauseResumeCommand(soundState)
  const stopState = soundStopButtonState(soundState)

  return (
    <div className="anim-sound-actions">
      <button
        type="button"
        className={`anim-conduct-btn anim-conduct-btn-${replayState} anim-sound-action-btn`}
        onClick={() => onCommand('PLAY')}
      >
        ↻ Rejouer
      </button>
      <button
        type="button"
        className={`anim-conduct-btn anim-conduct-btn-${pauseResumeState} anim-sound-action-btn`}
        disabled={pauseResumeState === 'off'}
        onClick={pauseResumeState === 'off' ? undefined : () => onCommand(pauseResumeCommand)}
      >
        {pauseResumeCommand === 'RESUME' ? '▶ Reprendre' : '⏸ Pause'}
      </button>
      <button
        type="button"
        className={`anim-conduct-btn anim-conduct-btn-${stopState} anim-sound-action-btn`}
        disabled={stopState === 'off'}
        onClick={stopState === 'off' ? undefined : () => onCommand('STOP')}
      >
        ⏹ Stop
      </button>
    </div>
  )
}
