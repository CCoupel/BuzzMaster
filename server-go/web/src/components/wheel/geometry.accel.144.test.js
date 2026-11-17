/**
 * #144 (12.0.0.4) — lancement de la roue ROUE : montée en vitesse 0 → v0 (accélération constante sur accelMs), jonction
 * continue avec la croisière, arrêt PENDANT la montée : le freinage part de la vitesse courante, sans saut de vitesse ni
 * d'angle, et atterrit exactement sur `to` sans dépasser `durationMs`.
 */
import { describe, it, expect } from 'vitest'
import { spinRampPosition, spinSpeed, spinPosition, brakePlan, brakePosition } from './geometry'

const wrap = (a, b, n) => ((b - a + n / 2) % n + n) % n - n / 2
const speedOf = (f, t, n, dt = 0.5) => wrap(f(t - dt), f(t + dt), n) / (2 * dt)
const RAMP = 1500

describe('spin avec montée', () => {
  const n = 24, per = 2500, v0 = n / per
  const pos = t => spinRampPosition(5, t, per, n, RAMP)

  it('part de l\'arrêt (vitesse 0), monte linéairement, atteint v0 à accelMs', () => {
    expect(spinSpeed(0, per, n, RAMP)).toBe(0)
    expect(spinSpeed(RAMP / 2, per, n, RAMP)).toBeCloseTo(v0 / 2, 9)
    expect(spinSpeed(RAMP, per, n, RAMP)).toBeCloseTo(v0, 9)
    expect(spinSpeed(5000, per, n, RAMP)).toBeCloseTo(v0, 9)
    expect(speedOf(pos, 1, n)).toBeLessThan(v0 * 0.01 + 1e-4) // ~0 à t=0+
  })
  it('vitesse mesurée sur la position = spinSpeed (montée puis croisière), continue à la jonction', () => {
    for (const t of [50, 200, 400, 700, 799, 801, 900, 1500, 3000]) {
      expect(speedOf(pos, t, n)).toBeCloseTo(spinSpeed(t, per, n, RAMP), 4)
    }
    expect(Math.abs(speedOf(pos, RAMP - 1, n) - speedOf(pos, RAMP + 1, n)) / v0).toBeLessThan(0.01)
  })
  it('position continue (pas de saut d\'angle) et normalisée', () => {
    let prev = pos(0)
    for (let t = 1; t < 4000; t += 3) {
      const p = pos(t)
      expect(p).toBeGreaterThanOrEqual(0); expect(p).toBeLessThan(n)
      expect(Math.abs(wrap(prev, p, n))).toBeLessThanOrEqual(v0 * 3 + 1e-9)
      prev = p
    }
  })
  it('sans accelMs : profil d\'origine (uniforme), inchangé', () => {
    expect(spinRampPosition(5, 1000, per, n, 0)).toBeCloseTo(spinPosition(5, 1000, per, n), 9)
    expect(spinSpeed(0, per, n, 0)).toBeCloseTo(v0, 9)
  })
})

describe('arrêt pendant la montée : freinage depuis la vitesse courante', () => {
  const SIZES = [8, 24, 64], PERS = [1000, 2500, 6000], DURS = [1500, 4000, 8000]
  it('vitesse continue au début du freinage (< 1 % de v0) et angle continu', () => {
    let plans = 0
    for (const n of SIZES) for (const per of PERS) for (const dur of DURS) for (const tStop of [0, 100, 300, 500, 799]) {
      const v0 = n / per, vs = spinSpeed(tStop, per, n, RAMP)
      const from = 3 + (v0 * tStop * tStop) / (2 * RAMP)
      for (let d = 0; d < n; d += Math.max(1, Math.floor(n / 6))) {
        const to = Math.floor(from + d) % n
        const plan = brakePlan(from, to, n, dur, per, vs, RAMP)
        if (!plan) continue
        plans++
        const f = t => brakePosition(plan, from, to, t, n)
        expect(Math.abs(speedOf(f, 0.5, n, 0.25) - vs) / v0).toBeLessThan(0.01)
        expect(Math.abs(wrap(from, f(0), n))).toBeLessThan(1e-6)
        // vitesse jamais au-dessus de v0 (aucune accélération parasite) ; atterrissage exact, jamais après durationMs
        for (let t = 5; t < dur; t += Math.max(5, dur / 60)) expect(Math.abs(speedOf(f, t, n, 2))).toBeLessThanOrEqual(v0 * 1.01 + 1e-9)
        expect(plan.t1 + plan.tc + plan.tb).toBeLessThanOrEqual(dur + 1e-6)
        expect(f(dur)).toBe(to)
      }
    }
    expect(plans).toBeGreaterThan(300)
  })
  it('arrêt à vitesse nulle (dès le lancement) : montée puis freinage, atterrissage exact', () => {
    const n = 24, per = 2500, dur = 4000
    for (const to of [4, 10, 18]) {
      const plan = brakePlan(3, to, n, dur, per, 0, RAMP)
      expect(plan).not.toBeNull()
      expect(brakePosition(plan, 3, to, dur, n)).toBe(to)
      expect(Math.abs(speedOf(t => brakePosition(plan, 3, to, t, n), 0.5, n, 0.25))).toBeLessThan(0.02 * (n / per))
    }
  })
  it('à vitesse de croisière (vStart = v0) : identique au plan de 12.0.0.3', () => {
    const p = brakePlan(3, 15, 24, 4000, 2500)
    expect(p.t1).toBe(0)
    expect(p.vp).toBeCloseTo(24 / 2500, 12)
    expect(p.tc + p.tb).toBeCloseTo(4000, 6)
  })
})
