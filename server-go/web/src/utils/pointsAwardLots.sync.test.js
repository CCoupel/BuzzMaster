import { describe, it, expect } from 'vitest'
import { planAward, lotOffsetMs, scheduleAward } from './pointsAwardLots'

// #265 — test CROISÉ TV ↔ tablette. La TV joue tous les lots ; la tablette ne joue que le gain de son
// équipe, au début du lot de la TV qui la contient, avec la durée de ce lot. Les deux appellent la MÊME
// fonction pure sur la MÊME entrée (SCORE_CHANGE.GAINS) : mêmes lots, mêmes décalages, même file.
// Contrat : game-state.md §SCORE_CHANGE.GAINS (Lecture) ; maquette vplayer_points_award__own_team.

const mk = (n, base = 0) => Array.from({ length: n }, (_, i) => ({ TEAM: `Equipe ${String(base + i + 1).padStart(2, '0')}`, POINTS: i + 1 }))
const tvLotOf = (plan, team) => plan.lots.find((l) => l.entries.some((e) => e.TEAM === team))

describe('TV ↔ tablette : même plan pour la même entrée', () => {
  for (const n of [1, 6, 7, 12, 13, 20, 24]) {
    it(`${n} équipes : pour CHAQUE équipe, le décalage de la tablette = début du lot de la TV, même durée`, () => {
      const gains = mk(n)
      const tv = planAward(gains)
      const tablet = planAward(gains) // appel indépendant, comme sur un autre appareil
      for (const g of gains) {
        const lot = tvLotOf(tv, g.TEAM)
        expect(lotOffsetMs(tablet, g.TEAM)).toBe(lot.startMs)
        expect(tvLotOf(tablet, g.TEAM).durationMs).toBeCloseTo(lot.durationMs, 9)
        expect(tvLotOf(tablet, g.TEAM).entries.find((e) => e.TEAM === g.TEAM).POINTS).toBe(g.POINTS)
      }
    })
  }

  it('équipe du 4ᵉ lot : la tablette reprend la durée accélérée de la TV (1,76 s)', () => {
    const gains = mk(24)
    const plan = planAward(gains)
    expect(tvLotOf(plan, 'Equipe 24').durationMs).toBeCloseTo(1760, 6)
    expect(lotOffsetMs(plan, 'Equipe 24')).toBe(9600)
  })

  it('équipe absente de GAINS : la tablette n\'a rien à jouer (null)', () => {
    expect(lotOffsetMs(planAward(mk(12)), 'Équipe fantôme')).toBeNull()
  })
})

describe('TV ↔ tablette : file identique sur plusieurs SEQ', () => {
  it('deux SEQ consécutifs (12 équipes puis 4) : la 2e annonce démarre au même instant des deux côtés', () => {
    const first = planAward(mk(12))
    const second = planAward(mk(4, 20))
    const now1 = 1000, now2 = 2500 // l'UPDATE du 2e SEQ arrive pendant l'animation du 1er
    const tv = scheduleAward(scheduleAward(0, now1, first).busyUntilMs, now2, second)
    const tablet = scheduleAward(scheduleAward(0, now1, first).busyUntilMs, now2, second)
    expect(tablet).toEqual(tv)
    expect(tv.startAtMs).toBe(now1 + first.totalMs) // 1000 + 6400
    // décalage de la tablette pour une équipe du 2e SEQ = attente de la file + décalage du lot
    expect(tv.startAtMs + lotOffsetMs(second, 'Equipe 21')).toBe(7400)
  })

  it('un SEQ sans gain exploitable (tout ≤ 0) n\'occupe pas la file de l\'un ni de l\'autre', () => {
    const none = planAward([{ TEAM: 'A', POINTS: -3 }])
    const r = scheduleAward(5000, 4000, none)
    expect(r.busyUntilMs).toBe(5000)
  })
})
