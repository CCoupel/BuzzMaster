import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import useWebSocket from './useWebSocket'

// ---------------------------------------------------------------------------
// Tests : useWebSocket — câblage RAFALE (milestone v8.0.0 #16/#107, contrat
// contracts/rafale.md §2.2/§2.3/§5.2), patron useWebSocket.ardoise.test.js /
// useWebSocket.creditPoints.test.js.
//
// Périmètre (plan _work/reports/plan-20260828-161558.md, "Tests Requis") :
//
//	useWebSocket.rafale : traitement de RAFALE_TICK / RAFALE_ANSWER
//
// Complète les champs RAFALE_* portés par GameState (mappés depuis UPDATE,
// comme ARDOISE_ANSWERS) — RAFALE_TICK et RAFALE_ANSWER sont des actions
// SÉPARÉES, dédiées (§2.2/§2.3 : le timer de question et la réponse
// attendue ne transitent JAMAIS par un simple champ GameState réémis en
// bloc), donc testées séparément du reste des champs RAFALE_* déjà couverts
// implicitement par le patron ARDOISE_ANSWERS ci-dessus.
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

beforeEach(() => {
  wsInstance = null
  vi.stubGlobal('WebSocket', MockWebSocket)
})

afterEach(() => {
  vi.unstubAllGlobals()
  vi.clearAllMocks()
})

// ---------------------------------------------------------------------------
// État initial — aucun `omitempty`, jamais nil (règle projet, CLAUDE.md).
// ---------------------------------------------------------------------------

describe('useWebSocket — RAFALE, état initial (jamais nil/undefined)', () => {
  it('rafaleAnswer démarre à null (aucune question tirée)', () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))
    expect(result.current.rafaleAnswer).toBeNull()
  })

  it('gameState.RAFALE_QUESTION_TIME démarre à 0', () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))
    expect(result.current.gameState.RAFALE_QUESTION_TIME).toBe(0)
  })

  it('gameState.RAFALE_CURRENT_QUESTION démarre non-nil (objet vide typé, jamais null)', () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))
    expect(result.current.gameState.RAFALE_CURRENT_QUESTION).toBeDefined()
    expect(result.current.gameState.RAFALE_CURRENT_QUESTION).not.toBeNull()
  })

  it('gameState.RAFALE_TEAM_COUNTERS / RAFALE_PARTICIPATING_TEAMS démarrent vides mais non-nil', () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))
    expect(result.current.gameState.RAFALE_TEAM_COUNTERS).toEqual({})
    expect(result.current.gameState.RAFALE_PARTICIPATING_TEAMS).toEqual([])
  })
})

// ---------------------------------------------------------------------------
// RAFALE_TICK (contrat §5.2) — décompte léger du timer de QUESTION, ne
// réémet PAS tout GameState : un seul champ (RAFALE_QUESTION_TIME) doit
// changer, tout le reste doit rester intact.
// ---------------------------------------------------------------------------

describe('useWebSocket — RAFALE_TICK (contrat §5.2)', () => {
  it('met à jour gameState.RAFALE_QUESTION_TIME depuis MSG.QUESTION_TIME', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))

    await act(async () => {
      wsInstance.onmessage({ data: JSON.stringify({ ACTION: 'RAFALE_TICK', MSG: { QUESTION_TIME: 2 } }) })
    })

    expect(result.current.gameState.RAFALE_QUESTION_TIME).toBe(2)
  })

  it('des tics successifs mettent à jour la valeur à chaque fois (décompte réel)', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))

    for (const remaining of [3, 2, 1, 0]) {
      await act(async () => {
        wsInstance.onmessage({ data: JSON.stringify({ ACTION: 'RAFALE_TICK', MSG: { QUESTION_TIME: remaining } }) })
      })
      expect(result.current.gameState.RAFALE_QUESTION_TIME).toBe(remaining)
    }
  })

  it('ne touche à AUCUN autre champ de gameState (mise à jour légère, pas un UPDATE complet)', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))

    // Fixe PHASE via un UPDATE normal d'abord.
    await act(async () => {
      wsInstance.onmessage({
        data: JSON.stringify({ ACTION: 'UPDATE', MSG: { GAME: { PHASE: 'STARTED', RAFALE_CURRENT_TEAM: 'Équipe A' } } }),
      })
    })
    expect(result.current.gameState.phase).toBe('STARTED')
    expect(result.current.gameState.RAFALE_CURRENT_TEAM).toBe('Équipe A')

    await act(async () => {
      wsInstance.onmessage({ data: JSON.stringify({ ACTION: 'RAFALE_TICK', MSG: { QUESTION_TIME: 1 } }) })
    })

    // Seul RAFALE_QUESTION_TIME a changé — PHASE et RAFALE_CURRENT_TEAM survivent intacts.
    expect(result.current.gameState.RAFALE_QUESTION_TIME).toBe(1)
    expect(result.current.gameState.phase).toBe('STARTED')
    expect(result.current.gameState.RAFALE_CURRENT_TEAM).toBe('Équipe A')
  })

  it('MSG.QUESTION_TIME absent : conserve la valeur précédente (repli, ne remet pas à 0/undefined)', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))

    await act(async () => {
      wsInstance.onmessage({ data: JSON.stringify({ ACTION: 'RAFALE_TICK', MSG: { QUESTION_TIME: 3 } }) })
    })
    expect(result.current.gameState.RAFALE_QUESTION_TIME).toBe(3)

    await act(async () => {
      wsInstance.onmessage({ data: JSON.stringify({ ACTION: 'RAFALE_TICK', MSG: {} }) })
    })
    expect(result.current.gameState.RAFALE_QUESTION_TIME).toBe(3)
  })
})

