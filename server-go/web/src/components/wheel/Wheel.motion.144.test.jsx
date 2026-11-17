import { describe, it, expect, afterEach, beforeEach, vi } from 'vitest'
import { render, cleanup, act } from '@testing-library/react'
import Wheel from './Wheel'
import { spinPosition, landTotal, landPosition, offsetToIndex } from './geometry'

// #144 (v12.0.0) — mouvement long `spin` / `land` (contracts/wheel-component.md §5, révisé).
afterEach(() => { cleanup(); vi.useRealTimers() })

const segs = (n = 22) => Array.from({ length: n }, (_, i) => ({ id: `s${i}`, label: String(100 * (i + 1)), kind: 'value' }))

describe('geometry — mouvement long', () => {
  it('spinPosition : un tour en periodMs, toujours dans [0, n)', () => {
    expect(spinPosition(0, 1250, 2500, 20)).toBeCloseTo(10, 5)
    expect(spinPosition(5, 2500, 2500, 20)).toBeCloseTo(5, 5)
    for (let k = 0; k < 1000; k++) {
      const p = spinPosition(Math.random() * 20, Math.random() * 1e7, 2500, 20)
      expect(p).toBeGreaterThanOrEqual(0)
      expect(p).toBeLessThan(20)
    }
  })
  it('landTotal : amplitude bornée (≤ 4n) et arrivée exacte sur `to`', () => {
    for (let k = 0; k < 1000; k++) {
      const n = 1 + Math.floor(Math.random() * 64)
      const from = Math.random() * n
      const to = Math.floor(Math.random() * n)
      const total = landTotal(from, to, n, Math.floor(Math.random() * 10))
      expect(total).toBeLessThanOrEqual(4 * n)
      const d = Math.abs(offsetToIndex(from + total - to, n))
      expect(Math.min(d, n - d)).toBeLessThan(1e-6)
      expect(landPosition(from, to, total, 1, n)).toBe(to)
    }
  })
  it('landTotal : au moins minTurns tours quand la position est déjà sur `to`', () => {
    expect(landTotal(7, 7, 22, 1)).toBe(22)
  })
})

describe('Wheel — mouvement long (composant)', () => {
  beforeEach(() => { vi.useFakeTimers() })

  const run = async (ms) => { await act(async () => { vi.advanceTimersByTime(ms) }) }

  it.each(['strip', 'arc'])('spin (%s) : défile, aucun segment mis en évidence, position bornée', async (layout) => {
    const { container } = render(<Wheel segments={segs()} current={3} layout={layout} motion={{ mode: 'spin', periodMs: 2500 }} />)
    const root = container.querySelector('[role="img"]')
    await run(1000)
    const p1 = Number(root.getAttribute('data-pos'))
    await run(700)
    const p2 = Number(root.getAttribute('data-pos'))
    expect(p2).not.toBe(p1)
    for (const p of [p1, p2]) { expect(p).toBeGreaterThanOrEqual(0); expect(p).toBeLessThan(22) }
    expect(container.querySelector('.is-current, [data-current="true"]')).toBeNull()
  })

  it.each(['strip', 'arc'])('land (%s) : s\'arrête sur `to`, mis en évidence, position normalisée', async (layout) => {
    const { container, rerender } = render(<Wheel segments={segs()} layout={layout} motion={{ mode: 'spin', periodMs: 2500 }} />)
    await run(1234)
    rerender(<Wheel segments={segs()} layout={layout} motion={{ mode: 'land', to: 7, durationMs: 4000, minTurns: 1 }} />)
    await run(4200)
    const root = container.querySelector('[role="img"]')
    expect(Number(root.getAttribute('data-pos'))).toBe(7)
    expect(container.querySelectorAll('[data-current="true"]').length).toBeGreaterThanOrEqual(1) // strip : une par répétition du ruban
    expect(root.getAttribute('aria-label')).toContain('800')
  })

  it('montage direct en land : placement immédiat sur `to`', () => {
    const { container } = render(<Wheel segments={segs()} motion={{ mode: 'land', to: 5, durationMs: 4000, minTurns: 1 }} />)
    expect(Number(container.querySelector('[role="img"]').getAttribute('data-pos'))).toBe(5)
  })

  it('100 mouvements consécutifs (spin → land) : cellules intactes, position normalisée, `to` sous le pointeur', async () => {
    const s = segs()
    const { container, rerender } = render(<Wheel segments={s} current={0} motion={null} />)
    const cells = container.querySelectorAll('.wheel-cell').length
    for (let k = 1; k <= 100; k++) {
      rerender(<Wheel segments={s} motion={{ mode: 'spin', periodMs: 2000 }} />)
      await run(300 + (k % 7) * 100)
      rerender(<Wheel segments={s} motion={{ mode: 'land', to: (k * 5) % 22, durationMs: 1500, minTurns: 1 }} />)
      await run(1600)
      const root = container.querySelector('[role="img"]')
      expect(Number(root.getAttribute('data-pos'))).toBe((k * 5) % 22)
    }
    expect(container.querySelectorAll('.wheel-cell').length).toBe(cells)
    const m = /translateX\(-([\d.]+)%\)/.exec(container.querySelector('.wheel-track').style.transform)
    expect(parseFloat(m[1])).toBeLessThan(100)
  })

  it('retour à motion=null : bascule courte vers `current` (RAFALE inchangé)', async () => {
    const s = segs()
    const { container, rerender } = render(<Wheel segments={s} current={2} motion={{ mode: 'land', to: 9, durationMs: 1000, minTurns: 1 }} />)
    rerender(<Wheel segments={s} current={2} motion={null} />)
    await run(1000)
    expect(Number(container.querySelector('[role="img"]').getAttribute('data-pos'))).toBe(2)
  })

  it('libellé sur deux lignes et pictogramme', () => {
    const s = [{ id: 'b', label: 'BANQUE\nROUTE', icon: '💸', kind: 'special' }, ...segs(5)]
    const { container } = render(<Wheel segments={s} current={0} />)
    expect(container.querySelector('.wheel-cell .wheel-icon').textContent).toBe('💸')
    expect(container.querySelector('.wheel-cell .wheel-lab br')).not.toBeNull()
    expect(container.querySelector('[role="img"]').getAttribute('aria-label')).toBe('Segment : BANQUE ROUTE')
  })
})
