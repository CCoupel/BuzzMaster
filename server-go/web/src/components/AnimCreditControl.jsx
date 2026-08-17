import './AnimCreditControl.css'

/**
 * AnimCreditControl — composant de crédit UNIQUE (#170/F2), consommé par
 * tous les points de crédit de `/anim` (carte d'équipe SPEEDY/QCM,
 * #156/#157 ; future liste ARDOISE, #158 ; MEMORY/MEMOTION à venir).
 * Responsabilité complète : décide ET rend.
 *
 * - `awarded` absent (`undefined`) -> deux gestes disponibles :
 *   "+N pts" (`amount`, fourni par l'appelant via `pointsAward.js` —
 *   JAMAIS recalculé ici) et "0 pt" (refus, même chemin de crédit,
 *   montant nul, #170/F4). Si `amount` est `null`/`undefined` (ligne sans
 *   réponse), seul "0 pt" est proposé — rien à créditer.
 * - `awarded` présent -> état VERROUILLÉ, montant affiché, aucune action.
 *   C'est aussi la confirmation du crédit (#170/F5) : l'arrivée de
 *   l'entrée dans `awardedTeams` EST l'événement annoncé, pas de mécanique
 *   de toast séparée.
 *
 * ⚠️ Le verrouillage vient EXCLUSIVEMENT de la PRÉSENCE de `awarded`
 * (`awardedTeams[teamName]`, alimenté par le serveur — #170/F1), jamais
 * d'une anticipation locale du clic. Test explicite : `if (awarded)`,
 * JAMAIS `if (awarded?.POINTS)` — un refus à 0 point (`awarded.POINTS ===
 * 0`) est un verrou tout aussi valide qu'un crédit à 20 points. Un
 * `if (points)` classique déverrouillerait silencieusement toutes les
 * lignes refusées : exactement le bug que ce composant doit éviter
 * (plan `_work/reports/plan-20260816-125123.md`, risque R1).
 *
 * L'état verrouillé affiche le libellé du GESTE réellement effectué,
 * pas un montant brut : `"+N pts"` (N > 0, même style que le bouton
 * "+N pts") ou littéralement `"0 pt"` (refus, même texte que le bouton
 * "0 pt") — jamais `"+0 pts"`, qui se lirait comme un crédit plutôt
 * qu'un refus. C'est l'« origine » demandée par le plan (T5) : quel des
 * deux gestes a verrouillé la ligne, lisible directement depuis
 * `awarded.POINTS` — aucun champ d'identité de tablette/client n'existe
 * dans le payload (contracts/websocket-actions.md §AWARDED_TEAMS), donc
 * aucune tentative d'en afficher un ici.
 *
 * @param {Object} props
 * @param {string} [props.team] - nom d'équipe, uniquement pour l'attribut title (accessibilité)
 * @param {number|null} [props.amount] - montant du geste "+N pts" ; absent/null -> seul "0 pt"
 * @param {{POINTS: number, TIMESTAMP: number}|undefined} props.awarded - awardedTeams[teamName]
 * @param {(points: number) => void} props.onCredit - émet le crédit (même chemin pour +N pts et 0 pt)
 */
export default function AnimCreditControl({ team, amount, awarded, onCredit }) {
  if (awarded) {
    const label = awarded.POINTS > 0 ? `+${awarded.POINTS} pts` : '0 pt'
    return (
      <span
        className="anim-credit-control anim-credit-control-locked"
        title={team ? `${team} — déjà créditée (${label})` : `Déjà créditée (${label})`}
      >
        <span className="anim-credit-control-check">✓</span>
        <span className="anim-credit-control-amount">{label}</span>
      </span>
    )
  }

  return (
    <span className="anim-credit-control">
      {amount != null && (
        <button
          className="anim-credit-control-btn anim-credit-control-btn-award"
          onClick={() => onCredit(amount)}
          title={team ? `Créditer ${amount} pts à ${team}` : `Créditer ${amount} pts`}
        >
          +{amount} pts
        </button>
      )}
      <button
        className="anim-credit-control-btn anim-credit-control-btn-zero"
        onClick={() => onCredit(0)}
        title={team ? `Refuser (0 pt) pour ${team}` : 'Refuser (0 pt)'}
      >
        0 pt
      </button>
    </span>
  )
}
