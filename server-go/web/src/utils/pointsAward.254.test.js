import { describe, it, expect } from 'vitest'
import * as pa from './pointsAward'

// #254 (v11.2, lot V2-4) — règles partagées : niveau d'attribution (équipe ou joueur) par type, commutateur « Tenir compte
// des indices » (paramètre useHints), pénalité toujours uniforme. Contrats : websocket-actions.md §TEAM_POINTS_BATCH
// « Niveau d'attribution » et « Montant », models.md (défaut POINTS_TARGET). Scénarios 9, 14, 22 à 26, 29, 30 du plan.

const Q = (over = {}) => ({
  TYPE: 'QCM', POINTS: '3', QCM_HINTS_ENABLED: true, QCM_CORRECT: 'GREEN',
  QCM_ANSWERS: { RED: 'a', GREEN: 'b', YELLOW: 'c', BLUE: 'd' }, ...over,
})
const team = (hints, color = 'GREEN') => [{ TIME: 1000, ANSWER_COLOR: color, HINTS_AT_BUZZ: hints }]

describe('resolveAwardLevel — tableau complet (scénario 9)', () => {
  it('est exportée, et resolvePointsTarget applique la même règle (source unique)', () => {
    expect(typeof pa.resolveAwardLevel).toBe('function')
    for (const type of ['SPEEDY', 'QCM', 'ARDOISE', 'MEMORY', 'MEMOTION', 'RAFALE']) {
      for (const target of [undefined, 'TEAM', 'PLAYER']) {
        const q = { TYPE: type, ...(target ? { POINTS_TARGET: target } : {}) }
        expect(pa.resolvePointsTarget(q), `${type}/${target}`).toBe(pa.resolveAwardLevel(q))
      }
    }
  })

  it.each([
    ['SPEEDY', undefined, 'PLAYER'], ['SPEEDY', 'TEAM', 'TEAM'], ['SPEEDY', 'PLAYER', 'PLAYER'],
    ['QCM', undefined, 'TEAM'], ['QCM', 'TEAM', 'TEAM'], ['QCM', 'PLAYER', 'PLAYER'],
    ['ARDOISE', undefined, 'TEAM'], ['ARDOISE', 'TEAM', 'TEAM'], ['ARDOISE', 'PLAYER', 'PLAYER'],
  ])('%s, POINTS_TARGET %s → %s', (type, target, expected) => {
    expect(pa.resolveAwardLevel({ TYPE: type, ...(target ? { POINTS_TARGET: target } : {}) })).toBe(expected)
  })

  it.each(['MEMORY', 'MEMOTION', 'RAFALE'])('Critical — %s : toujours TEAM, même si un ancien fichier porte POINTS_TARGET PLAYER', (type) => {
    for (const target of [undefined, 'TEAM', 'PLAYER']) {
      expect(pa.resolveAwardLevel({ TYPE: type, ...(target ? { POINTS_TARGET: target } : {}) })).toBe('TEAM')
    }
  })

  it('une ancienne ARDOISE sans POINTS_TARGET crédite l’équipe (changement de défaut #254)', () => {
    expect(pa.resolveAwardLevel({ TYPE: 'ARDOISE' })).toBe('TEAM')
  })
})

