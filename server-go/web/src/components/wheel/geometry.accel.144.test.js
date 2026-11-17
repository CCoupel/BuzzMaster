/**
 * #144 (12.0.0) — montée en vitesse de la roue ROUE PAR DISTANCE (contracts/wheel-component.md §5, roue.md §6) :
 * `accelTurns` tours (2) à accélération constante, durée `2 × accelTurns × periodMs`, puis croisière `vc = n / periodMs`.
 * Freinage depuis la vitesse instantanée (croisière ou, pour « Forcer l'arrêt » de l'admin, pendant la montée).
 * Critères d'acceptation 1 à 3 (fonctions pures). Signatures : spinSpeed(t, periodMs, n, accelTurns),
 * spinRampPosition(from, t, periodMs, n, accelTurns), brakePlan(from, to, n, durationMs, periodMs, vStart, accelTurns),
 * brakePosition(plan, from, to, t, n).
 */
import { describe, it, expect } from 'vitest'
import { spinRampPosition, spinSpeed, spinPosition, brakePlan, brakePosition } from './geometry'

const wrap = (a, b, n) => ((b - a + n / 2) % n + n) % n - n / 2
const speedOf = (f, t, n, dt = 0.5) => wrap(f(t - dt), f(t + dt), n) / (2 * dt)
const A = 2 // accelTurns de la Roue de la Fortune
const upMs = per => 2 * A * per

describe('critère 1 — montée : vitesse 0 → vc en 2 × accelTurns × periodMs, distance accelTurns × n', () => {
  for (const [n, per] of [[24, 2500], [24, 1000], [8, 6000], [64, 2500]]) {
    const vc = n / per, T = upMs(per)
    const pos = t => spinRampPosition(5, t, per, n, A)
    it(`n=${n}, periodMs=${per}`, () => {
      expect(spinSpeed(0, per, n, A)).toBe(0)
      expect(Math.abs(spinSpeed(T, per, n, A) - vc) / vc).toBeLessThanOrEqual(0.01)
      expect(spinSpeed(T + 5000, per, n, A)).toBeCloseTo(vc, 9)
      // croissante, jamais au-dessus de vc
      let prev = -1
      for (let t = 0; t <= T; t += T / 40) {
        const v = spinSpeed(t, per, n, A)
        expect(v).toBeGreaterThanOrEqual(prev); expect(v).toBeLessThanOrEqual(vc * 1.0001)
        prev = v
      }
      expect(spinSpeed(T / 2, per, n, A)).toBeCloseTo(vc / 2, 6) // accélération constante
      // distance parcourue = accelTurns × n segments (écart ≤ 1 %), la roue est revenue sur le segment de départ
      let dist = 0, p = pos(0)
      const dt = Math.max(1, T / 4000)
      for (let t = dt; t <= T + 1e-9; t += dt) { const q = pos(t); dist += (wrap(p, q, n) + n) % n; p = q }
      expect(Math.abs(dist - A * n) / (A * n)).toBeLessThanOrEqual(0.01)
      expect(Math.abs(wrap(5, pos(T), n))).toBeLessThan(0.01 * n)
    })
    it(`n=${n}, periodMs=${per} : position continue à la jonction, vitesse mesurée = spinSpeed, normalisée`, () => {
      for (const t of [T * 0.1, T * 0.5, T * 0.99, T * 1.01, T * 1.5]) {
        expect(speedOf(pos, t, n, 1)).toBeCloseTo(spinSpeed(t, per, n, A), 4)
      }
      expect(Math.abs(speedOf(pos, T - 1, n, 0.5) - speedOf(pos, T + 1, n, 0.5)) / vc).toBeLessThan(0.01)
      let prevP = pos(0)
      for (let t = 1; t < T * 1.3; t += T / 500) {
        const q = pos(t)
        expect(q).toBeGreaterThanOrEqual(0); expect(q).toBeLessThan(n)
        expect(Math.abs(wrap(prevP, q, n))).toBeLessThanOrEqual(vc * (T / 500) * 1.5 + 1e-9)
        prevP = q
      }
    })
  }
  it('accelTurns absent, nul ou invalide : pas de montée, vitesse vc dès le départ (comportement antérieur)', () => {
    const n = 24, per = 2500, vc = n / per
    for (const a of [undefined, 0, -1, NaN]) {
      expect(spinSpeed(0, per, n, a)).toBeCloseTo(vc, 9)
      expect(spinRampPosition(5, 1000, per, n, a)).toBeCloseTo(spinPosition(5, 1000, per, n), 9)
    }
  })
})

