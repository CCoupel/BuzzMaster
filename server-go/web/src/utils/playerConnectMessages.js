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
  // #122 (F2) — le nom est porté par un VJoueur DÉCONNECTÉ (cand.Connected
  // == false côté serveur, engine.go cas 3) : probablement le joueur
  // lui-même, ayant perdu son ID (stockage vidé, changement d'appareil...),
  // pas un homonyme. NAME_TAKEN reste inchangé pour le cas connecté — ne
  // jamais le modifier, verrouillé par les tests de non-régression #109.
  // Texte validé au GATE 2 (#122) — voir _work/mockups/122-name-recovery.html.
  NAME_TAKEN_OFFLINE: 'Cette place est peut-être la tienne. Demande à l’animateur de te la rendre, puis réessaie — tu retrouveras ton score.',
}

export const DEFAULT_REJECTION_MESSAGE = 'Connexion refusée, réessaie avec un autre pseudo'

// #120 — motifs de RENVOI vers l'inscription (PLAYER_EVICTED, ou le filet de
// sécurité local SESSION_EXPIRED). Distinct de REJECTION_MESSAGES ci-dessus :
// celui-ci couvre un refus à la SOUMISSION (verrouillé par les tests de
// non-régression #109, ne jamais le modifier pour ce ticket) ; celui-ci
// couvre le cas où le joueur, déjà accepté, est renvoyé en cours de partie.
// Textes validés au GATE 2 (#120) — voir _work/mockups/120-enroll-redirect-message.html.
export const REDIRECT_MESSAGES = {
  PLAYER_REMOVED: 'Ta place a été libérée par l’animateur. Tu peux te réinscrire.',
  GAME_RESET: 'Une nouvelle partie a commencé. Tous les joueurs doivent se réinscrire.',
  SESSION_EXPIRED: 'Ta session n’est plus valide. Réinscris-toi pour continuer.',
}

export const DEFAULT_REDIRECT_MESSAGE = 'Tu as été renvoyé à l’inscription. Réinscris-toi pour continuer.'
