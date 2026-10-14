// Bandeau rouge persistant « serveur sans réseau » (#96, enrichi par #267).
// Sans props il reste le bandeau d'origine ; NetworkLostOverlay lui passe, à l'état replié
// (« Continuer quand même »), la fraîcheur de la vérification et le bouton « Détails ».
// Maquette : admin_network_overlay__no_network (état 2).
import './NetworkLostOverlay.css'

// « il y a N s » / « il y a N min » depuis CHECKED_AT (ms epoch). null si la date est inconnue (0).
export function formatCheckedAgo(checkedAt, now) {
  if (!checkedAt || checkedAt <= 0) return null
  const seconds = Math.max(0, Math.floor((now - checkedAt) / 1000))
  if (seconds < 60) return `il y a ${seconds} s`
  return `il y a ${Math.floor(seconds / 60)} min`
}

export default function NetworkWarningBanner({ checkedAt = 0, now = Date.now(), onDetails, detailsRef }) {
  const ago = formatCheckedAgo(checkedAt, now)
  return (
    <div className="network-warning-banner" role="status">
      <span>
        ⚠️ Serveur sans réseau — les buzzers, les tablettes, la TV et l'animateur ne peuvent pas se connecter.
      </span>
      <span className="network-warning-spacer" />
      {ago && <small>Vérifié {ago}</small>}
      {onDetails && (
        <button ref={detailsRef} type="button" className="network-warning-details" onClick={onDetails}>
          Détails
        </button>
      )}
    </div>
  )
}