// ---------------------------------------------------------------------------
// RAFALE_ANSWER (contrat §2.3/§5.2) — réponse attendue de la question
// courante, admin+anim uniquement côté serveur. rafaleAnswer est un état
// SÉPARÉ de gameState (jamais fusionné dedans) — c'est précisément ce qui
// garantit que PlayerDisplay.jsx (TV/VPlayer, qui ne lit QUE gameState) ne
// peut structurellement jamais l'afficher, même par erreur de câblage.
// ---------------------------------------------------------------------------

describe('useWebSocket — RAFALE_ANSWER (contrat §2.3/§5.2)', () => {
  // #202 (contrat §13.3) — RAFALE_ANSWER porte désormais TOUJOURS un champ
  // NEXT (objet, ou `null` explicite : §13.3 "null est une information à
  // part entière"). Les 4 assertions ci-dessous incluent donc `NEXT: null`
  // depuis #202 (CHANGELOG [20260901], CHANGED additif — MSG ne porte pas
  // NEXT dans ces payloads de test, donc `MSG.NEXT ?? null` résout à null) —
  // mise à jour couverte par le CHANGELOG documentant ce changement.
  it('met à jour rafaleAnswer depuis MSG.ID/MSG.ANSWER', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))

    await act(async () => {
      wsInstance.onmessage({ data: JSON.stringify({ ACTION: 'RAFALE_ANSWER', MSG: { ID: 'r-042', ANSWER: 'Rome' } }) })
    })

    expect(result.current.rafaleAnswer).toEqual({ ID: 'r-042', ANSWER: 'Rome', NEXT: null })
  })

  it('un RAFALE_ANSWER ultérieur écrase le précédent (question suivante)', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))

    await act(async () => {
      wsInstance.onmessage({ data: JSON.stringify({ ACTION: 'RAFALE_ANSWER', MSG: { ID: 'r-001', ANSWER: 'Paris' } }) })
    })
    expect(result.current.rafaleAnswer).toEqual({ ID: 'r-001', ANSWER: 'Paris', NEXT: null })

    await act(async () => {
      wsInstance.onmessage({ data: JSON.stringify({ ACTION: 'RAFALE_ANSWER', MSG: { ID: 'r-002', ANSWER: 'Berlin' } }) })
    })
    expect(result.current.rafaleAnswer).toEqual({ ID: 'r-002', ANSWER: 'Berlin', NEXT: null })
  })

  it('MSG.ID absent : rafaleAnswer n\'est PAS modifié (garde explicite, évite un état {ID: undefined})', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))

    await act(async () => {
      wsInstance.onmessage({ data: JSON.stringify({ ACTION: 'RAFALE_ANSWER', MSG: {} }) })
    })

    expect(result.current.rafaleAnswer).toBeNull()
  })

  it('MSG.ANSWER absent (mais ID présent) : replie sur une chaîne vide, jamais undefined', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))

    await act(async () => {
      wsInstance.onmessage({ data: JSON.stringify({ ACTION: 'RAFALE_ANSWER', MSG: { ID: 'r-042' } }) })
    })

    expect(result.current.rafaleAnswer).toEqual({ ID: 'r-042', ANSWER: '', NEXT: null })
  })

  it('rafaleAnswer reste dans son propre état, JAMAIS fusionné dans gameState (contrat §2.3 — TV/VPlayer ne lisent que gameState)', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))

    await act(async () => {
      wsInstance.onmessage({ data: JSON.stringify({ ACTION: 'RAFALE_ANSWER', MSG: { ID: 'r-042', ANSWER: 'SECRET' } }) })
    })

    expect(result.current.rafaleAnswer).toEqual({ ID: 'r-042', ANSWER: 'SECRET', NEXT: null })
    // Aucune clé de gameState (sérialisé) ne doit contenir la réponse.
    expect(JSON.stringify(result.current.gameState)).not.toContain('SECRET')
  })
})

