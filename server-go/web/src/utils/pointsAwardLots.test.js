import { describe, it, expect } from 'vitest'
import {
  planAward, lotOffsetMs, scheduleAward, pickArrivalSlots,
  LOT_MAX, NORMAL_LOT_MS, ACCEL_FACTOR, ACCEL_FROM_LOT,
} from './pointsAwardLots'

// #265 — module PUR partagé TV / tablette (web/src/utils/pointsAwardLots.js).
// L'API ci-dessous est le contrat fixé par ces tests (le plan nomme les fonctions sans figer les
// signatures) ; maquettes validées : docs/mockup/v11.2.1/ui/tv_points_award__multi_team.html
// (« Règles de l'animation ») et vplayer_points_award__own_team.html ; contrat game-state.md
// §SCORE_CHANGE.GAINS.
//
//   planAward(gains)  gains = SCORE_CHANGE.GAINS [{TEAM, POINTS}]
//     -> { lots: [{ index, entries: [{TEAM, POINTS}], durationMs, startMs }], totalMs }
//        POINTS <= 0 exclus ; une équipe n'apparaît qu'une fois ; ordre de GAINS conservé (aucun tri).
//   lotOffsetMs(plan, team)  -> startMs du lot contenant l'équipe, ou null si absente.
//   scheduleAward(busyUntilMs, nowMs, plan)
//     -> { startAtMs, delayMs, busyUntilMs } : la file ; un plan sans lot ne change rien.
//   pickArrivalSlots(count, rng = Math.random) -> [{ x, y }] (centres d'arrivée en %, count <= 6).
//   Constantes : LOT_MAX = 6, NORMAL_LOT_MS = 3200, ACCEL_FACTOR = 0.55, ACCEL_FROM_LOT = 4.

const gainsOf = (n, pts = 3) => Array.from({ length: n }, (_, i) => ({ TEAM: `Equipe ${String(i + 1).padStart(2, '0')}`, POINTS: pts }))
const sizes = (plan) => plan.lots.map((l) => l.entries.length)

describe('constantes (décisions utilisateur)', () => {
  it('lots de 6 au plus, 3,2 s, ×0,55 à partir du 4ᵉ lot', () => {
    expect(LOT_MAX).toBe(6)
    expect(NORMAL_LOT_MS).toBe(3200)
    expect(ACCEL_FACTOR).toBe(0.55)
    expect(ACCEL_FROM_LOT).toBe(4)
  })
})

describe('planAward — découpage en lots (au plus juste)', () => {
  const cases = [
    [1, [1]], [4, [4]], [6, [6]], [7, [4, 3]], [8, [4, 4]], [12, [6, 6]],
    [13, [5, 4, 4]], [18, [6, 6, 6]], [20, [5, 5, 5, 5]], [24, [6, 6, 6, 6]],
  ]
  it.each(cases)('%i équipe(s) → lots %j', (n, expected) => {
    const plan = planAward(gainsOf(n))
    expect(sizes(plan)).toEqual(expected)
    expect(plan.lots.length).toBe(Math.ceil(n / LOT_MAX))
  })

  it('conserve l\'ordre de GAINS d\'un lot à l\'autre (aucun tri par gain ni par nom)', () => {
    const gains = [{ TEAM: 'Zèbres', POINTS: 1 }, { TEAM: 'Anges', POINTS: 9 }, { TEAM: 'Mulots', POINTS: 5 }]
    const plan = planAward(gains)
    expect(plan.lots[0].entries.map((e) => e.TEAM)).toEqual(['Zèbres', 'Anges', 'Mulots'])
  })

  it('numérote les lots à partir de 0 dans l\'ordre', () => {
    expect(planAward(gainsOf(13)).lots.map((l) => l.index)).toEqual([0, 1, 2])
  })
})