describe('critère 2 — freinage depuis la croisière', () => {
  const SIZES = [8, 24, 64], PERS = [1000, 2500, 6000], DURS = [1500, 4000, 8000]
  it('vitesse de départ vc (≤ 5 %), jamais en arrière, jamais au-dessus de vc, arrêt exact sur `to`, en durationMs au plus', () => {
    let plans = 0
    for (const n of SIZES) for (const per of PERS) for (const dur of DURS) {
      const vc = n / per
      for (let d = 0; d < n; d += Math.max(1, Math.floor(n / 6))) {
        const from = 3, to = (3 + d) % n
        const plan = brakePlan(from, to, n, dur, per, vc, A)
        if (!plan) continue
        plans++
        const f = t => brakePosition(plan, from, to, t, n)
        expect(Math.abs(speedOf(f, 0.5, n, 0.25) - vc) / vc).toBeLessThanOrEqual(0.05)
        let prev = f(0)
        for (let t = 5; t <= dur; t += Math.max(5, dur / 80)) {
          const q = f(t)
          expect(wrap(prev, q, n)).toBeGreaterThanOrEqual(-1e-6) // jamais de marche arrière
          expect(Math.abs(speedOf(f, t, n, 2))).toBeLessThanOrEqual(vc * 1.01 + 1e-9)
          prev = q
        }
        expect(f(dur)).toBe(to)
        expect(plan.t1 + plan.tc + plan.tb).toBeLessThanOrEqual(dur + 1e-6)
      }
    }
    expect(plans).toBeGreaterThan(100)
  })
})

describe('critère 3 — freinage pendant la montée (arrêt forcé) : v0 ∈ { 0,5 vc ; 0,1 vc }, sans saut', () => {
  const SIZES = [8, 24, 64], PERS = [1000, 2500, 6000], DURS = [1500, 4000, 8000]
  for (const frac of [0.5, 0.1]) {
    it(`vitesse de départ ${frac} vc : vitesse continue, jamais en arrière, ≤ vc, atterrissage exact, en durationMs au plus`, () => {
      let plans = 0
      for (const n of SIZES) for (const per of PERS) for (const dur of DURS) {
        const vc = n / per, T = upMs(per)
        const tStop = frac * T // vitesse atteinte sur la rampe = frac × vc
        const vs = spinSpeed(tStop, per, n, A)
        expect(vs / vc).toBeCloseTo(frac, 9)
        const from = spinRampPosition(3, tStop, per, n, A) // position à l'instant de l'arrêt
        for (let d = 1; d < n; d += Math.max(1, Math.floor(n / 6))) {
          const to = Math.floor(from + d) % n
          const plan = brakePlan(from, to, n, dur, per, vs, A)
          if (!plan) continue
          plans++
          const f = t => brakePosition(plan, from, to, t, n)
          // aucun saut de position ni de vitesse entre la dernière image du spin et la première du land
          expect(Math.abs(wrap(from, f(0), n))).toBeLessThan(1e-6)
          const vSpin = speedOf(t => spinRampPosition(3, t, per, n, A), tStop, n, 0.25)
          expect(Math.abs(speedOf(f, 0.5, n, 0.25) - vSpin) / vc).toBeLessThanOrEqual(0.05)
          let prev = f(0)
          for (let t = 5; t <= dur; t += Math.max(5, dur / 80)) {
            const q = f(t)
            expect(wrap(prev, q, n)).toBeGreaterThanOrEqual(-1e-6)
            expect(Math.abs(speedOf(f, t, n, 2))).toBeLessThanOrEqual(vc * 1.01 + 1e-9)
            prev = q
          }
          expect(f(dur)).toBe(to)
          expect(plan.t1 + plan.tc + plan.tb).toBeLessThanOrEqual(dur + 1e-6)
        }
      }
      expect(plans).toBeGreaterThan(100)
    })
  }
  it('arrêt dès le lancement (vitesse nulle) : montée poursuivie puis freinage, atterrissage exact', () => {
    const n = 24, per = 2500, dur = 4000
    for (const to of [4, 10, 18]) {
      const plan = brakePlan(3, to, n, dur, per, 0, A)
      if (!plan) continue // fenêtre trop courte : repli sur le profil sans periodMs (contrat §5)
      expect(brakePosition(plan, 3, to, dur, n)).toBe(to)
    }
  })
})