// ---------------------------------------------------------------------------
// RAFALE_ANSWER.NEXT (#202, contrat §13.3) — pré-tirage de la question
// suivante, transporté par le MÊME canal admin+anim que ANSWER ci-dessus.
// `null` est une information à part entière ("aucune question suivante",
// fin de réservoir, §13.5), jamais une absence — donc `MSG.NEXT ?? null`,
// PAS `MSG.NEXT || null` (un objet NEXT valide ne doit jamais être confondu
// avec une valeur falsy).
// ---------------------------------------------------------------------------

describe('useWebSocket — RAFALE_ANSWER.NEXT (#202, contrat §13.3)', () => {
  it('MSG.NEXT objet valide : NEXT reflète exactement cet objet', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))
    const next = { ID: 'r-017', QUESTION: 'Plus long fleuve d\'Europe ?', CATEGORY: 'GEOGRAPHY', DIFFICULTY: 2 }

    await act(async () => {
      wsInstance.onmessage({ data: JSON.stringify({ ACTION: 'RAFALE_ANSWER', MSG: { ID: 'r-042', ANSWER: 'Rome', NEXT: next } }) })
    })

    expect(result.current.rafaleAnswer.NEXT).toEqual(next)
  })

  it('MSG.NEXT absent (clé manquante) : NEXT vaut null', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))

    await act(async () => {
      wsInstance.onmessage({ data: JSON.stringify({ ACTION: 'RAFALE_ANSWER', MSG: { ID: 'r-042', ANSWER: 'Rome' } }) })
    })

    expect(result.current.rafaleAnswer.NEXT).toBeNull()
  })

  it('MSG.NEXT explicitement null (dernière question du réservoir, §13.5) : NEXT reste null', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))

    await act(async () => {
      wsInstance.onmessage({ data: JSON.stringify({ ACTION: 'RAFALE_ANSWER', MSG: { ID: 'r-042', ANSWER: 'Rome', NEXT: null } }) })
    })

    expect(result.current.rafaleAnswer.NEXT).toBeNull()
  })

  it('un NEXT non-null suivi d\'un NEXT null (fin de réservoir atteinte) : le dernier message fait foi, pas de résidu de l\'objet précédent', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))

    await act(async () => {
      wsInstance.onmessage({
        data: JSON.stringify({
          ACTION: 'RAFALE_ANSWER',
          MSG: { ID: 'r-001', ANSWER: 'Paris', NEXT: { ID: 'r-002', QUESTION: 'Q2', CATEGORY: 'HISTORY', DIFFICULTY: 1 } },
        }),
      })
    })
    expect(result.current.rafaleAnswer.NEXT).not.toBeNull()

    await act(async () => {
      wsInstance.onmessage({ data: JSON.stringify({ ACTION: 'RAFALE_ANSWER', MSG: { ID: 'r-002', ANSWER: 'Q2 answer', NEXT: null } }) })
    })
    expect(result.current.rafaleAnswer).toEqual({ ID: 'r-002', ANSWER: 'Q2 answer', NEXT: null })
  })

  it('l\'énoncé de NEXT ne fuite jamais dans gameState (même discipline que ANSWER, même famille que ardoise_leak_128)', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))

    await act(async () => {
      wsInstance.onmessage({
        data: JSON.stringify({
          ACTION: 'RAFALE_ANSWER',
          MSG: { ID: 'r-042', ANSWER: 'Rome', NEXT: { ID: 'r-017', QUESTION: 'ENONCE_SUIVANT_SECRET', CATEGORY: 'GEOGRAPHY', DIFFICULTY: 2 } },
        }),
      })
    })

    expect(result.current.rafaleAnswer.NEXT.QUESTION).toBe('ENONCE_SUIVANT_SECRET')
    expect(JSON.stringify(result.current.gameState)).not.toContain('ENONCE_SUIVANT_SECRET')
  })
})
