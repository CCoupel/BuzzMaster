import { describe, it, expect } from 'vitest'
import { sortTeamsByBuzzOrder, getRankBadge, formatReactionTime } from './buzzOrder'

// ========================================
// sortTeamsByBuzzOrder — mutualisé entre GamePage.jsx et AnimPage.jsx (#156)
// ========================================

describe('sortTeamsByBuzzOrder', () => {
  const teams = [
    { name: 'C', TIME: 0 },
    { name: 'A', TIME: 5000 },
    { name: 'B', TIME: 2000 },
    { name: 'D', TIME: 0 },
  ]

  it('hors des phases actives, retourne la liste inchangée (même référence)', () => {
    ;['STOP', 'PREPARE', 'READY', 'NEW_GAME', 'COUNTDOWN'].forEach(phase => {
      expect(sortTeamsByBuzzOrder(teams, phase)).toBe(teams)
    })
  })

  it('STARTED : équipes buzzées triées par TIME croissant, non-buzzées à la suite', () => {
    const result = sortTeamsByBuzzOrder(teams, 'STARTED')
    expect(result.map(t => t.name)).toEqual(['B', 'A', 'C', 'D'])
  })

  it('même tri en PAUSED, REVEALED et STOPPED', () => {
    ;['PAUSED', 'REVEALED', 'STOPPED'].forEach(phase => {
      expect(sortTeamsByBuzzOrder(teams, phase).map(t => t.name)).toEqual(['B', 'A', 'C', 'D'])
    })
  })

  it('ne mute pas la liste d\'entrée', () => {
    const original = [...teams]
    sortTeamsByBuzzOrder(teams, 'STARTED')
    expect(teams).toEqual(original)
  })

  it('équipe sans TIME (undefined) traitée comme non-buzzée', () => {
    const list = [{ name: 'X' }, { name: 'Y', TIME: 100 }]
    expect(sortTeamsByBuzzOrder(list, 'STARTED').map(t => t.name)).toEqual(['Y', 'X'])
  })
})

// ========================================
// getRankBadge
// ========================================

describe('getRankBadge', () => {
  it('retourne le trophée pour le rang 1', () => {
    expect(getRankBadge(1)).toBe('🏆')
  })

  it('retourne la médaille argent pour le rang 2', () => {
    expect(getRankBadge(2)).toBe('🥈')
  })

  it('retourne la médaille bronze pour le rang 3', () => {
    expect(getRankBadge(3)).toBe('🥉')
  })

  it('retourne null au-delà du rang 3', () => {
    expect(getRankBadge(4)).toBeNull()
    expect(getRankBadge(10)).toBeNull()
  })
})

// ========================================
// formatReactionTime
// ========================================

describe('formatReactionTime', () => {
  it('retourne null si timestamp est absent', () => {
    expect(formatReactionTime(undefined, 1000000)).toBeNull()
    expect(formatReactionTime(0, 1000000)).toBeNull()
  })

  it('retourne null si gameTime est absent', () => {
    expect(formatReactionTime(2000000, undefined)).toBeNull()
    expect(formatReactionTime(2000000, 0)).toBeNull()
  })

  it('formate en secondes avec 3 décimales, suffixe "s"', () => {
    // 1.5s d'écart en microsecondes
    expect(formatReactionTime(2_500_000, 1_000_000)).toBe('1.500s')
  })

  it('gère les écarts sous la seconde', () => {
    expect(formatReactionTime(1_234_000, 1_000_000)).toBe('0.234s')
  })
})
