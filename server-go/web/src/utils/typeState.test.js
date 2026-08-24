import { describe, it, expect } from 'vitest'
import { getTypeState } from './typeState'

// ---------------------------------------------------------------------------
// typeState — #184/B-F1. `getTypeState` est le SEUL point du code autorisé à
// choisir entre les champs question-scopés et `MEMOTION_ACTIVE.STATE`
// (contrat question-types.md §5.3). Ces tests figent ce choix ; tout
// composant de type doit passer par cet accesseur.
// ---------------------------------------------------------------------------

describe('getTypeState — hôte question (cardId vide)', () => {
  it('lit qcmInvalidated directement sur gameState, jamais MEMOTION_ACTIVE', () => {
    const gameState = {
      qcmInvalidated: ['RED', 'YELLOW'],
      MEMOTION_ACTIVE: { CARD_ID: 'mc-9', TYPE: 'QCM', STATE: { QCM_INVALIDATED: ['BLUE'] } },
    }
    const hostContext = { playable: true, revealed: false, timerRunning: true, cardId: '' }
    expect(getTypeState(gameState, hostContext)).toEqual({ qcmInvalidated: ['RED', 'YELLOW'] })
  })

  it('gameState.qcmInvalidated absent → tableau vide, pas d’exception', () => {
    const hostContext = { playable: true, revealed: false, timerRunning: true, cardId: '' }
    expect(getTypeState({}, hostContext)).toEqual({ qcmInvalidated: [] })
  })
})

describe('getTypeState — hôte carte MEMOTION (cardId renseigné)', () => {
  it('lit gameState.MEMOTION_ACTIVE.STATE quand CARD_ID correspond à hostContext.cardId', () => {
    const gameState = {
      qcmInvalidated: ['RED'], // champ question-scopé — ne doit PAS être lu ici
      MEMOTION_ACTIVE: { CARD_ID: 'mc-3', TYPE: 'QCM', STATE: { QCM_INVALIDATED: ['GREEN', 'YELLOW'] } },
    }
    const hostContext = { playable: true, revealed: false, timerRunning: true, cardId: 'mc-3' }
    expect(getTypeState(gameState, hostContext)).toEqual({ qcmInvalidated: ['GREEN', 'YELLOW'] })
  })

  it('MEMOTION_ACTIVE.CARD_ID ne correspond pas à hostContext.cardId (transition en cours) → état vide, pas l’ancien état', () => {
    const gameState = {
      MEMOTION_ACTIVE: { CARD_ID: 'mc-2', TYPE: 'QCM', STATE: { QCM_INVALIDATED: ['GREEN'] } },
    }
    const hostContext = { playable: true, revealed: false, timerRunning: true, cardId: 'mc-3' }
    expect(getTypeState(gameState, hostContext)).toEqual({ qcmInvalidated: [] })
  })

  it('MEMOTION_ACTIVE absent (champ pas encore câblé côté useWebSocket.js) → état vide, pas d’exception', () => {
    const hostContext = { playable: true, revealed: false, timerRunning: true, cardId: 'mc-3' }
    expect(getTypeState({}, hostContext)).toEqual({ qcmInvalidated: [] })
  })

  it('MEMOTION_ACTIVE.STATE vide ({}) → tableau vide', () => {
    const gameState = { MEMOTION_ACTIVE: { CARD_ID: 'mc-3', TYPE: '', STATE: {} } }
    const hostContext = { playable: false, revealed: false, timerRunning: false, cardId: 'mc-3' }
    expect(getTypeState(gameState, hostContext)).toEqual({ qcmInvalidated: [] })
  })
})
