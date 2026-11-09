// #290 (LOT-4, review m1) — un geste direct (POST /config.json son/décompte/cues, POST /api/lighting/mode) dont la
// réponse porte `scope: "round"` n'a PAS modifié le global : la manche impose ce réglage (ambiance-settings.md §4.2).
export const ROUND_SCOPE_MESSAGE = 'Appliqué à la manche en cours seulement (la manche impose ce réglage).'

/** Vrai si la réponse `res` (non consommée) annonce `scope: "round"`. Jamais d'exception. */
export async function isRoundScope(res) {
  try {
    const body = await res.clone().json()
    return body?.scope === 'round'
  } catch {
    return false
  }
}