describe('Commutateur « Tenir compte des indices » — niveau équipe (scénarios 22 à 25)', () => {
  const amount = (hints, base, useHints, over = {}) => pa.defaultTeamAward({
    question: Q(over), teamName: 'A', basePoints: base, teamBumpers: team(hints), invalidatedCount: 2, useHints,
  })

  it('Critical (22) — activé (défaut) : base 3 → 3 / 2 / 1 pour 0, 1, 2 indices au buzz', () => {
    expect([0, 1, 2].map((h) => amount(h, 3, undefined))).toEqual([3, 2, 1])
    expect([0, 1, 2].map((h) => amount(h, 3, true))).toEqual([3, 2, 1])
  })

  it('(23) désactivé : montant uniforme, la base partout', () => {
    expect([0, 1, 2].map((h) => amount(h, 3, false))).toEqual([3, 3, 3])
  })

  it('Critical (24) — valeur forcée à 6 : activé → 6 / 4 / 2 ; désactivé → 6 partout', () => {
    expect([0, 1, 2].map((h) => amount(h, 6, true))).toEqual([6, 4, 2])
    expect([0, 1, 2].map((h) => amount(h, 6, false))).toEqual([6, 6, 6])
  })

  it('(25) arrondis : base 1 avec 2 indices → 1 (au moins 1 point) ; base 5 → 5 / 3 / 2', () => {
    expect(amount(2, 1, true)).toBe(1)
    expect([0, 1, 2].map((h) => amount(h, 5, true))).toEqual([5, 3, 2])
  })

  it('un coefficient personnalisé de la question est respecté', () => {
    expect(amount(1, 10, true, { QCM_PENALTY_1: 0.5, QCM_PENALTY_2: 0.25 })).toBe(5)
    expect(amount(2, 10, true, { QCM_PENALTY_1: 0.5, QCM_PENALTY_2: 0.25 })).toBe(3) // round(2,5)
  })

  it('équipe sans joueur ayant la bonne couleur : repli sur les indices actuellement révélés (activé), uniforme (désactivé)', () => {
    const wrong = team(0, 'RED')
    expect(pa.defaultTeamAward({ question: Q(), teamName: 'A', basePoints: 6, teamBumpers: wrong, invalidatedCount: 1, useHints: true })).toBe(4)
    expect(pa.defaultTeamAward({ question: Q(), teamName: 'A', basePoints: 6, teamBumpers: wrong, invalidatedCount: 1, useHints: false })).toBe(6)
  })
})

describe('Pénalité toujours uniforme — option A (scénario 26)', () => {
  it('Critical — montant négatif : aucun coefficient d’indices, quels que soient les indices et le commutateur', () => {
    for (const hints of [0, 1, 2]) {
      for (const useHints of [true, false, undefined]) {
        expect(pa.defaultTeamAward({
          question: Q(), teamName: 'A', basePoints: -2, teamBumpers: team(hints), invalidatedCount: hints, useHints,
        }), `${hints} indice(s), useHints=${useHints}`).toBe(-2)
      }
    }
  })

  it('une réponse éliminée par un indice reçoit la même pénalité (-2) qu’avant tout indice', () => {
    const eliminated = [{ TIME: 1000, ANSWER_COLOR: 'RED', HINTS_AT_BUZZ: 2 }]
    expect(pa.defaultTeamAward({ question: Q(), teamName: 'A', basePoints: -2, teamBumpers: eliminated, invalidatedCount: 2 })).toBe(-2)
    expect(pa.defaultTeamAward({ question: Q(), teamName: 'A', basePoints: -2, teamBumpers: eliminated, invalidatedCount: 0 })).toBe(-2)
  })
})

describe('Niveau joueur — coefficient du joueur (scénario 29)', () => {
  it('playerHintsCount : HINTS_AT_BUZZ du joueur qui a buzzé, sinon les indices actuels', () => {
    expect(pa.playerHintsCount({ TIME: 5, HINTS_AT_BUZZ: 1 }, 2)).toBe(1)
    expect(pa.playerHintsCount({ TIME: 0, HINTS_AT_BUZZ: 1 }, 2)).toBe(2)
    expect(pa.playerHintsCount({}, 0)).toBe(0)
  })

  it('resolvePointsAward QCM : montant dû au joueur selon ses indices au buzz (3 / 2 / 1 pour une base de 3)', () => {
    const amounts = [0, 1, 2].map((h) => pa.resolvePointsAward(Q(), 3, { hintsAtBuzz: h }).amount)
    expect(amounts).toEqual([3, 2, 1])
  })
})
