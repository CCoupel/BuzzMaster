// #120 (F4) — nettoyage mutualisé de la session locale VJoueur.
//
// Avant ce fix, quatre emplacements effaçaient ces trois clés séparément et
// de façon incohérente : EnrollPage.jsx en oubliait une (`vplayer_id`), ce
// qui pouvait renvoyer un ID orphelin lors d'une tentative ultérieure et
// contribuait à la cascade "NAME_TAKEN sur son propre pseudo" (#120, cause
// racine E). Un point d'entrée unique élimine ce risque de divergence.
const VPLAYER_SESSION_KEYS = ['vplayer_name', 'vplayer_session', 'vplayer_id']

export function clearVPlayerSession() {
  VPLAYER_SESSION_KEYS.forEach((key) => localStorage.removeItem(key))
}

// #246 (F-ID) — le localStorage est PARTAGÉ par tous les onglets d'un
// navigateur, alors que la session d'un onglet vit en mémoire. Un onglet ne
// doit donc jamais effacer ni se fier au localStorage courant sans comparer
// l'ID stocké à son propre ID de session.

export function getStoredVPlayerId() {
  try {
    return localStorage.getItem('vplayer_id')
  } catch {
    return null
  }
}

// Onglet périmé : un autre onglet a enregistré une session différente
// (`vplayer_id` stocké non vide et ≠ l'ID de CET onglet). Un ID stocké absent
// n'est pas une preuve de péremption (session effacée par éviction légitime).
export function isStaleVPlayerSession(ownId) {
  if (!ownId) return false
  const stored = getStoredVPlayerId()
  return stored !== null && stored !== '' && stored !== ownId
}

// N'efface les clés `vplayer_*` que si l'ID stocké est celui de l'appelant (ou
// absent). Retourne true si l'effacement a eu lieu.
export function clearOwnVPlayerSession(ownId) {
  const stored = getStoredVPlayerId()
  // Sans ID propre connu, un ID stocké appartient forcément à un autre onglet.
  if (stored && (!ownId || stored !== ownId)) return false
  clearVPlayerSession()
  return true
}
