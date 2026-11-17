/**
 * #144 (correctif 12.0.0.3) — freinage de la roue ROUE à vitesse continue : la vitesse au début du freinage égale la
 * vitesse de croisière du `spin` (< 1 %), l'atterrissage est exact sur `to`, la position reste normalisée.
 */
import { describe, it, expect } from 'vitest'
import { brakePlan, brakePosition, spinPosition, landTotal, landPosition } from './geometry'

const wrap = (a, b, n) => ((b - a + n / 2) % n + n) % n - n / 2
const speed = (f, t, n, dt = 1) => wrap(f(t), f(t + dt), n) / dt
const SIZES = [8, 24, 64]
const PERIODS = [1000, 2500, 4000, 6000]
const DURS = [1500, 3000, 4000, 8000]

describe('brakePlan / brakePosition', () => {
  it('vitesse continue à t=0 du freinage (< 1 %) quand un plan existe', () => {
    let plans = 0
    for (const n of SIZES) for (const per of PERIODS) for (const dur of DURS) {
      for (let d = 0; d < n; d += Math.max(1, Math.floor(n / 8))) {
        const to = (5 + d) % n
        const plan = brakePlan(5, to, n, dur, per)
        if (!plan) continue
        plans++
        const vSpin = n / per
        const v = speed(t => brakePosition(plan, 5, to, t, n), 0, n, 0.5)
        expect(Math.abs(v - vSpin) / vSpin).toBeLessThan(0.01)
      }
    }
    expect(plans).toBeGreaterThan(100)
  })

  it('atterrit exactement sur `to` à la fin du plan, jamais après durationMs', () => {
    for (const n of SIZES) for (const per of PERIODS) for (const dur of DURS) {
      for (let d = 0; d < n; d++) {
        const to = (7 + d) % n
        const plan = brakePlan(7, to, n, dur, per)
        if (!plan) continue
        expect(plan.tc + plan.tb).toBeLessThanOrEqual(dur + 1e-6)
        expect(plan.tc).toBeGreaterThanOrEqual(0)
        expect(brakePosition(plan, 7, to, plan.tc + plan.tb, n)).toBe(to)
        expect(brakePosition(plan, 7, to, dur, n)).toBe(to)
        // l'amplitude parcourue retombe sur la case tirée
        const end = ((7 + plan.D) % n + n) % n
        expect(Math.abs(end - to) < 1e-9 || Math.abs(Math.abs(end - to) - n) < 1e-9).toBe(true)
      }
    }
  })

  it('profil monotone, vitesse jamais supérieure à la croisière (pas d’accélération), position normalisée', () => {
    const n = 24, per = 2500, dur = 4000
    for (const to of [3, 4, 9, 15, 23, 2]) {
      const plan = brakePlan(3, to, n, dur, per)
      expect(plan).not.toBeNull()
      let prev = 3
      for (let t = 1; t <= dur; t += 7) {
        const p = brakePosition(plan, 3, to, t, n)
        expect(p).toBeGreaterThanOrEqual(0); expect(p).toBeLessThan(n)
        expect(Math.abs(wrap(prev, p, n))).toBeLessThanOrEqual(n / per * 7 + 1e-6)
        prev = p
      }
    }
  })

  it('défauts (24 segments, 2500 / 4000 ms) : plan pour les 24 distances ; amplitude ≤ 4n', () => {
    for (let d = 0; d < 24; d++) {
      const plan = brakePlan(0, d, 24, 4000, 2500)
      expect(plan, `d=${d}`).not.toBeNull()
      expect(plan.D).toBeLessThanOrEqual(4 * 24)
    }
  })

  it('fenêtre impossible (roue lente, arrêt très court) : null → l’ancien profil prend le relais', () => {
    expect(brakePlan(0, 20, 24, 1500, 6000)).toBeNull()
    expect(brakePlan(0, 5, 0, 4000, 2500)).toBeNull()
    expect(brakePlan(0, 5, 24, 4000, 0)).toBeNull()
  })
})

describe('non-régression : ancien profil `land` (RAFALE / sans periodMs) inchangé', () => {
  it('landTotal / landPosition donnent les mêmes valeurs qu’avant', () => {
    expect(landTotal(3, 15, 24, 1)).toBe(36)
    expect(landPosition(3, 15, 36, 0, 24)).toBeCloseTo(3, 9)
    expect(landPosition(3, 15, 36, 1, 24)).toBe(15)
    expect(landPosition(3, 15, 36, 0.5, 24)).toBeCloseTo(((3 + 36 * 0.875) % 24 + 24) % 24, 9)
    expect(spinPosition(3, 1250, 2500, 24)).toBeCloseTo(15, 9)
  })
})