describe('planAward — entrées exclues ou dégénérées', () => {
  it('aucune équipe, [] ou undefined → aucun lot, durée nulle', () => {
    for (const g of [[], undefined, null]) {
      const plan = planAward(g)
      expect(plan.lots).toEqual([])
      expect(plan.totalMs).toBe(0)
    }
  })

  it('POINTS ≤ 0 exclus (pénalité, compensation à 0) et ne comptent pas dans le découpage', () => {
    const gains = [
      { TEAM: 'A', POINTS: 3 }, { TEAM: 'B', POINTS: -2 }, { TEAM: 'C', POINTS: 0 }, { TEAM: 'D', POINTS: 1 },
    ]
    const plan = planAward(gains)
    expect(plan.lots[0].entries.map((e) => e.TEAM)).toEqual(['A', 'D'])
    expect(plan.lots.length).toBe(1)
  })

  it('uniquement des gains ≤ 0 → aucun lot', () => {
    expect(planAward([{ TEAM: 'A', POINTS: -1 }, { TEAM: 'B', POINTS: 0 }]).lots).toEqual([])
  })

  it('une même équipe n\'apparaît jamais deux fois dans les lots', () => {
    const plan = planAward([{ TEAM: 'A', POINTS: 2 }, { TEAM: 'B', POINTS: 1 }, { TEAM: 'A', POINTS: 4 }])
    const names = plan.lots.flatMap((l) => l.entries.map((e) => e.TEAM))
    expect(names.filter((n) => n === 'A').length).toBe(1)
  })
})

describe('planAward — durées et décalages (tableau de la maquette)', () => {
  const totals = [[1, 3200], [4, 3200], [6, 3200], [8, 6400], [12, 6400], [18, 9600], [20, 11360], [24, 11360]]
  it.each(totals)('%i équipe(s) → durée totale %i ms', (n, ms) => {
    expect(planAward(gainsOf(n)).totalMs).toBeCloseTo(ms, 6)
  })

  it('lots 1 à 3 au rythme normal (3,2 s), 4ᵉ lot et suivants à 1,76 s', () => {
    const plan = planAward(gainsOf(24))
    const d = plan.lots.map((l) => l.durationMs)
    expect(d.slice(0, 3)).toEqual([3200, 3200, 3200])
    expect(d[3]).toBeCloseTo(1760, 6)
    const d30 = planAward(gainsOf(30)).lots.map((l) => l.durationMs) // 5 lots
    expect(d30.slice(3).every((x) => Math.abs(x - 1760) < 1e-6)).toBe(true)
  })

  it('startMs = somme des durées des lots précédents (sans chevauchement)', () => {
    const plan = planAward(gainsOf(24))
    expect(plan.lots.map((l) => l.startMs)).toEqual([0, 3200, 6400, 9600])
    expect(plan.totalMs).toBeCloseTo(plan.lots[3].startMs + plan.lots[3].durationMs, 6)
  })
})

describe('lotOffsetMs', () => {
  const plan = planAward(gainsOf(12))
  it('équipe du 1er lot → 0 ; équipe du 2e lot → 3200', () => {
    expect(lotOffsetMs(plan, 'Equipe 01')).toBe(0)
    expect(lotOffsetMs(plan, 'Equipe 06')).toBe(0)
    expect(lotOffsetMs(plan, 'Equipe 07')).toBe(3200)
    expect(lotOffsetMs(plan, 'Equipe 12')).toBe(3200)
  })
  it('équipe absente ou exclue (gain ≤ 0) → null', () => {
    expect(lotOffsetMs(plan, 'Inconnue')).toBeNull()
    expect(lotOffsetMs(planAward([{ TEAM: 'A', POINTS: -1 }]), 'A')).toBeNull()
  })
  it('plan vide → null', () => {
    expect(lotOffsetMs(planAward([]), 'A')).toBeNull()
  })
})

