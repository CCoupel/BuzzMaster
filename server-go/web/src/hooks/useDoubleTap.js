import { useRef } from 'react'

const DEFAULT_DELAY_MS = 300
const DEFAULT_MOVE_TOLERANCE_PX = 10

/**
 * useDoubleTap — geste discret et compté (#176), distinct de
 * `useHoldToPeek` (geste MAINTENU) : deux pièces différentes plutôt qu'un
 * hook à deux modes qui ne partageraient aucune logique (décision ③ du plan
 * #176, `_work/reports/plan-20260818-141638.md`).
 *
 * Utilise les **Pointer Events** (`onPointerDown`/`onPointerUp`), jamais
 * `onClick`/`onDoubleClick` : `dblclick` est inégalement supporté sur
 * tactile et entre en conflit avec le double-tap-zoom natif du navigateur
 * (neutralisé côté CSS par l'appelant via `touch-action: manipulation`,
 * indispensable pour que ce geste reste utilisable sur tablette).
 *
 * @param {() => void} onDoubleTap - appelé quand deux appuis successifs
 *   tombent dans la fenêtre de temps, sans dépassement de la tolérance de
 *   déplacement.
 * @param {Object} [options]
 * @param {number} [options.delay=300] - fenêtre maximale (ms) entre les deux
 *   appuis pour qu'ils comptent comme un double-tap.
 * @param {number} [options.moveTolerance=10] - déplacement maximal (px,
 *   distance euclidienne) toléré entre le pointerdown et le pointerup d'un
 *   même appui — un glissement au-delà n'est pas compté comme un tap.
 * @returns {{ handlers: { onPointerDown: Function, onPointerUp: Function } }}
 */
export default function useDoubleTap(onDoubleTap, { delay = DEFAULT_DELAY_MS, moveTolerance = DEFAULT_MOVE_TOLERANCE_PX } = {}) {
  // Position du pointerdown courant — sert à mesurer le déplacement au
  // pointerup (AC15, un glissement n'est pas un tap).
  const downPosRef = useRef(null)
  // Horodatage du DERNIER tap valide (pointerup sans glissement excessif) —
  // `null` tant qu'aucun premier tap n'est en attente d'un second.
  const lastTapAtRef = useRef(null)

  const onPointerDown = (e) => {
    downPosRef.current = { x: e.clientX, y: e.clientY }
  }

  const onPointerUp = (e) => {
    const downPos = downPosRef.current
    downPosRef.current = null
    if (!downPos) return

    const dx = e.clientX - downPos.x
    const dy = e.clientY - downPos.y
    const distance = Math.sqrt(dx * dx + dy * dy)
    if (distance > moveTolerance) {
      // Glissement : ni un tap valide, ni un point de départ pour le
      // prochain — repart de zéro (AC15).
      lastTapAtRef.current = null
      return
    }

    const now = Date.now()
    const lastTapAt = lastTapAtRef.current
    if (lastTapAt !== null && now - lastTapAt <= delay) {
      // Second tap dans la fenêtre — déclenche et réinitialise (AC12).
      lastTapAtRef.current = null
      onDoubleTap?.()
      return
    }

    // Premier tap (ou précédent hors fenêtre, AC14) — devient le point de
    // départ du prochain.
    lastTapAtRef.current = now
  }

  return { handlers: { onPointerDown, onPointerUp } }
}
