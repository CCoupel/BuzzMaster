import './AnimConductPanel.css'

/**
 * AnimConductPanel — zone de conduite SPEEDY de la page animateur
 * (`/anim` zone B, #156/F5).
 *
 * Contextuel à la phase : n'affiche JAMAIS un geste inactif "pour
 * information" (règle d'ergonomie du cadrage, maquette §02) — contrairement
 * à `/admin` qui affiche les boutons START/PAUSE/REPONSE en permanence,
 * désactivés selon la phase. Les CONDITIONS d'activation sont les mêmes que
 * `/admin` (`GamePage.jsx:373-378` — `isPlaying`/`canReveal`/`canStart`),
 * seule la PRÉSENTATION change (plan §4 F5).
 *
 * | Phase                          | Geste affiché                        |
 * |---------------------------------|---------------------------------------|
 * | PREPARE                         | attente des PONG, aucun geste         |
 * | STARTED / PAUSED (isPlaying)    | PAUSE/CONTINUER + STOP, parts égales  |
 * | STOPPED après jeu (canReveal)   | RÉPONSE seule                         |
 * | READY                           | LANCER + enchaînement (si dispo)      |
 * | STOPPED idle / NEW_GAME/REVEALED| enchaînement seul (si dispo)          |
 *
 * @param {string} phase - gameState.phase
 * @param {boolean} isPlaying - STARTED || PAUSED (même calcul que GamePage.jsx:373)
 * @param {boolean} canStart - phase === READY (GamePage.jsx:378)
 * @param {boolean} canReveal - STOPPED && question.STATUS === 'STOPPED' (GamePage.jsx:376)
 * @param {{ID?: string}|null} nextQuestion - dernier NEXT_QUESTION reçu
 * @param {() => void} onStart
 * @param {() => void} onPause
 * @param {() => void} onContinue
 * @param {() => void} onStop
 * @param {() => void} onReveal
 * @param {(questionId: string) => void} onSelectNext
 */
export default function AnimConductPanel({
  phase,
  isPlaying,
  canStart,
  canReveal,
  nextQuestion,
  onStart,
  onPause,
  onContinue,
  onStop,
  onReveal,
  onSelectNext,
}) {
  const hasNext = !!nextQuestion?.ID

  if (phase === 'PREPARE') {
    return (
      <div className="anim-conduct anim-conduct-waiting">
        <span className="anim-conduct-waiting-text">En attente des joueurs…</span>
      </div>
    )
  }

  if (isPlaying) {
    const isPaused = phase === 'PAUSED'
    return (
      <div className="anim-conduct anim-conduct-pair">
        <button
          className="anim-conduct-btn anim-conduct-btn-pause"
          onClick={isPaused ? onContinue : onPause}
        >
          {isPaused ? 'CONTINUER' : 'PAUSE'}
        </button>
        <button className="anim-conduct-btn anim-conduct-btn-stop" onClick={onStop}>
          STOP
        </button>
      </div>
    )
  }

  if (canReveal) {
    return (
      <div className="anim-conduct anim-conduct-single">
        <button className="anim-conduct-btn anim-conduct-btn-reveal" onClick={onReveal}>
          RÉPONSE
        </button>
      </div>
    )
  }

  // STOPPED (idle, pas encore jouée) / NEW_GAME / REVEALED / READY
  return (
    <div className="anim-conduct anim-conduct-idle">
      {canStart && (
        <button className="anim-conduct-btn anim-conduct-btn-start" onClick={onStart}>
          LANCER
        </button>
      )}
      {hasNext && (
        <button
          className="anim-conduct-btn anim-conduct-btn-next"
          onClick={() => onSelectNext(nextQuestion.ID)}
        >
          <span className="anim-conduct-next-label">À suivre</span>
          <span className="anim-conduct-next-id">#{nextQuestion.ID}</span>
        </button>
      )}
      {!canStart && !hasNext && (
        <span className="anim-conduct-empty">Aucune question disponible</span>
      )}
    </div>
  )
}
