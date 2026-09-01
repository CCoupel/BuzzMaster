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
 * Zone « SUIVANTE » (#202, contrat rafale.md §13) — pourquoi `NEXT` arrive
 * par `RAFALE_ANSWER` et jamais par `GameState` : §13.1 impose un
 * **pré-tirage réel** (la question affichée ici est celle qui deviendra
 * effectivement courante au tick suivant, jamais une repioche indépendante)
 * et §13.2 interdit tout champ `GameState` pour ce texte — `SerializeForWebClient`
 * sert le même payload à `/tv`/`/player`, et connaître l'énoncé d'avance
 * serait un avantage compétitif matériel dans un mode à ~3s/question (même
 * famille de fuite que `ardoise_leak_128`). `RAFALE_ANSWER` est donc réutilisé
 * TEL QUEL (§13.3) : déjà restreint à admin+anim, déjà émis à l'instant où la
 * question devient courante — la garde anti-obsolescence côté appelant
 * (`AnimPage.jsx`, `rafaleAnswer.ID === RAFALE_CURRENT_QUESTION.ID`) protège
 * gratuitement ce nouveau champ, sans garde dédiée à écrire ici.
 *
 * @param {Object} props
 * @param {Object} [props.current] - gameState.RAFALE_CURRENT_QUESTION (sans réponse, contrat §3.3)
 * @param {string} [props.teamName] - gameState.RAFALE_CURRENT_TEAM
 * @param {string} [props.teamColorCss] - couleur CSS déjà résolue (`rgb(r,g,b)` ou repli `var(--error)`)
 * @param {string} [props.answerValue] - réponse jugée (rafaleAnswer.ANSWER — garde anti-obsolescence déjà appliquée par l'appelant)
 * @param {{icon?: string, imageURL?: string, label: string}} [props.catMeta] - categoryMeta(current.CATEGORY, ...) déjà résolu
 * @param {number} [props.askedCount] - gameState.RAFALE_ASKED_COUNT
 * @param {Object|null} [props.next] - question suivante SANS réponse (rafaleAnswer.NEXT, contrat
 *   §13.3 — `{ID, QUESTION, CATEGORY, DIFFICULTY}`), `null` = fin de réservoir (§13.5),
 *   `undefined` (valeur par défaut) = pas encore connue/périmée pour la question courante
 *   (garde anti-obsolescence déjà appliquée par l'appelant, AnimPage.jsx) — dans ce cas la
 *   zone « SUIVANTE » ne rend RIEN plutôt que d'afficher une information potentiellement
 *   fausse (même discipline que `answerValue` ci-dessus, jamais de texte tant que la donnée
 *   n'est pas confirmée pour la question affichée).
 * @param {{icon?: string, imageURL?: string, label: string}} [props.nextCatMeta] - categoryMeta(next.CATEGORY, ...) déjà résolu
 * @param {boolean} [props.showNext] - sous-phase QUESTION (contrat §13.5 dernière ligne) ;
 *   `false` = zone entièrement masquée (ex. ROUND_END, rien à préparer) ; défaut `true`
 *   pour rester rétrocompatible avec les appels existants qui ne la passent pas.
 */
export default function AnimRafaleQuestion({
  current = {},
  teamName = '',
  teamColorCss = 'var(--error)',
  answerValue = '',
  catMeta = null,
  askedCount = 0,
  next = undefined,
  nextCatMeta = null,
  showNext = true,
}) {
  const renderNext = showNext !== false && next !== undefined
  const isLastQuestion = renderNext && next === null

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
      {renderNext && (
        <div className={`rafale-anim-qcard-next${isLastQuestion ? ' rafale-anim-qcard-next-last' : ''}`}>
          <div className="rafale-anim-qcard-next-label">
            <span>Suivante</span>
            {!isLastQuestion && nextCatMeta && (
              <span className="anim-chip">
                {nextCatMeta.icon && <span className="anim-chip-glyph">{nextCatMeta.icon}</span>}
                {nextCatMeta.label}
              </span>
            )}
            {!isLastQuestion && next.DIFFICULTY > 0 && (
              <span className="anim-chip">{'★'.repeat(next.DIFFICULTY)}</span>
            )}
          </div>
          <p className="rafale-anim-qcard-next-text">
            {isLastQuestion ? 'Dernière question du réservoir' : next.QUESTION}
          </p>
        </div>
      )}
    </div>
  )
}
