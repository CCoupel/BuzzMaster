import './TeamAwardBar.css'

/**
 * TeamAwardBar — barre d'attribution groupée (#244/#251, maquette validée
 * admin_game_team_award__selection) : montant par équipe (−, saisie, +), source du
 * montant, « Tout sélectionner / Tout désélectionner » et bouton « Donner les points (N) »
 * (« Retirer les points (N) », rouge, si le montant est négatif). Désactivé à N = 0 et
 * pendant l'envoi. Visible seulement après le REVEAL (décidé par l'appelant).
 */
export default function TeamAwardBar({
  amount, source, count, sending, blockedReason,
  onAmountChange, onSelectAll, onSelectNone, onGive,
}) {
  const negative = amount < 0
  const setAmount = (value) => {
    const n = Number.parseInt(value, 10)
    onAmountChange(Number.isFinite(n) ? n : 0)
  }
  return (
    <div className="award-bar" role="group" aria-label="Attribution des points aux équipes sélectionnées">
      <span className="award-bar-label">Points par équipe</span>
      <div className={`award-amount${negative ? ' neg' : ''}`}>
        <button type="button" onClick={() => onAmountChange(amount - 1)} aria-label="Diminuer">−</button>
        <input
          type="number"
          value={amount}
          onChange={(e) => setAmount(e.target.value)}
          aria-label="Points par équipe"
        />
        <button type="button" onClick={() => onAmountChange(amount + 1)} aria-label="Augmenter">+</button>
      </div>
      <span className="award-src">{source}</span>
      <span className="award-sel-links">
        <button type="button" className="award-link" onClick={onSelectAll}>Tout sélectionner</button>
        <button type="button" className="award-link" onClick={onSelectNone}>Tout désélectionner</button>
      </span>
      <button
        type="button"
        className={`award-give${negative ? ' neg' : ''}`}
        disabled={count === 0 || sending || !!blockedReason}
        title={blockedReason || undefined}
        onClick={onGive}
      >
        {negative ? 'Retirer les points' : 'Donner les points'} ({count})
      </button>
    </div>
  )
}
