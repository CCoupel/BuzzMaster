import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import useWebSocket from './useWebSocket'

// ---------------------------------------------------------------------------
// Mock WebSocket global — même pattern que useWebSocket.creditPoints.test.js
// et useWebSocket.questionPosition.test.js
// ---------------------------------------------------------------------------

let wsInstance = null

class MockWebSocket {
  constructor(url) {
    wsInstance = this
    this.url = url
    this.readyState = MockWebSocket.CONNECTING
  }
  send() {}
  close() {
    this.readyState = MockWebSocket.CLOSED
    this.onclose && this.onclose()
  }
}
MockWebSocket.CONNECTING = 0
MockWebSocket.OPEN      = 1
MockWebSocket.CLOSING   = 2
MockWebSocket.CLOSED    = 3

// ---------------------------------------------------------------------------
// Tests : useWebSocket — awardedTeams (#170/F1, T4)
//
// Source de vérité du verrouillage de crédit (AnimCreditControl, F2) :
// JAMAIS d'anticipation locale du clic, uniquement ce que le serveur
// confirme via AWARDED_TEAMS. Garde anti-obsolescence : un payload dont
// QUESTION_ID ne correspond plus à la question affichée localement
// (gameState.question.ID) est rejeté — sinon une réponse en vol au moment
// d'un changement de question pourrait verrouiller/déverrouiller la
// mauvaise question.
// ---------------------------------------------------------------------------

function setCurrentQuestion(id) {
  return act(async () => {
    wsInstance.onmessage({
      data: JSON.stringify({ ACTION: 'UPDATE', MSG: { GAME: { PHASE: 'STARTED', QUESTION: { ID: id } } } }),
    })
  })
}

describe('useWebSocket — awardedTeams (#170/F1)', () => {
  beforeEach(() => {
    wsInstance = null
    vi.stubGlobal('WebSocket', MockWebSocket)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    vi.clearAllMocks()
  })

  it('initialise awardedTeams à {} (état vide)', () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))
    expect(result.current.awardedTeams).toEqual({})
  })

  it('indexe les équipes créditées par nom (TEAM) quand QUESTION_ID correspond à la question courante', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))
    await setCurrentQuestion('1')

    await act(async () => {
      wsInstance.onmessage({
        data: JSON.stringify({
          ACTION: 'AWARDED_TEAMS',
          MSG: { QUESTION_ID: '1', TEAMS: [{ TEAM: 'Les Rouges', POINTS: 10, TIMESTAMP: 1000 }] },
        }),
      })
    })

    expect(result.current.awardedTeams).toEqual({ 'Les Rouges': { POINTS: 10, TIMESTAMP: 1000 } })
  })

  // R1 — un crédit à 0 point doit être indexé au même titre qu'un crédit
  // positif : l'entrée existe, POINTS vaut 0, ce n'est pas absent.
  it('indexe une équipe créditée à 0 point (refus) — entrée présente, pas absente', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))
    await setCurrentQuestion('1')

    await act(async () => {
      wsInstance.onmessage({
        data: JSON.stringify({
          ACTION: 'AWARDED_TEAMS',
          MSG: { QUESTION_ID: '1', TEAMS: [{ TEAM: 'Les Bleus', POINTS: 0, TIMESTAMP: 1000 }] },
        }),
      })
    })

    expect(result.current.awardedTeams['Les Bleus']).toBeDefined()
    expect(result.current.awardedTeams['Les Bleus'].POINTS).toBe(0)
  })

  it('rejette un payload dont QUESTION_ID ne correspond plus à la question courante', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))
    await setCurrentQuestion('1')

    // Un AWARDED_TEAMS de la question "1" arrive légitimement d'abord…
    await act(async () => {
      wsInstance.onmessage({
        data: JSON.stringify({
          ACTION: 'AWARDED_TEAMS',
          MSG: { QUESTION_ID: '1', TEAMS: [{ TEAM: 'Les Rouges', POINTS: 10, TIMESTAMP: 1000 }] },
        }),
      })
    })
    expect(result.current.awardedTeams).toEqual({ 'Les Rouges': { POINTS: 10, TIMESTAMP: 1000 } })

    // … puis la question locale change (nouvelle question "2")…
    await setCurrentQuestion('2')

    // … et un payload EN RETARD, encore marqué QUESTION_ID "1", arrive
    // après coup : il doit être ignoré (ni appliqué, ni utilisé pour
    // réinitialiser l'état de la question "2").
    await act(async () => {
      wsInstance.onmessage({
        data: JSON.stringify({
          ACTION: 'AWARDED_TEAMS',
          MSG: { QUESTION_ID: '1', TEAMS: [{ TEAM: 'Les Verts', POINTS: 5, TIMESTAMP: 2000 }] },
        }),
      })
    })
    expect(result.current.awardedTeams).toEqual({ 'Les Rouges': { POINTS: 10, TIMESTAMP: 1000 } })
  })

  it('vidé (reset à {}) sur un payload dont TEAMS est vide, pour la question courante', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))
    await setCurrentQuestion('1')

    await act(async () => {
      wsInstance.onmessage({
        data: JSON.stringify({
          ACTION: 'AWARDED_TEAMS',
          MSG: { QUESTION_ID: '1', TEAMS: [{ TEAM: 'Les Rouges', POINTS: 10, TIMESTAMP: 1000 }] },
        }),
      })
    })
    expect(result.current.awardedTeams).not.toEqual({})

    await act(async () => {
      wsInstance.onmessage({
        data: JSON.stringify({ ACTION: 'AWARDED_TEAMS', MSG: { QUESTION_ID: '1', TEAMS: [] } }),
      })
    })
    expect(result.current.awardedTeams).toEqual({})
  })

  it('un AWARDED_TEAMS ultérieur (même question) écrase l\'état précédent (pas de fusion)', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))
    await setCurrentQuestion('1')

    await act(async () => {
      wsInstance.onmessage({
        data: JSON.stringify({
          ACTION: 'AWARDED_TEAMS',
          MSG: { QUESTION_ID: '1', TEAMS: [{ TEAM: 'Les Rouges', POINTS: 10, TIMESTAMP: 1000 }] },
        }),
      })
    })

    await act(async () => {
      wsInstance.onmessage({
        data: JSON.stringify({
          ACTION: 'AWARDED_TEAMS',
          MSG: {
            QUESTION_ID: '1',
            TEAMS: [
              { TEAM: 'Les Rouges', POINTS: 10, TIMESTAMP: 1000 },
              { TEAM: 'Les Bleus', POINTS: 0, TIMESTAMP: 2000 },
            ],
          },
        }),
      })
    })

    expect(Object.keys(result.current.awardedTeams).sort()).toEqual(['Les Bleus', 'Les Rouges'])
  })

  it('accepte un payload QUESTION_ID="" quand aucune question n\'est courante localement', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))
    // Pas de setCurrentQuestion — gameState.question reste absent.

    await act(async () => {
      wsInstance.onmessage({
        data: JSON.stringify({ ACTION: 'AWARDED_TEAMS', MSG: { QUESTION_ID: '', TEAMS: [] } }),
      })
    })
    expect(result.current.awardedTeams).toEqual({})
  })
})
