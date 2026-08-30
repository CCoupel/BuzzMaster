import './AnimRafaleQuestion.css'

/**
 * AnimRafaleQuestion — encart question+réponse RAFALE, zone centrale (L3)
 * de `/anim` (`AnimConductPanel.jsx`), même emplacement que
 * `AnimQcmOptions`/`AnimMemoryGrid`/`AnimMotionGrid` pour les autres types.
 *
 * Retour utilisateur QUALIF 8.0.0.13 (issue #198) : "je veux que la
 * question et la réponse soient dans la zone centrale, pas dans la zone
 * question actuelle" — ce bloc vivait auparavant dans `.anim-zone-context`
 * (le bandeau du haut, réutilisé pour la méta/le chrono de TOUS les
 * types) ; il est désormais rendu ici, dans le slot central réservé aux
 * types qui ont un contenu propre à afficher (contrat rafale.md §5.1,
 * maquette rafale-v8.html §9.2 — inchangée sur le FOND, seul
 * l'EMPLACEMENT change).
 *
 * Question ET réponse attendue affichées ENSEMBLE, sans masquage
 * hold-to-peek (contrairement à `AnimAnswerZone`, utilisé par les autres
 * types) — le rythme ~3s/question ne s'y prête pas.
 *
 * @param {Object} props
 * @param {Object} [props.current] - gameState.RAFALE_CURRENT_QUESTION (sans réponse, contrat §3.3)
 * @param {string} [props.teamName] - gameState.RAFALE_CURRENT_TEAM
 * @param {string} [props.teamColorCss] - couleur CSS déjà résolue (`rgb(r,g,b)` ou repli `var(--error)`)
 * @param {string} [props.answerValue] - réponse jugée (rafaleAnswer.ANSWER — garde anti-obsolescence déjà appliquée par l'appelant)
 * @param {{icon?: string, imageURL?: string, label: string}} [props.catMeta] - categoryMeta(current.CATEGORY, ...) déjà résolu
 * @param {number} [props.askedCount] - gameState.RAFALE_ASKED_COUNT
 */
export default function AnimRafaleQuestion({
  current = {},
  teamName = '',
  teamColorCss = 'var(--error)',
  answerValue = '',
  catMeta = null,
  askedCount = 0,
}) {
  return (
    <div className="rafale-anim-qcard" style={{ '--rafale-active-color': teamColorCss }}>
      <div className="rafale-anim-qcard-meta">
        {teamName && <span className="rafale-anim-qcard-team">{teamName}</span>}
        {catMeta && (
          <span className="anim-chip">
            {catMeta.icon && <span className="anim-chip-glyph">{catMeta.icon}</span>}
            {catMeta.label}
          </span>
        )}
        {current.DIFFICULTY > 0 && <span className="anim-chip">{'★'.repeat(current.DIFFICULTY)}</span>}
        {askedCount > 0 && <span className="anim-chip">question {askedCount}</span>}
      </div>
      {current.QUESTION && <p className="rafale-anim-qcard-text">{current.QUESTION}</p>}
      {answerValue && (
        <div className="ans rafale-anim-qcard-answer">
          <b>Réponse attendue</b>
          {answerValue}
        </div>
      )}
    </div>
  )
}
