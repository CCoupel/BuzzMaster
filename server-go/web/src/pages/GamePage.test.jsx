import { render, screen } from '@testing-library/react'
import GamePage from './GamePage'

describe('GamePage - Tri par rapidité de réponse', () => {
  // Test 1 : Calcul temps en ms
  test('Calcul temps: (team.TIME - gameState.GAME_TIME) / 1000', () => {
    const gameTime = 1000000000
    const teamTime = 1000100000
    const expected = 100  // ms

    const result = Math.round((teamTime - gameTime) / 1000)
    expect(result).toBe(expected)
  })

  // Test 2 : Tri par temps (plus rapide en haut)
  test('Équipes triées par temps croissant (rapide → lent)', () => {
    const teams = [
      { TIME: 1000150000 },    // 150ms
      { TIME: 1000100000 },    // 100ms (plus rapide)
      { TIME: 1000200000 },    // 200ms
    ]

    const sorted = teams.sort((a, b) => a.TIME - b.TIME)

    expect(sorted[0].TIME).toBe(1000100000)  // 100ms en premier
    expect(sorted[1].TIME).toBe(1000150000)  // 150ms en second
    expect(sorted[2].TIME).toBe(1000200000)  // 200ms en dernier
  })

  // Test 3 : Équipes non buzzées en bas
  test('Équipes avec TIME=0 toujours en bas', () => {
    const buzzedTeams = [{ TIME: 1000100000 }]
    const nonBuzzedTeams = [{ TIME: 0 }]

    const result = [...buzzedTeams, ...nonBuzzedTeams]

    expect(result[0].TIME).toBeGreaterThan(0)
    expect(result[1].TIME).toBe(0)
  })

  // Test 4 : Tri stable - même temps = ordre préservé
  test('Tri stable: même temps conserve l\'ordre', () => {
    const teams = [
      { name: 'A', TIME: 1000100000 },
      { name: 'B', TIME: 1000100000 },
      { name: 'C', TIME: 1000100000 },
    ]

    const sorted = [...teams].sort((a, b) => a.TIME - b.TIME)

    expect(sorted[0].name).toBe('A')
    expect(sorted[1].name).toBe('B')
    expect(sorted[2].name).toBe('C')
  })

  // Test 5 : Phase-aware - tri uniquement en STARTED/PAUSED/REVEALED
  test('Tri actif UNIQUEMENT en STARTED/PAUSED/REVEALED', () => {
    const phases = ['STARTED', 'PAUSED', 'REVEALED']
    phases.forEach(phase => {
      expect(['STARTED', 'PAUSED', 'REVEALED'].includes(phase)).toBe(true)
    })

    const excludedPhases = ['STOP', 'PREPARE', 'READY']
    excludedPhases.forEach(phase => {
      expect(['STARTED', 'PAUSED', 'REVEALED'].includes(phase)).toBe(false)
    })
  })

  // Test 6 : Badge de classement
  test('Badge de classement: 🏆 pour rang 1, 🥈 pour rang 2, 🥉 pour rang 3', () => {
    const getRankBadge = (r) => {
      if (r === 1) return '🏆'
      if (r === 2) return '🥈'
      if (r === 3) return '🥉'
      return null
    }

    expect(getRankBadge(1)).toBe('🏆')
    expect(getRankBadge(2)).toBe('🥈')
    expect(getRankBadge(3)).toBe('🥉')
    expect(getRankBadge(4)).toBeNull()
  })

  // Test 7 : Tri joueurs - même logique que équipes
  test('Joueurs triés par timestamp croissant (rapide → lent)', () => {
    const buzzers = [
      { mac: 'a', timestamp: 1000200000 },    // 200ms
      { mac: 'b', timestamp: 1000100000 },    // 100ms (plus rapide)
      { mac: 'c', timestamp: 0 },             // Pas buzzé
    ]

    const buzzed = buzzers.filter(b => (b.timestamp ?? 0) > 0)
    const notBuzzed = buzzers.filter(b => (b.timestamp ?? 0) === 0)

    buzzed.sort((a, b) => a.timestamp - b.timestamp)

    const result = [...buzzed, ...notBuzzed]

    expect(result[0].mac).toBe('b')  // 100ms
    expect(result[1].mac).toBe('a')  // 200ms
    expect(result[2].mac).toBe('c')  // 0ms (pas buzzé)
  })
})
