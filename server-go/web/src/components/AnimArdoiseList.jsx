import { getRgbColor } from '../utils/colorUtils'
import { formatArdoiseDelay } from '../utils/ardoiseOrder'
import { calcArdoiseDefaultPoints } from '../utils/pointsAward'
import AnimCreditControl from './AnimCreditControl'
import './AnimArdoiseList.css'

/**
 * AnimArdoiseList — liste des copies ARDOISE en zone équipes de `/anim`
 * (#158/F2, plan `_work/reports/plan-20260816-125123.md` §6.1). Remplace
 * `AnimTeamCard` à la même place, uniquement en mode ARDOISE (F3).
 *
 * Allégée par #170 (crédit synchronisé) : cette liste ne porte plus AUCUNE
 * logique de crédit/verrouillage — elle monte `AnimCreditControl`
 * (composant unique de #170) qui décide et rend tout seul. Trois états de
 * ligne, tous dérivés sans calcul local :
 *   - a répondu, pas encore créditée -> `AnimCreditControl` propose "+N pts"
 *     (montant via `calcArdoiseDefaultPoints`, mirror exact du bouton
 *     ARDOISE de `/admin`, `GamePage.jsx`) et "0 pt".
 *   - créditée (`awardedTeams[teamName]` présent) -> `AnimCreditControl`
 *     rend SEUL son état verrouillé (montant, origine du geste) ; monté
 *     tant que l'entrée existe, y compris hors `REVEALED`.
 *   - sans réponse -> pas de rang/délai, mention explicite ; `amount` reste
 *     `null` donc `AnimCreditControl` ne propose QUE "0 pt" (déjà géré par
 *     #170, rien à dupliquer ici).
 *
 * Geste de crédit visible seulement à partir de `REVEALED` (comme le
 * bouton ARDOISE de `/admin`, `GamePage.jsx` — gate différent de
 * SPEEDY/QCM qui autorisent aussi `STOPPED`) — sauf si la ligne est déjà
 * verrouillée, auquel cas la confirmation reste affichée.
 *
 * Copie longue : retour à la ligne (CSS `overflow-wrap`), jamais de
 * troncature.
 *
 * @param {Object} props
 * @param {Array<{team: Object, teamName: string, answer: {TEXT?: string, STARTED_AT?: number, SUBMITTED_AT?: number}|null}>} props.entries - sortArdoiseEntries (#158/F1)
 * @param {Object|null} props.question - gameState.question (ARDOISE)
 * @param {number} props.gameTime - gameState.gameTime
 * @param {number} props.creditPoints - CREDIT_POINTS courant (MAJEUR-1), repli si question.POINTS absent
 * @param {boolean} props.revealed - phase === 'REVEALED'
 * @param {Object} props.awardedTeams - awardedTeams (#170/F1), indexé par nom d'équipe
 * @param {(teamName: string, points: number) => void} props.onCredit
 */
export default function AnimArdoiseList({ entries, question, gameTime, creditPoints, revealed, awardedTeams, onCredit }) {
  return (
    <div className="anim-ardoise-list">
      {entries.map(({ team, teamName, answer }, index) => {
        const hasAnswer = !!answer
        const delayLabel = hasAnswer ? formatArdoiseDelay(answer, gameTime) : null
        const teamColor = getRgbColor(team.COLOR)
        const amount = hasAnswer ? calcArdoiseDefaultPoints(question, creditPoints) : null
        const awarded = awardedTeams[teamName]
        // Geste affiché à partir de REVEALED ; une ligne déjà verrouillée
        // reste visible même si la phase avance ensuite (confirmation
        // persistante, pas une action qui disparaît).
        const showCredit = revealed || awarded

        return (
          <div key={teamName} className={`anim-ardoise-row ${hasAnswer ? 'has-answer' : 'no-answer'}`}>
            <div className="anim-ardoise-row-header">
              {hasAnswer && <span className="anim-ardoise-rank">{index + 1}</span>}
              <span className="anim-ardoise-team-dot" style={{ background: teamColor }} />
              <span className="anim-ardoise-team-name" style={{ color: teamColor }}>{teamName}</span>
              {delayLabel && <span className="anim-ardoise-delay">{delayLabel}</span>}
              {!hasAnswer && <span className="anim-ardoise-no-answer-hint">Aucune copie</span>}
            </div>
            <div className="anim-ardoise-body">
              <span className={`anim-ardoise-text ${hasAnswer ? 'has-answer' : 'no-answer'}`}>
                {answer?.TEXT || '—'}
              </span>
              {showCredit && (
                <AnimCreditControl
                  team={teamName}
                  amount={amount}
                  awarded={awarded}
                  onCredit={(points) => onCredit(teamName, points)}
                />
              )}
            </div>
          </div>
        )
      })}
    </div>
  )
}
