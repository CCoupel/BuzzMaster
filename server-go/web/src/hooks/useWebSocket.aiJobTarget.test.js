/**
 * Tests — useWebSocket : mapping AI_GENERATION_PROGRESS.TARGET
 * (issue #203, génération IA du réservoir RAFALE, milestone v8.1.0).
 *
 * Contexte (code-review-20260901-175014.md, Majeur 2) : AIJobModalShell.target.test.jsx
 * et RafalePage.aiGeneration203.test.jsx couvrent bien le filtrage sur `target` —
 * mais construisent `aiJob` à la main (`runningJob('RAFALE')`, ou un
 * `useOptionalGame()` mocké) et ne passent jamais par le vrai code de mapping
 * WS. La ligne réellement ajoutée par #203 (`hooks/useWebSocket.js:817`,
 * `target: MSG.TARGET || 'QUIZ'`) n'avait donc aucun test — un bug dans ce
 * mapping (ex: mauvaise clé, `??` au lieu de `||` laissant passer une chaîne
 * vide comme "présente", oubli du repli 'QUIZ') serait invisible à toute la
 * suite #203, détecté seulement en QUALIF/PROD. Même classe de lacune déjà
 * rencontrée et corrigée sur ce même fichier pour `errorMessage` (#142) —
 * reproduit ici le pattern de useWebSocket.aiJobErrorMessage.test.js.
 *
 * Complète la chaîne de vérification bout-en-bout : backend
 * (rafale_generate_endpoint_203_test.go, TARGET sur AI_GENERATION_PROGRESS)
 * → hook WS (ici) → filtrage (AIJobModalShell.target.test.jsx, déjà en place).
 *
 * Suit le pattern de mocks de useWebSocket.aiJobErrorMessage.test.js.
 */
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import useWebSocket from './useWebSocket'

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

describe('useWebSocket — AI_GENERATION_PROGRESS.TARGET mapping (issue #203)', () => {
  beforeEach(() => {
    wsInstance = null
    vi.stubGlobal('WebSocket', MockWebSocket)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    vi.clearAllMocks()
  })

  it('aiJob est null tant qu\'aucun AI_GENERATION_PROGRESS n\'est reçu', () => {
    const { result } = renderHook(() => useWebSocket('/ws/admin'))

    expect(result.current.aiJob).toBeNull()
  })

  it('TARGET: "RAFALE" est mappé tel quel dans aiJob.target', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/admin'))

    await act(async () => {
      wsInstance.onmessage({
        data: JSON.stringify({
          ACTION: 'AI_GENERATION_PROGRESS',
          MSG: {
            JOB_ID: 'gen-20260901-000000-a1b2',
            TARGET: 'RAFALE',
            STATE: 'RUNNING',
            BATCHES_DONE: 1,
            BATCHES_TOTAL: 3,
            CREATED_COUNT: 8,
            SKIPPED_COUNT: 0,
            ERROR_CODE: '',
            PROVIDER: 'anthropic',
          },
        }),
      })
    })

    expect(result.current.aiJob.target).toBe('RAFALE')
  })

  it('TARGET: "QUIZ" (chemin historique, envoyé explicitement) est mappé tel quel', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/admin'))

    await act(async () => {
      wsInstance.onmessage({
        data: JSON.stringify({
          ACTION: 'AI_GENERATION_PROGRESS',
          MSG: {
            JOB_ID: 'gen-20260901-000001-c3d4',
            TARGET: 'QUIZ',
            STATE: 'RUNNING',
            BATCHES_DONE: 1,
            BATCHES_TOTAL: 3,
            CREATED_COUNT: 8,
            SKIPPED_COUNT: 0,
            ERROR_CODE: '',
            PROVIDER: 'anthropic',
          },
        }),
      })
    })

    expect(result.current.aiJob.target).toBe('QUIZ')
  })

  it('TARGET absent (clé non présente dans MSG — serveur antérieur à #203) devient \'QUIZ\', jamais undefined/null/\'\' (contrat §6 "additif, absent ⇒ QUIZ")', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/admin'))

    await act(async () => {
      wsInstance.onmessage({
        data: JSON.stringify({
          ACTION: 'AI_GENERATION_PROGRESS',
          MSG: {
            JOB_ID: 'gen-20260901-000002-e5f6',
            STATE: 'RUNNING',
            BATCHES_DONE: 1,
            BATCHES_TOTAL: 3,
            CREATED_COUNT: 8,
            SKIPPED_COUNT: 0,
            ERROR_CODE: '',
            PROVIDER: 'anthropic',
            // Pas de TARGET — serveur pré-#203.
          },
        }),
      })
    })

    expect(result.current.aiJob.target).toBe('QUIZ')
  })

  it('un job RAFALE ultérieur écrase un précédent target QUIZ — STATE/JOB_ID font autorité, pas de fusion résiduelle', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/admin'))

    await act(async () => {
      wsInstance.onmessage({
        data: JSON.stringify({
          ACTION: 'AI_GENERATION_PROGRESS',
          MSG: { JOB_ID: 'job-1', TARGET: 'QUIZ', STATE: 'DONE', CREATED_COUNT: 5 },
        }),
      })
    })
    expect(result.current.aiJob.target).toBe('QUIZ')

    await act(async () => {
      wsInstance.onmessage({
        data: JSON.stringify({
          ACTION: 'AI_GENERATION_PROGRESS',
          MSG: { JOB_ID: 'job-2', TARGET: 'RAFALE', STATE: 'RUNNING', CREATED_COUNT: 0 },
        }),
      })
    })
    expect(result.current.aiJob.target).toBe('RAFALE')
    expect(result.current.aiJob.jobId).toBe('job-2')
  })
})
