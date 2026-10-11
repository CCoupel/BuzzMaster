import { nextButtonState } from '../utils/phaseRules'
import { formatNextQuestionStatement, formatNextQuestionMeta } from '../utils/nextQuestionFormat'
import './AnimNextButton.css'

/**
 * AnimNextButton — bouton "à suivre" de la conduite `/anim` (#166/F4).
 *
 * Extrait d'`AnimConductPanel.jsx` (où il vivait conditionnellement,
 * #163/#165) puis déplacé en conduite, juste après L1 : "à suivre" émet
 * `READY`, un geste global autorisé dans tous les modes au même titre que
 * les cinq de L1 (plan §"Position de « À suivre »"). Déplacement et
 * extraction — jamais réécriture : format toujours via
 * `nextQuestionFormat.js`, couleurs toujours celles de #165.
 *
 * Trois états, dérivés de `phaseRules.nextButtonState(phase, question)` —
 * 'go' / 'optional' / 'inert' — SAUF cas limite "fin du quiz"
 * (`!nextQuestion`) : ce n'est pas une règle de PHASE (phaseRules ne
 * connaît pas les données de question), donc forcé ici à 'inert' avec un
 * libellé dédié plutôt que réécrit dans phaseRules.js.
 *
 * `question` (la question COURANTE, distincte de `nextQuestion`) est
 * nécessaire pour distinguer STOPPED "jouée" (RÉPONSE dispo à côté ->
 * bleu) de STOPPED "non jouée" (seule action possible -> vert) — même
 * `phase`, deux lignes différentes de la matrice, seul `question.STATUS`
 * (via `canReveal`) les sépare.
 *
 * @param {Object} props
 * @param {{ID?: string}|null} [props.nextQuestion] - dernier NEXT_QUESTION reçu
 * @param {string} props.phase - gameState.phase
 * @param {{STATUS?: string}|null} [props.question] - gameState.question (question COURANTE)
 * @param {(questionId: string) => void} props.onSelectNext
 */
export default function AnimNextButton({ nextQuestion, phase, question, onSelectNext, entracteActive = false, onEndEntracte }) {
  const hasNext = !!nextQuestion?.ID
  const state = hasNext ? nextButtonState(phase, question) : 'inert'

  // #245 (maquette (g‴)) — pendant une entracte, le grand bouton devient « Terminer l'entracte »
  // (ENTRACTE_SET{ACTIVE:false}, permis à l'anim, y compris pendant le verrou de lancement) : il met fin au panneau
  // ET à l'entrée, puis « suivant » reprend. L'anim ne DÉCLENCHE jamais une entracte manuelle.
  if (entracteActive) {
    return (
      <button type="button" className="anim-next-btn anim-next-btn-endent" onClick={() => onEndEntracte?.()}>
        <span className="anim-next-btn-label">Terminer l'entracte</span>
        <span className="anim-next-btn-body">
          <span className="anim-next-btn-statement">
            {question?.QUESTION ? `🎬 ${question.QUESTION} — ` : ''}retour au déroulé
          </span>
        </span>
      </button>
    )
  }

  // #245 — la prochaine entrée est une ENTRACTE (proposée comme toute entrée) ou ouvre une autre manche.
  const nextIsEntracte = hasNext && nextQuestion.TYPE === 'ENTRACTE'
  const nextIsNewRound = hasNext && !nextIsEntracte && nextQuestion.NEW_ROUND === true
  const variant = state === 'inert' ? '' : nextIsEntracte ? ' anim-next-btn-entracte' : nextIsNewRound ? ' anim-next-btn-newround' : ''
  const label = nextIsEntracte ? "Lancer l'entracte"
    : nextIsNewRound ? `Manche suivante : ${nextQuestion.ROUND_NAME || 'Sans manche'}`
    : 'À suivre'

  const handleClick = () => {
    if (state === 'inert') return
    onSelectNext(nextQuestion.ID)
  }

  return (
    <button
      className={`anim-next-btn anim-next-btn-${state}${variant}`}
      disabled={state === 'inert'}
      onClick={handleClick}
    >
      <span className="anim-next-btn-label">{label}</span>
      <span className="anim-next-btn-body">
        <span className="anim-next-btn-statement">
          {!hasNext ? 'Fin du quiz' : nextIsEntracte ? `🎬 ${nextQuestion.QUESTION || 'Entracte'}` : formatNextQuestionStatement(nextQuestion)}
        </span>
        {hasNext && !nextIsEntracte && (
          <span className="anim-next-btn-meta">{formatNextQuestionMeta(nextQuestion)}</span>
        )}
      </span>
    </button>
  )
}