describe('scheduleAward — file d\'attente', () => {
  const one = planAward(gainsOf(4)) // 3200 ms
  it('file libre : démarre tout de suite', () => {
    const r = scheduleAward(0, 10000, one)
    expect(r.startAtMs).toBe(10000)
    expect(r.delayMs).toBe(0)
    expect(r.busyUntilMs).toBe(13200)
  })
  it('file occupée : la 2e annonce démarre à la fin de la 1re, sans chevauchement', () => {
    const first = scheduleAward(0, 10000, one)
    const second = scheduleAward(first.busyUntilMs, 11000, one)
    expect(second.startAtMs).toBe(13200)
    expect(second.delayMs).toBe(2200)
    expect(second.busyUntilMs).toBe(16400)
  })
  it('file terminée depuis longtemps : repart de maintenant', () => {
    const r = scheduleAward(13200, 50000, one)
    expect(r.startAtMs).toBe(50000)
    expect(r.delayMs).toBe(0)
  })
  it('plan sans lot : n\'occupe pas la file', () => {
    const r = scheduleAward(20000, 15000, planAward([]))
    expect(r.busyUntilMs).toBe(20000)
  })
  it('une file vidée (changement de vue) = busyUntil remis à 0 : repart de maintenant', () => {
    expect(scheduleAward(0, 7000, one).startAtMs).toBe(7000)
  })
})

describe('pickArrivalSlots — grille 3 × 2 sans remise, zone sûre, zooms jamais recouverts', () => {
  const SAFE = { x0: 8, x1: 92, y0: 12, y1: 78 }
  const CELL_W = (SAFE.x1 - SAFE.x0) / 3
  const CELL_H = (SAFE.y1 - SAFE.y0) / 2
  const cellOf = (p) => `${Math.floor((p.x - SAFE.x0) / CELL_W)}:${Math.floor((p.y - SAFE.y0) / CELL_H)}`
  const lcg = (seed) => { let s = seed; return () => { s = (s * 1664525 + 1013904223) % 4294967296; return s / 4294967296 } }
  const rngs = [() => 0, () => 0.999999, () => 0.5, ...[1, 2, 3, 4, 5, 6, 7, 8].map((s) => lcg(s * 7919))]

  it('count 0 → aucune case', () => {
    expect(pickArrivalSlots(0)).toEqual([])
  })

  for (let count = 1; count <= 6; count++) {
    it(`${count} équipe(s) : cases distinctes, dans la zone sûre, jitter ≤ ±1,5 % / ±5 %, écart minimal respecté`, () => {
      for (const rng of rngs) {
        const slots = pickArrivalSlots(count, rng)
        expect(slots).toHaveLength(count)
        expect(new Set(slots.map(cellOf)).size).toBe(count) // sans remise
        for (const p of slots) {
          expect(p.x).toBeGreaterThanOrEqual(SAFE.x0)
          expect(p.x).toBeLessThanOrEqual(SAFE.x1)
          expect(p.y).toBeGreaterThanOrEqual(SAFE.y0)
          expect(p.y).toBeLessThanOrEqual(SAFE.y1)
          const col = Math.floor((p.x - SAFE.x0) / CELL_W), row = Math.floor((p.y - SAFE.y0) / CELL_H)
          const cx = SAFE.x0 + CELL_W * (col + 0.5), cy = SAFE.y0 + CELL_H * (row + 0.5)
          expect(Math.abs(p.x - cx)).toBeLessThanOrEqual(1.5 + 1e-9)
          expect(Math.abs(p.y - cy)).toBeLessThanOrEqual(5 + 1e-9)
        }
        for (let i = 0; i < slots.length; i++) {
          for (let j = i + 1; j < slots.length; j++) {
            const dx = Math.abs(slots[i].x - slots[j].x), dy = Math.abs(slots[i].y - slots[j].y)
            expect(dx >= 25 - 1e-9 || dy >= 23 - 1e-9).toBe(true)
          }
        }
      }
    })
  }

  it('à 6 équipes les 6 cases servent toutes', () => {
    const slots = pickArrivalSlots(6, lcg(42))
    expect(new Set(slots.map(cellOf)).size).toBe(6)
  })

  it('l\'attribution des cases dépend du hasard injecté (deux graines → au moins une différence sur 20 essais)', () => {
    const a = pickArrivalSlots(6, lcg(1)).map(cellOf).join()
    const others = Array.from({ length: 20 }, (_, i) => pickArrivalSlots(6, lcg(100 + i)).map(cellOf).join())
    expect(others.some((o) => o !== a)).toBe(true)
  })
})
