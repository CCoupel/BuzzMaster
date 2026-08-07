/**
 * Tests — useWebSocket : mapping AI_GENERATION_PROGRESS.ERROR_MESSAGE
 * (issue #142, verbosité des erreurs de génération IA).
 *
 * Contexte (_work/handoff/task-test-writer-post-qualif-fixes-20260807-104945.md,
 * point 2) : dev-frontend a ajouté 2 tests dans AIGenerateModal.progress.test.jsx
 * qui couvrent bien le rendu du bloc "Détail technique" repliable — mais ils
 * construisent `aiJob` à la main (`runningJob({ errorMessage: '...' })`) et ne
 * passent jamais par le vrai code de mapping WS. La ligne réellement ajoutée
 * par ce lot (`hooks/useWebSocket.js:580`, `errorMessage: MSG.ERROR_MESSAGE
 * || ''`) n'avait donc aucun test — un bug dans ce mapping (ex: mauvaise clé,
 * `??` au lieu de `||` laissant passer une chaîne vide comme "présente")
 * serait invisible à la suite existante. Complète la chaîne de vérification
 * bout-en-bout : backend (5fb9db6, ai_error_verbosity_test.go) → hook WS (ici)
 * → rendu (AIGenerateModal.progress.test.jsx, déjà en place).
 *
 * Suit le pattern de mocks de useWebSocket.ardoise.test.js.
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

describe('useWebSocket — AI_GENERATION_PROGRESS.ERROR_MESSAGE mapping (issue #142)', () => {
  beforeEach(() => {
    wsInstance = null
    vi.stubGlobal('WebSocket', MockWebSocket)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    vi.clearAllMocks()
  })

  it('aiJob.errorMessage est vide (null) tant qu\'aucun AI_GENERATION_PROGRESS n\'est reçu', () => {
    const { result } = renderHook(() => useWebSocket('/ws/admin'))

    expect(result.current.aiJob).toBeNull()
  })

  it('ERROR_MESSAGE présent (STATE=FAILED) est mappé tel quel dans aiJob.errorMessage', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/admin'))

    await act(async () => {
      wsInstance.onmessage({
        data: JSON.stringify({
          ACTION: 'AI_GENERATION_PROGRESS',
          MSG: {
            JOB_ID: 'gen-20260807-000000-a1b2',
            STATE: 'FAILED',
            BATCHES_DONE: 2,
            BATCHES_TOTAL: 5,
            CREATED_COUNT: 40,
            SKIPPED_COUNT: 0,
            ERROR_CODE: 'upstream_error',
            ERROR_MESSAGE: 'discriminator: multiple candidate properties CATEGORY, DIFFICULTY, TYPE',
            PROVIDER: 'groq',
          },
        }),
      })
    })

    expect(result.current.aiJob.errorMessage).toBe(
      'discriminator: multiple candidate properties CATEGORY, DIFFICULTY, TYPE'
    )
    expect(result.current.aiJob.state).toBe('FAILED')
  })

  it('ERROR_MESSAGE absent (clé non présente dans MSG — serveur antérieur à #142, ou état non-FAILED) devient \'\' et pas undefined/null', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/admin'))

    await act(async () => {
      wsInstance.onmessage({
        data: JSON.stringify({
          ACTION: 'AI_GENERATION_PROGRESS',
          MSG: {
            JOB_ID: 'gen-20260807-000001-c3d4',
            STATE: 'RUNNING',
            BATCHES_DONE: 1,
            BATCHES_TOTAL: 5,
            CREATED_COUNT: 8,
            SKIPPED_COUNT: 0,
            ERROR_CODE: '',
            PROVIDER: 'groq',
            // Pas de ERROR_MESSAGE — omitempty côté serveur sur les états non-FAILED.
          },
        }),
      })
    })

    expect(result.current.aiJob.errorMessage).toBe('')
  })

  it('un job FAILED ultérieur SANS ERROR_MESSAGE écrase un précédent errorMessage — STATE fait autorité, pas de fusion résiduelle', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/admin'))

    await act(async () => {
      wsInstance.onmessage({
        data: JSON.stringify({
          ACTION: 'AI_GENERATION_PROGRESS',
          MSG: { JOB_ID: 'job-1', STATE: 'FAILED', ERROR_CODE: 'upstream_error', ERROR_MESSAGE: 'détail du premier job' },
        }),
      })
    })
    expect(result.current.aiJob.errorMessage).toBe('détail du premier job')

    // Nouveau job (JOB_ID différent), FAILED lui aussi mais sans détail —
    // le hook remplace intégralement l'objet aiJob (pas de fusion avec l'ancien).
    await act(async () => {
      wsInstance.onmessage({
        data: JSON.stringify({
          ACTION: 'AI_GENERATION_PROGRESS',
          MSG: { JOB_ID: 'job-2', STATE: 'FAILED', ERROR_CODE: 'timeout' },
        }),
      })
    })
    expect(result.current.aiJob.errorMessage).toBe('')
    expect(result.current.aiJob.jobId).toBe('job-2')
  })
})
