import './ConnectionBadge.css'

// Machine d'état connexion (4 états, CONN_STATE backend — voir contracts/models.md) :
// ""       → HIDDEN  : rien à afficher (connecté, aucun souci)
// "orange" → déconnecté (buzzer/VJoueur), aucune perte de message confirmée
// "red"    → déconnecté avec au moins un message perdu pendant la coupure
// "green"  → reconnecté, fenêtre de confirmation (≥2s) avant de repasser HIDDEN
const STATE_META = {
  orange: { title: 'Déconnecté' },
  red: { title: 'Déconnecté — message(s) perdu(s)' },
  green: { title: 'Reconnecté' },
}

// Icône "connexion perdue" (wifi/plug barré) — reprend le pictogramme utilisé
// historiquement dans TeamCard/TeamsPage pour orange et rouge.
function DisconnectedIcon() {
  return (
    <svg viewBox="0 0 24 24" width="11" height="11" fill="none" stroke="white" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
      <line x1="1" y1="1" x2="23" y2="23" />
      <path d="M16.72 11.06A10.94 10.94 0 0 1 19 12.55" />
      <path d="M5 12.55a10.94 10.94 0 0 1 5.17-2.39" />
      <path d="M10.71 5.05A16 16 0 0 1 22.56 9" />
      <path d="M1.42 9a15.91 15.91 0 0 1 4.7-2.88" />
      <path d="M8.53 16.11a6 6 0 0 1 6.95 0" />
      <line x1="12" y1="20" x2="12.01" y2="20" />
    </svg>
  )
}

// Icône "reconnecté" (check) — état vert transitoire (D2/D3, ≥2s).
function ReconnectedIcon() {
  return (
    <svg viewBox="0 0 24 24" width="11" height="11" fill="none" stroke="white" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
      <polyline points="20 6 9 17 4 12" />
    </svg>
  )
}

/**
 * Badge de connexion mutualisé — unique source de vérité pour l'affichage
 * de l'état de connexion d'un bumper (buzzer physique ou VJoueur) sur les
 * pages admin (GamePage, TeamsPage). Remplace les 3 implémentations SVG
 * dupliquées (#109).
 *
 * @param {string} state - Valeur brute de `Bumper.CONN_STATE` : ""/"orange"/"red"/"green".
 *                          Toute valeur absente ou inconnue est traitée comme HIDDEN (rien de rendu).
 * @param {string} className - Classes additionnelles optionnelles.
 */
export default function ConnectionBadge({ state, className = '' }) {
  const meta = STATE_META[state]
  if (!meta) return null

  return (
    <span
      className={`connection-badge connection-badge-${state}${className ? ` ${className}` : ''}`}
      title={meta.title}
    >
      {state === 'green' ? <ReconnectedIcon /> : <DisconnectedIcon />}
    </span>
  )
}
