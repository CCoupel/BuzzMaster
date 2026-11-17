/**
 * #144 — montée par distance, 1 tour pour la Roue de la Fortune (contracts/wheel-component.md §5, critères d'acceptation 1 à 6).
 * Critère 1 : montée (vitesse 0 → vc, distance accelTurns×n, croissante, continue) — « critère 1 ».
 * Critère 2 : freinage depuis la croisière — « critère 2 ». Critère 3 (`land` reçu pendant la montée locale) : Wheel.accel.144.test.jsx.
 * Critère 5 (fonctions pures) : accelTurns absent = croisière immédiate — « critère 5 ». Critères 4 et 6 : Wheel.accel.144.test.jsx.
 */
import { describe, it, expect } from 'vitest'
import { accelDurationMs, spinRampPosition, spinSpeed, spinPosition, brakePlan, brakePosition } from './geometry'

const wrap = (a, b, n) => ((b - a + n / 2) % n + n) % n - n / 2
const speedOf = (f, t, n, dt = 0.5) => wrap(f(t - dt), f(t + dt), n) / (2 * dt)

// Roue de la Fortune : 1 tour depuis dd637f92 (SPIN_UP_MS = 2 × période) ; le composant reste générique (2 tours vérifiés aussi).
for (const TURNS of [1, 2]) {
describe(`critère 1 — montée sur ${TURNS} tour(s) (accelTurns = ${TURNS})`, () => {
  for (const [n, per] of [[24, 2500], [24, 1000], [40, 4000], [8, 6000]]) {
    const vc = n / per, T = accelDurationMs(per, TURNS)
    const pos = t => spinRampPosition(0, t, per, n, TURNS)
    // distance non repliée : intégrale de spinSpeed (trapèzes fins)
    const dist = (t) => { let d = 0; const dt = T / 4000; for (let x = 0; x < t; x += dt) d += spinSpeed(x + dt / 2, per, n, TURNS) * dt; return d }
    it(`n=${n} période=${per} : durée 2·accelTurns·période, vitesse 0 à t=0 et vc à la fin (≤ 1 %), distance accelTurns×n (≤ 1 %)`, () => {
      expect(T).toBe(2 * TURNS * per)
      expect(spinSpeed(0, per, n, TURNS)).toBe(0)
      expect(Math.abs(spinSpeed(T, per, n, TURNS) - vc) / vc).toBeLessThan(0.01)
      expect(Math.abs(dist(T) - TURNS * n) / (TURNS * n)).toBeLessThan(0.01)
    })
    it(`n=${n} période=${per} : vitesse croissante, position (modulo n) continue à la jonction et conforme à d = vc·t²/(4·accelTurns·période)`, () => {
      let prev = -1
      for (let t = 0; t <= T; t += T / 50) { const v = spinSpeed(t, per, n, TURNS); expect(v).toBeGreaterThanOrEqual(prev); prev = v }
      for (const t of [T / 4, T / 2, T - 1, T, T + 1, T * 2]) {
        const analytic = t < T ? (vc * t * t) / (4 * TURNS * per) : vc * (t - T / 2)
        expect(Math.abs(wrap(analytic, pos(t), n))).toBeLessThan(1e-6)
      }
      expect(Math.abs(speedOf(pos, T - 1, n) - speedOf(pos, T + 1, n)) / vc).toBeLessThan(0.01)
    })
  }
})

}

const PERS = [1000, 2500, 6000], DURS = [1500, 4000, 8000], SIZES = [8, 24, 64]

describe('critère 2 — freinage depuis la croisière (le seul freinage : roue.md §5.1)', () => {
  it('vitesse au début = vc (≤ 5 %), jamais de recul, vitesse ≤ vc, arrêt exact sur `to`, en durationMs au plus', () => {
    let plans = 0
    for (const n of SIZES) for (const per of PERS) for (const dur of DURS) for (let d = 0; d < n; d += Math.max(1, Math.floor(n / 6))) {
      const vc = n / per, from = 5, to = (5 + d) % n
      const plan = brakePlan(from, to, n, dur, per)
      if (!plan) continue
      plans++
      const f = t => brakePosition(plan, from, to, t, n)
      expect(Math.abs(speedOf(f, 0.5, n, 0.25) - vc) / vc).toBeLessThan(0.05)
      for (let t = 5; t < dur; t += Math.max(5, dur / 80)) {
        const v = wrap(f(t - 1), f(t + 1), n) / 2
        expect(v).toBeGreaterThanOrEqual(-1e-9); expect(v).toBeLessThanOrEqual(vc * 1.05 + 1e-9)
      }
      expect(plan.tc + plan.tb).toBeLessThanOrEqual(dur + 1e-6)
      expect(Math.abs(wrap(from, f(0), n))).toBeLessThan(1e-6)
      expect(f(dur)).toBe(to)
    }
    expect(plans).toBeGreaterThan(150)
  })
  it('défauts (24 segments, 2500 / 4000 ms) : un plan pour les 24 distances', () => {
    for (let d = 0; d < 24; d++) expect(brakePlan(0, d, 24, 4000, 2500), `d=${d}`).not.toBeNull()
  })
  it('fenêtre impossible (roue lente, arrêt très court) ou paramètres invalides : null → profil sans periodMs', () => {
    expect(brakePlan(0, 20, 24, 1500, 6000)).toBeNull()
    expect(brakePlan(0, 5, 0, 4000, 2500)).toBeNull()
    expect(brakePlan(0, 5, 24, 4000, 0)).toBeNull()
  })
})

describe('critère 5 (fonctions pures) — sans accelTurns : vitesse vc dès le départ', () => {
  it('accelTurns absent / nul / invalide', () => {
    const n = 24, per = 2500, vc = n / per
    for (const a of [undefined, 0, -1, NaN]) {
      expect(spinSpeed(0, per, n, a)).toBeCloseTo(vc, 12)
      expect(spinRampPosition(5, 1000, per, n, a)).toBeCloseTo(spinPosition(5, 1000, per, n), 9)
    }
  })
})
