import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import useDoubleTap from './useDoubleTap'

// ---------------------------------------------------------------------------
// useDoubleTap — geste de double-tap compté (v6.4.x, #176 F4).
//
// Plan : _work/reports/plan-20260818-141638.md, tâche F4, décision ③ (pas de
// réutilisation de useHoldToPeek — un geste MAINTENU et un geste DISCRET
// compté ne partagent aucune logique). Contrat : aucun (front uniquement).
//
// Signature : useDoubleTap(onDoubleTap, { delay = 300, moveTolerance = 10 })
// -> { handlers } — handlers = { onPointerDown, onPointerUp } à répandre sur
// l'élément qui porte le geste (Pointer Events, PAS onClick/onDoubleClick —
// dblclick est inégalement supporté sur tactile, voir F4).
//
// T6 — deux taps dans la fenêtre déclenchent ; un seul tap ne déclenche
// rien ; deux taps hors fenêtre (>300ms) ne comptent pas ; un glissement
// (> tolérance) entre down et up n'est pas compté comme un tap (AC12-AC15).
//
// vi.useFakeTimers() fake aussi Date/performance.now par défaut (sinon
// fake-timers) — utilisé ici pour avancer le temps ENTRE deux taps, que
// l'implémentation compare des timestamps directement ou passe par un
// timer interne : les deux cas sont couverts sans dépendre du détail.
// ---------------------------------------------------------------------------

function tap(result, x = 10, y = 10) {
  act(() => {
    result.current.handlers.onPointerDown({ clientX: x, clientY: y })
    result.current.handlers.onPointerUp({ clientX: x, clientY: y })
  })
}

beforeEach(() => {
  vi.useFakeTimers()
})

afterEach(() => {
  vi.useRealTimers()
})

describe('useDoubleTap — déclenchement (AC12)', () => {
  it('deux taps francs, rapprochés, déclenchent onDoubleTap UNE fois', () => {
    const onDoubleTap = vi.fn()
    const { result } = renderHook(() => useDoubleTap(onDoubleTap))

    tap(result)
    act(() => { vi.advanceTimersByTime(100) }) // bien dans la fenêtre par défaut (300ms)
    tap(result)

    expect(onDoubleTap).toHaveBeenCalledTimes(1)
  })

  it('exposé via { handlers } avec onPointerDown/onPointerUp, pas onClick/onDoubleClick', () => {
    const { result } = renderHook(() => useDoubleTap(vi.fn()))
    expect(typeof result.current.handlers.onPointerDown).toBe('function')
    expect(typeof result.current.handlers.onPointerUp).toBe('function')
    expect(result.current.handlers.onClick).toBeUndefined()
    expect(result.current.handlers.onDoubleClick).toBeUndefined()
  })

  it('un troisième tap juste après un double-tap déclenché ne redéclenche pas tout seul (compteur réinitialisé)', () => {
    const onDoubleTap = vi.fn()
    const { result } = renderHook(() => useDoubleTap(onDoubleTap))

    tap(result)
    act(() => { vi.advanceTimersByTime(50) })
    tap(result) // double-tap n°1
    expect(onDoubleTap).toHaveBeenCalledTimes(1)

    tap(result) // un seul tap isolé après le déclenchement
    act(() => { vi.advanceTimersByTime(50) })
    expect(onDoubleTap).toHaveBeenCalledTimes(1) // toujours 1, pas 2
  })
})

describe('useDoubleTap — un seul tap ne déclenche rien (AC13)', () => {
  it('un tap isolé ne déclenche pas onDoubleTap', () => {
    const onDoubleTap = vi.fn()
    const { result } = renderHook(() => useDoubleTap(onDoubleTap))

    tap(result)
    act(() => { vi.advanceTimersByTime(500) })

    expect(onDoubleTap).not.toHaveBeenCalled()
  })
})

