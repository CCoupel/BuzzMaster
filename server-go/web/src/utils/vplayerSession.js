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
