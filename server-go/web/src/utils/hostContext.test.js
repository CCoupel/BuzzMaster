import { describe, it, expect } from 'vitest'
import { resolveHostContext } from './hostContext'

// ---------------------------------------------------------------------------
// hostContext — #184/B-F1. Cas nommés à l'identique de la table de
// dérivation du contrat (contracts/question-types.md §4) et des tests Go
// équivalents (dev-backend, B-B3) : "hôte question / playable",
// "hôte question / revealed", "hôte carte MEMOTION / playable",
// "hôte carte MEMOTION / revealed", "aucun hôte actif".
// ---------------------------------------------------------------------------

describe('resolveHostContext — hôte question', () => {
  it('hôte question / playable : PHASE=STARTED, MEMOTION_SUBPHASE vide', () => {
    const ctx = resolveHostContext({ phase: 'STARTED', MEMOTION_SUBPHASE: '', timer: 12 })
    expect(ctx).toEqual({ playable: true, revealed: false, timerRunning: true, cardId: '' })
  })

  it('hôte question / revealed : PHASE=REVEALED', () => {
    const ctx = resolveHostContext({ phase: 'REVEALED', MEMOTION_SUBPHASE: '' })
    expect(ctx).toEqual({ playable: false, revealed: true, timerRunning: false, cardId: '' })
  })

  it('hôte question / ni playable ni revealed sur les autres phases (STOPPED, PAUSED, PREPARE, READY)', () => {
    for (const phase of ['STOPPED', 'PAUSED', 'PREPARE', 'READY', 'COUNTDOWN']) {
      const ctx = resolveHostContext({ phase, MEMOTION_SUBPHASE: '' })
      expect(ctx).toEqual({ playable: false, revealed: false, timerRunning: false, cardId: '' })
    }
  })

  it('MEMOTION_SUBPHASE absent (undefined) est traité comme hôte question, comme une chaîne vide', () => {
    const ctx = resolveHostContext({ phase: 'STARTED' })
    expect(ctx).toEqual({ playable: true, revealed: false, timerRunning: true, cardId: '' })
  })
})

describe('resolveHostContext — hôte carte MEMOTION', () => {
  it('hôte carte MEMOTION / playable : MEMOTION_SUBPHASE=QUESTION, timer actif', () => {
    const ctx = resolveHostContext({
      phase: 'STARTED',
      MEMOTION_SUBPHASE: 'QUESTION',
      MEMOTION_SELECTED: 'mc-3',
      timer: 8,
    })
    expect(ctx).toEqual({ playable: true, revealed: false, timerRunning: true, cardId: 'mc-3' })
  })

  it('hôte carte MEMOTION / QUESTION mais chrono à zéro : timerRunning false, toujours playable', () => {
    const ctx = resolveHostContext({
      phase: 'STARTED',
      MEMOTION_SUBPHASE: 'QUESTION',
      MEMOTION_SELECTED: 'mc-3',
      timer: 0,
    })
    expect(ctx).toEqual({ playable: true, revealed: false, timerRunning: false, cardId: 'mc-3' })
  })

  it('hôte carte MEMOTION / revealed : MEMOTION_SUBPHASE=REVEAL', () => {
    const ctx = resolveHostContext({
      phase: 'STARTED',
      MEMOTION_SUBPHASE: 'REVEAL',
      MEMOTION_SELECTED: 'mc-3',
      timer: 0,
    })
    expect(ctx).toEqual({ playable: false, revealed: true, timerRunning: false, cardId: 'mc-3' })
  })

  it('PHASE ne doit jamais être lu pendant une sous-phase carte active (STARTED constant tout au long de la manche)', () => {
    // Piège explicite : si l'implémentation retombait sur `phase === 'STARTED'`
    // pour `playable`, ce cas resterait vrai par accident. On le fait échouer
    // en testant aussi REVEAL, où `playable` doit être false malgré phase=STARTED.
    const ctx = resolveHostContext({ phase: 'STARTED', MEMOTION_SUBPHASE: 'REVEAL', MEMOTION_SELECTED: 'mc-3' })
    expect(ctx.playable).toBe(false)
  })
})

describe('resolveHostContext — aucun hôte actif', () => {
  it('aucun hôte actif : MEMORIZE — ni playable ni revealed malgré PHASE=STARTED', () => {
    const ctx = resolveHostContext({ phase: 'STARTED', MEMOTION_SUBPHASE: 'MEMORIZE' })
    expect(ctx).toEqual({ playable: false, revealed: false, timerRunning: false, cardId: '' })
  })

  it('aucun hôte actif : GRID — ni playable ni revealed, cardId vide (aucune carte sélectionnée)', () => {
    const ctx = resolveHostContext({ phase: 'STARTED', MEMOTION_SUBPHASE: 'GRID', MEMOTION_SELECTED: '' })
    expect(ctx).toEqual({ playable: false, revealed: false, timerRunning: false, cardId: '' })
  })

  it('aucun hôte actif : SELECTED — ni playable ni revealed, mais cardId renseigné ("selon le cas", contrat §4)', () => {
    const ctx = resolveHostContext({ phase: 'STARTED', MEMOTION_SUBPHASE: 'SELECTED', MEMOTION_SELECTED: 'mc-3' })
    expect(ctx).toEqual({ playable: false, revealed: false, timerRunning: false, cardId: 'mc-3' })
  })
})

describe('resolveHostContext — robustesse', () => {
  it('gameState minimal ({}) ne lève pas et retombe sur l’hôte question au repos', () => {
    expect(resolveHostContext({})).toEqual({ playable: false, revealed: false, timerRunning: false, cardId: '' })
  })
})