describe('useDoubleTap — fenêtre temporelle (AC14)', () => {
  it('deux taps séparés de plus de 300ms (délai par défaut) ne comptent PAS comme un double-tap', () => {
    const onDoubleTap = vi.fn()
    const { result } = renderHook(() => useDoubleTap(onDoubleTap))

    tap(result)
    act(() => { vi.advanceTimersByTime(301) })
    tap(result)

    expect(onDoubleTap).not.toHaveBeenCalled()
  })

  it('un tap hors fenêtre devient le PREMIER tap d\'une nouvelle paire (pas ignoré)', () => {
    const onDoubleTap = vi.fn()
    const { result } = renderHook(() => useDoubleTap(onDoubleTap))

    tap(result) // tap A
    act(() => { vi.advanceTimersByTime(301) }) // hors fenêtre — A expire
    tap(result) // tap B — devient le nouveau "premier tap"
    act(() => { vi.advanceTimersByTime(50) }) // dans la fenêtre suivant B
    tap(result) // tap C — B+C forment une paire valide

    expect(onDoubleTap).toHaveBeenCalledTimes(1)
  })

  it('délai personnalisé ({ delay: 500 }) est respecté', () => {
    const onDoubleTap = vi.fn()
    const { result } = renderHook(() => useDoubleTap(onDoubleTap, { delay: 500 }))

    tap(result)
    act(() => { vi.advanceTimersByTime(400) }) // hors des 300ms par défaut, dans les 500ms custom
    tap(result)

    expect(onDoubleTap).toHaveBeenCalledTimes(1)
  })
})

describe('useDoubleTap — tolérance de déplacement (AC15)', () => {
  it('un glissement supérieur à la tolérance par défaut (10px) entre down et up n\'est PAS compté comme un tap', () => {
    const onDoubleTap = vi.fn()
    const { result } = renderHook(() => useDoubleTap(onDoubleTap))

    act(() => {
      result.current.handlers.onPointerDown({ clientX: 0, clientY: 0 })
      result.current.handlers.onPointerUp({ clientX: 25, clientY: 0 }) // 25px de glissement
    })
    act(() => { vi.advanceTimersByTime(50) })
    tap(result, 0, 0) // tap franc, valide

    // Le tap glissé ne devait pas compter comme le "premier tap" — celui-ci
    // (franc) est donc lui-même seulement le premier, pas un second.
    expect(onDoubleTap).not.toHaveBeenCalled()
  })

  it('un déplacement dans la tolérance (≤10px) reste un tap valide', () => {
    const onDoubleTap = vi.fn()
    const { result } = renderHook(() => useDoubleTap(onDoubleTap))

    act(() => {
      result.current.handlers.onPointerDown({ clientX: 0, clientY: 0 })
      result.current.handlers.onPointerUp({ clientX: 5, clientY: 0 }) // 5px, sous la tolérance
    })
    act(() => { vi.advanceTimersByTime(50) })
    tap(result, 0, 0)

    expect(onDoubleTap).toHaveBeenCalledTimes(1)
  })

  it('tolérance personnalisée ({ moveTolerance: 30 }) est respectée', () => {
    const onDoubleTap = vi.fn()
    const { result } = renderHook(() => useDoubleTap(onDoubleTap, { moveTolerance: 30 }))

    act(() => {
      result.current.handlers.onPointerDown({ clientX: 0, clientY: 0 })
      result.current.handlers.onPointerUp({ clientX: 25, clientY: 0 }) // 25px, sous la tolérance custom de 30
    })
    act(() => { vi.advanceTimersByTime(50) })
    tap(result, 0, 0)

    expect(onDoubleTap).toHaveBeenCalledTimes(1)
  })
})

describe('useDoubleTap — cycle de vie', () => {
  it('ne lève aucune erreur au démontage, même avec un tap en attente', () => {
    const onDoubleTap = vi.fn()
    const { result, unmount } = renderHook(() => useDoubleTap(onDoubleTap))

    tap(result) // un seul tap, encore "en attente" d'un éventuel second

    expect(() => unmount()).not.toThrow()
    expect(() => { act(() => { vi.advanceTimersByTime(1000) }) }).not.toThrow()
  })

  it('onDoubleTap peut changer entre deux rendus sans casser la détection (référence à jour utilisée)', () => {
    const first = vi.fn()
    const second = vi.fn()
    const { result, rerender } = renderHook(({ cb }) => useDoubleTap(cb), { initialProps: { cb: first } })

    rerender({ cb: second })

    tap(result)
    act(() => { vi.advanceTimersByTime(50) })
    tap(result)

    expect(second).toHaveBeenCalledTimes(1)
    expect(first).not.toHaveBeenCalled()
  })
})
