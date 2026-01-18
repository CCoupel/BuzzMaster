import { useMemo } from 'react'

/**
 * Calculate rankings with tie support
 * Items with the same score share the same rank
 *
 * @param {Array} items - Array of items with score property
 * @param {string} scoreKey - Key to use for score (default: 'score')
 * @returns {Array} Items with added 'rank' property
 */
export function calculateRanks(items, scoreKey = 'score') {
  if (!items || items.length === 0) return []

  // Sort by score descending
  const sorted = [...items].sort((a, b) => (b[scoreKey] || 0) - (a[scoreKey] || 0))

  let currentRank = 1
  let lastScore = null
  let lastRank = 1

  return sorted.map((item, index) => {
    const score = item[scoreKey] || 0
    if (index > 0 && score === lastScore) {
      // Same score = same rank (tie)
      return { ...item, rank: lastRank }
    }
    currentRank = index + 1
    lastScore = score
    lastRank = currentRank
    return { ...item, rank: currentRank }
  })
}

/**
 * Hook to sort and rank teams by score
 *
 * @param {Object} teams - Teams object { name: { SCORE, COLOR, ... } }
 * @returns {Array} Sorted teams with rank: [{ name, score, color, rank }, ...]
 */
export function useSortedTeams(teams) {
  return useMemo(() => {
    if (!teams || Object.keys(teams).length === 0) return []

    const teamsArray = Object.entries(teams).map(([name, data]) => ({
      name,
      score: data.SCORE || 0,
      color: data.COLOR,
      teamPoints: data.TEAM_POINTS || 0,
      time: data.TIME || 0,
      status: data.STATUS,
      ready: data.READY || false,
    }))

    return calculateRanks(teamsArray, 'score')
  }, [teams])
}

/**
 * Hook to sort and rank players (bumpers) by score
 *
 * @param {Object} bumpers - Bumpers object { mac: { NAME, SCORE, TEAM, ... } }
 * @param {Object} teams - Teams object for team colors
 * @returns {Array} Sorted players with rank: [{ mac, name, score, team, teamColor, rank }, ...]
 */
export function useSortedPlayers(bumpers, teams) {
  return useMemo(() => {
    if (!bumpers || Object.keys(bumpers).length === 0) return []

    const playersArray = Object.entries(bumpers).map(([mac, data]) => ({
      mac,
      name: data.NAME || mac.slice(-6),
      score: data.SCORE || 0,
      team: data.TEAM,
      teamColor: teams?.[data.TEAM]?.COLOR,
      time: data.TIME || 0,
      button: data.BUTTON,
      ready: data.READY || false,
      answerColor: data.ANSWER_COLOR,
    }))

    return calculateRanks(playersArray, 'score')
  }, [bumpers, teams])
}

/**
 * Get max score from a ranked list
 * @param {Array} items - Array with score property
 * @returns {number} Maximum score (minimum 1 to avoid division by zero)
 */
export function getMaxScore(items) {
  if (!items || items.length === 0) return 1
  return Math.max(...items.map(item => item.score || 0), 1)
}
