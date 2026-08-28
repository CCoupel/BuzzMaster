import { describe, it, expect } from 'vitest'
import { getTypeState } from './typeState'

// ---------------------------------------------------------------------------
// typeState — #184/B-F1, étendu #187 (memory). `getTypeState` est le SEUL
// point du code autorisé à choisir entre les champs question-scopés et
// `MEMOTION_ACTIVE.STATE` (contrat question-types.md §5.3). Ces tests figent
// ce choix ; tout composant de type doit passer par cet accesseur.
// ---------------------------------------------------------------------------

const EMPTY_MEMORY = { flippedCards: [], matchedPairs: [], errors: 0 }

describe('getTypeState — hôte question (cardId vide)', () => {
  it('lit qcmInvalidated directement sur gameState, jamais MEMOTION_ACTIVE', () => {
    const gameState = {
      qcmInvalidated: ['RED', 'YELLOW'],
      MEMOTION_ACTIVE: { CARD_ID: 'mc-9', TYPE: 'QCM', STATE: { QCM_INVALIDATED: ['BLUE'] } },
    }
    const hostContext = { playable: true, revealed: false, timerRunning: true, cardId: '' }
    expect(getTypeState(gameState, hostContext)).toEqual({ qcmInvalidated: ['RED', 'YELLOW'], memory: EMPTY_MEMORY })
  })

  it('gameState.qcmInvalidated absent → tableau vide, pas d’exception', () => {
    const hostContext = { playable: true, revealed: false, timerRunning: true, cardId: '' }
    expect(getTypeState({}, hostContext)).toEqual({ qcmInvalidated: [], memory: EMPTY_MEMORY })
  })

  // #187 — état MEMORY question-scopé (memoryFlippedCards/memoryMatchedPairs/
  // memoryErrors, noms camelCase posés par useWebSocket.js), jamais lu sur
  // MEMOTION_ACTIVE pour l'hôte question.
  it('#187 — lit memoryFlippedCards/memoryMatchedPairs/memoryErrors directement sur gameState', () => {
    const gameState = {
      memoryFlippedCards: ['1-1'],
      memoryMatchedPairs: [2, 4],
      memoryErrors: 3,
      MEMOTION_ACTIVE: { CARD_ID: 'mc-9', TYPE: 'MEMORY', STATE: { MEMORY_FLIPPED_CARDS: ['9-1'], MEMORY_MATCHED_PAIRS: [9], MEMORY_ERRORS: 99 } },
    }
    const hostContext = { playable: true, revealed: false, timerRunning: true, cardId: '' }
    expect(getTypeState(gameState, hostContext).memory).toEqual({ flippedCards: ['1-1'], matchedPairs: [2, 4], errors: 3 })
  })
})

describe('getTypeState — hôte carte MEMOTION (cardId renseigné)', () => {
  it('lit gameState.MEMOTION_ACTIVE.STATE quand CARD_ID correspond à hostContext.cardId', () => {
    const gameState = {
      qcmInvalidated: ['RED'], // champ question-scopé — ne doit PAS être lu ici
      MEMOTION_ACTIVE: { CARD_ID: 'mc-3', TYPE: 'QCM', STATE: { QCM_INVALIDATED: ['GREEN', 'YELLOW'] } },
    }
    const hostContext = { playable: true, revealed: false, timerRunning: true, cardId: 'mc-3' }
    expect(getTypeState(gameState, hostContext)).toEqual({ qcmInvalidated: ['GREEN', 'YELLOW'], memory: EMPTY_MEMORY })
  })

  it('MEMOTION_ACTIVE.CARD_ID ne correspond pas à hostContext.cardId (transition en cours) → état vide, pas l’ancien état', () => {
    const gameState = {
      MEMOTION_ACTIVE: { CARD_ID: 'mc-2', TYPE: 'QCM', STATE: { QCM_INVALIDATED: ['GREEN'] } },
    }
    const hostContext = { playable: true, revealed: false, timerRunning: true, cardId: 'mc-3' }
    expect(getTypeState(gameState, hostContext)).toEqual({ qcmInvalidated: [], memory: EMPTY_MEMORY })
  })

  it('MEMOTION_ACTIVE absent (champ pas encore câblé côté useWebSocket.js) → état vide, pas d’exception', () => {
    const hostContext = { playable: true, revealed: false, timerRunning: true, cardId: 'mc-3' }
    expect(getTypeState({}, hostContext)).toEqual({ qcmInvalidated: [], memory: EMPTY_MEMORY })
  })

  it('MEMOTION_ACTIVE.STATE vide ({}) → tableau vide', () => {
    const gameState = { MEMOTION_ACTIVE: { CARD_ID: 'mc-3', TYPE: '', STATE: {} } }
    const hostContext = { playable: false, revealed: false, timerRunning: false, cardId: 'mc-3' }
    expect(getTypeState(gameState, hostContext)).toEqual({ qcmInvalidated: [], memory: EMPTY_MEMORY })
  })

  // #187 — état MEMORY d'une carte MEMOTION, lu sur MEMOTION_ACTIVE.STATE,
  // JAMAIS sur les champs question-scopés (contrat game-state.md §"STATE
  // pour une carte TYPE=MEMORY").
  it('#187 — lit MEMORY_FLIPPED_CARDS/MEMORY_MATCHED_PAIRS/MEMORY_ERRORS sur MEMOTION_ACTIVE.STATE quand CARD_ID correspond', () => {
    const gameState = {
      memoryFlippedCards: ['1-1'], // question-scopé — ne doit PAS être lu ici
      memoryMatchedPairs: [1],
      memoryErrors: 7,
      MEMOTION_ACTIVE: {
        CARD_ID: 'card_3',
        TYPE: 'MEMORY',
        STATE: { MEMORY_FLIPPED_CARDS: ['2-1'], MEMORY_MATCHED_PAIRS: [1, 4], MEMORY_ERRORS: 3 },
      },
    }
    const hostContext = { playable: true, revealed: false, timerRunning: true, cardId: 'card_3' }
    expect(getTypeState(gameState, hostContext).memory).toEqual({ flippedCards: ['2-1'], matchedPairs: [1, 4], errors: 3 })
  })

  it('#187 — garde-fou : CARD_ID ne correspond pas -> état MEMORY vide, pas l’ancienne carte', () => {
    const gameState = {
      MEMOTION_ACTIVE: { CARD_ID: 'card_2', TYPE: 'MEMORY', STATE: { MEMORY_FLIPPED_CARDS: ['1-1'], MEMORY_MATCHED_PAIRS: [1], MEMORY_ERRORS: 2 } },
    }
    const hostContext = { playable: true, revealed: false, timerRunning: true, cardId: 'card_3' }
    expect(getTypeState(gameState, hostContext).memory).toEqual(EMPTY_MEMORY)
  })
})
