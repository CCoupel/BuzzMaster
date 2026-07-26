// Fix R1 (#109) — messages affichés selon la raison de rejet renvoyée par le
// serveur (PLAYER_REJECTED.REASON). Partagé entre EnrollPage (premier
// enrôlement) et VPlayerPage (reconnexion) pour éviter la duplication.
// NAME_TAKEN = motif introduit par le fix ID (lookup par ID, voir
// contracts/websocket-actions.md).
export const REJECTION_MESSAGES = {
  NAME_TAKEN: 'Ce pseudo est déjà utilisé, choisis-en un autre',
  INVALID_NAME: 'Pseudo invalide, choisis-en un autre',
  ENROLLMENT_CLOSED: 'Les inscriptions sont fermées',
  LIMIT_REACHED: 'Nombre maximum de joueurs atteint',
}

export const DEFAULT_REJECTION_MESSAGE = 'Connexion refusée, réessaie avec un autre pseudo'
