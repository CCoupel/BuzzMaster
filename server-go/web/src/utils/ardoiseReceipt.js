// #246 (v11.2) — règle UNIQUE « équipes attendues / hors ligne / reçues » de la
// garde de REVEAL ARDOISE, partagée par /admin (GamePage) et /anim. Miroir du
// serveur (contracts/websocket-actions.md §REVEAL « Garde ARDOISE ») :
//   - équipe attendue : clé de `teams` ayant ≥ 1 bumper IS_VPLAYER avec TEAM = cette clé ;
//   - hors ligne (affichage seulement, ne lève pas la garde) : TOUS ses VJoueurs CONNECTED=false ;
//   - reçue : ARDOISE_ANSWERS[équipe].RECEIVED === true (absence d'entrée = non reçue).

function vplayersOf(bumpers, teamName) {
  return Object.values(bumpers || {}).filter((b) => b && b.IS_VPLAYER && b.TEAM === teamName)
}

export function getExpectedArdoiseTeams(teams, bumpers) {
  return Object.keys(teams || {}).filter((name) => vplayersOf(bumpers, name).length > 0)
}

export function isArdoiseTeamOffline(bumpers, teamName) {
  const vps = vplayersOf(bumpers, teamName)
  return vps.length > 0 && vps.every((b) => b.CONNECTED === false)
}

/**
 * @returns {{expected: string[], total: number, receivedCount: number,
 *   pending: Array<{team: string, offline: boolean}>, complete: boolean,
 *   byTeam: Object<string, 'ok'|'wait'|'off'>}}
 */
export function getArdoiseReceipt(teams, bumpers, answers) {
  const expected = getExpectedArdoiseTeams(teams, bumpers)
  const byTeam = {}
  const pending = []
  for (const team of expected) {
    if (answers?.[team]?.RECEIVED === true) {
      byTeam[team] = 'ok'
    } else {
      const offline = isArdoiseTeamOffline(bumpers, team)
      byTeam[team] = offline ? 'off' : 'wait'
      pending.push({ team, offline })
    }
  }
  return {
    expected,
    total: expected.length,
    receivedCount: expected.length - pending.length,
    pending,
    complete: pending.length === 0,
    byTeam,
  }
}

// Phases où la garde s'applique (le REVEAL part de STOPPED ; PAUSED reçoit aussi les finals).
export const ARDOISE_RECEIPT_PHASES = ['STOPPED', 'PAUSED', 'REVEALED']
