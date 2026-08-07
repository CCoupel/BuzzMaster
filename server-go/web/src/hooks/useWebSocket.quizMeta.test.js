/**
 * Tests — useWebSocket : mapping QUIZ_POPULATIONS/QUIZ_DIFFICULTIES/
 * QUIZ_HIDDEN_FIELDS/QUIZ_OBJECTIVES (#137 Batch 2b T2.1).
 *
 * Contexte (_work/handoff/task-test-writer-review-batch2b-20260806-165427.md) :
 * dev-frontend a livré la normalisation défensive `normalizeQuizArray()` et
 * le mapping v6.1.0 sans nouveau test Vitest dédié. Deux zones couvertes
 * ici :
 *  1. Normalisation défensive : un GameState malformé où QUIZ_POPULATIONS/
 *     QUIZ_DIFFICULTIES/QUIZ_HIDDEN_FIELDS seraient une string au lieu d'un
 *     tableau doit devenir [] sans planter (risque R2 du contrat
 *     game-state.md).
 *  2. Confidentialité round-trip (défense en profondeur) : QUIZ_OBJECTIVES
 *     ne doit jamais apparaître dans le state mappé depuis un message
 *     conforme à ce que /ws/tv ou /ws/player envoient réellement (déjà
 *     filtré côté serveur, cf. test-writer côté backend) — verrouille le
 *     comportement du hook lui-même, indépendamment du serveur.
 *
 * Suit le pattern de mocks de useWebSocket.ardoise.test.js.
 */
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import useWebSocket from './useWebSocket'

// ---------------------------------------------------------------------------
// Mock WebSocket global — capture l'instance pour simuler les messages
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
MockWebSocket.OPEN = 1
MockWebSocket.CLOSING = 2
MockWebSocket.CLOSED = 3

describe('useWebSocket — normalisation défensive QUIZ_* multi-valeurs (#137 Batch 2b T2.1)', () => {
  beforeEach(() => {
    wsInstance = null
    vi.stubGlobal('WebSocket', MockWebSocket)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    vi.clearAllMocks()
  })

  it('initialise quizPopulations/quizDifficulties/quizHiddenFields à [] dans le state initial', () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))

    expect(result.current.gameState.quizPopulations).toEqual([])
    expect(result.current.gameState.quizDifficulties).toEqual([])
    expect(result.current.gameState.quizHiddenFields).toEqual([])
  })

  it('un UPDATE avec des tableaux normaux mappe les valeurs telles quelles', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))

    await act(async () => {
      wsInstance.onmessage({
        data: JSON.stringify({
          ACTION: 'UPDATE',
          MSG: {
            GAME: {
              PHASE: 'NEW_GAME',
              QUIZ_POPULATIONS: ['Ado (13-17 ans)', 'Adulte (18-64 ans)'],
              QUIZ_DIFFICULTIES: ['Moyen'],
              QUIZ_HIDDEN_FIELDS: ['DIFFICULTIES'],
            },
          },
        }),
      })
    })

    expect(result.current.gameState.quizPopulations).toEqual(['Ado (13-17 ans)', 'Adulte (18-64 ans)'])
    expect(result.current.gameState.quizDifficulties).toEqual(['Moyen'])
    expect(result.current.gameState.quizHiddenFields).toEqual(['DIFFICULTIES'])
  })

  it.each([
    ['QUIZ_POPULATIONS', 'quizPopulations'],
    ['QUIZ_DIFFICULTIES', 'quizDifficulties'],
    ['QUIZ_HIDDEN_FIELDS', 'quizHiddenFields'],
  ])('%s reçu comme une string (au lieu d\'un tableau) devient [] sans planter', async (wireKey, stateKey) => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))

    await act(async () => {
      wsInstance.onmessage({
        data: JSON.stringify({
          ACTION: 'UPDATE',
          MSG: {
            GAME: {
              PHASE: 'NEW_GAME',
              [wireKey]: 'Adulte (18-64 ans)', // malformé : string au lieu de string[]
            },
          },
        }),
      })
    })

    expect(result.current.gameState[stateKey]).toEqual([])
  })

  it('un champ absent du message (serveur non redéployé) devient [] plutôt que undefined ou null', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))

    await act(async () => {
      wsInstance.onmessage({
        data: JSON.stringify({
          ACTION: 'UPDATE',
          MSG: {
            GAME: { PHASE: 'NEW_GAME' /* pas de QUIZ_POPULATIONS/DIFFICULTIES/HIDDEN_FIELDS */ },
          },
        }),
      })
    })

    expect(result.current.gameState.quizPopulations).toEqual([])
    expect(result.current.gameState.quizDifficulties).toEqual([])
    expect(result.current.gameState.quizHiddenFields).toEqual([])
  })
})

describe('useWebSocket — QUIZ_OBJECTIVES : round-trip de confidentialité côté client (#137 Batch 2b, défense en profondeur)', () => {
  beforeEach(() => {
    wsInstance = null
    vi.stubGlobal('WebSocket', MockWebSocket)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    vi.clearAllMocks()
  })

  it('quizObjectives reste \'\' (défaut) sur un message conforme à ce que /ws/tv ou /ws/player envoient (QUIZ_OBJECTIVES absent)', async () => {
    // Le serveur est censé déjà filtrer QUIZ_OBJECTIVES pour /ws/tv et
    // /ws/player (couvert côté backend, internal/protocol/messages_quiz_objectives_test.go).
    // Ce test verrouille le hook LUI-MÊME : un message qui ne porte pas la
    // clé ne doit produire aucune valeur résiduelle inattendue.
    const { result } = renderHook(() => useWebSocket('/ws/tv'))

    await act(async () => {
      wsInstance.onmessage({
        data: JSON.stringify({
          ACTION: 'UPDATE',
          MSG: {
            GAME: {
              PHASE: 'NEW_GAME',
              QUIZ_NAME: 'Quiz ciné',
              QUIZ_POPULATIONS: ['Adulte (18-64 ans)'],
              // Pas de QUIZ_OBJECTIVES — c'est le comportement attendu du serveur.
            },
          },
        }),
      })
    })

    expect(result.current.gameState.quizObjectives).toBe('')
    expect(result.current.gameState.quizName).toBe('Quiz ciné')
  })

  it('un QUIZ_OBJECTIVES déjà connu (ex: reçu plus tôt sur /ws/admin) n\'est PAS effacé par un UPDATE ultérieur qui l\'omet — absent = inchangé, pas une fuite', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/admin'))

    await act(async () => {
      wsInstance.onmessage({
        data: JSON.stringify({
          ACTION: 'UPDATE',
          MSG: { GAME: { PHASE: 'NEW_GAME', QUIZ_OBJECTIVES: 'Réviser le chapitre 3' } },
        }),
      })
    })
    expect(result.current.gameState.quizObjectives).toBe('Réviser le chapitre 3')

    await act(async () => {
      wsInstance.onmessage({
        data: JSON.stringify({
          ACTION: 'UPDATE',
          MSG: { GAME: { PHASE: 'STARTED' /* pas de QUIZ_OBJECTIVES */ } },
        }),
      })
    })
    expect(result.current.gameState.quizObjectives).toBe('Réviser le chapitre 3')
  })
})
