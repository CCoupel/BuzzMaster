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
  clearArdoiseDraft()
}

// #246 — texte ARDOISE en cours, conservé dans le sessionStorage de l'ONGLET
// (jamais partagé entre onglets) pour survivre à un rechargement de la page
// (pull-to-refresh) : sinon la page remontée enverrait un ARDOISE_FINAL "" qui
// écraserait la réponse que le serveur détient encore. Emplacement unique
// { id, qid, text } : bumper + question. Toutes les opérations tolèrent un
// sessionStorage indisponible (navigation privée, quota) : comportement dégradé.
const ARDOISE_DRAFT_KEY = 'vplayer_ardoise_draft'

export function saveArdoiseDraft(id, qid, text) {
  try {
    if (!id || qid == null) return
    if (text) sessionStorage.setItem(ARDOISE_DRAFT_KEY, JSON.stringify({ id, qid: String(qid), text }))
    else sessionStorage.removeItem(ARDOISE_DRAFT_KEY)
  } catch { /* dégradé */ }
}

// Retourne le texte conservé pour CE bumper et CETTE question, sinon ''. Un
// brouillon d'une autre question est supprimé au passage.
export function loadArdoiseDraft(id, qid) {
  try {
    const raw = sessionStorage.getItem(ARDOISE_DRAFT_KEY)
    if (!raw) return ''
    const d = JSON.parse(raw)
    if (d && d.id === id && d.qid === String(qid) && typeof d.text === 'string') return d.text
    sessionStorage.removeItem(ARDOISE_DRAFT_KEY)
  } catch { /* dégradé */ }
  return ''
}

export function clearArdoiseDraft() {
  try { sessionStorage.removeItem(ARDOISE_DRAFT_KEY) } catch { /* dégradé */ }
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
