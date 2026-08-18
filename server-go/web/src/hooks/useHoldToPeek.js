import { useEffect, useState } from 'react'

/**
 * useHoldToPeek — geste de révélation par pression maintenue (#169),
 * extrait de `AnimAnswerZone.jsx` en pièce réutilisable (#168/F6) pour être
 * partagé avec `AnimExplanationNote.jsx` (note d'explication, même geste
 * EXACT — pas une seconde implémentation qui pourrait diverger).
 *
 * Règle : avant `revealed`, le contenu est masqué par défaut et révélé tant
 * qu'un pointeur (doigt ou souris — Pointer Events) est MAINTENU sur la
 * zone ; relâché ou sorti de la zone -> remasqué. Une fois `revealed`, reste
 * visible en PERMANENCE, sans interaction (les handlers deviennent des
 * no-op) — l'animateur ne doit pas garder le doigt appuyé pendant qu'il
 * crédite les équipes.
 *
 * @param {boolean} revealed - une fois true, `visible` reste true en
 *   permanence et les handlers de pointeur n'ont plus aucun effet.
 * @param {*} resetKey - valeur dont un changement réinitialise `peeking` à
 *   false (garde-fou si un `pointerup` est manqué — ex. changement de
 *   question pendant une pression). Typiquement `question?.ID`.
 * @returns {{ visible: boolean, handlers: Object }} `visible` = revealed ||
 *   peeking ; `handlers` = les 4 gestionnaires Pointer Events à répandre
 *   (spread) sur l'élément qui porte le geste.
 */
export default function useHoldToPeek(revealed, resetKey) {
  const [peeking, setPeeking] = useState(false)

  // Garde-fou : un changement de resetKey en cours de pression (ex. STOP
  // puis enchaînement) ne doit jamais laisser le contenu de l'élément
  // SUIVANT déjà visible avant même le prochain pointerdown.
  useEffect(() => {
    setPeeking(false)
  }, [resetKey])

  const visible = revealed || peeking

  // No-op une fois `revealed` : le contenu reste figé visible, aucune
  // interaction n'est plus nécessaire ni souhaitée à ce stade.
  const startPeek = () => { if (!revealed) setPeeking(true) }
  const endPeek = () => { if (!revealed) setPeeking(false) }

  const handlers = {
    onPointerDown: startPeek,
    onPointerUp: endPeek,
    onPointerLeave: endPeek,
    onPointerCancel: endPeek,
  }

  return { visible, handlers }
}
