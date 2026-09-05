import { describe, it, expect } from 'vitest'
import { getTypeState } from './typeState'

// ---------------------------------------------------------------------------
// typeState — #184/B-F1, étendu #187 (memory). `getTypeState` est le SEUL
// point du code autorisé à choisir entre les champs question-scopés et
// `MEMOTION_ACTIVE.STATE` (contrat question-types.md §5.3). Ces tests figent
// ce choix ; tout composant de type doit passer par cet accesseur.
// ---------------------------------------------------------------------------

const EMPTY_MEMORY = { flippedCards: [], matchedPairs: [], errors: 0 }
// #217 — état RAFALE d'une carte MEMOTION, ajouté à getTypeState. Les tests
// de ce fichier antérieurs à #217 vérifient l'objet ENTIER par égalité
// stricte (toEqual) : chacun gagne ce champ vide pour rester exact plutôt
// que de basculer sur un `expect.objectContaining` qui masquerait une
// régression de forme sur les DEUX autres champs (qcmInvalidated/memory).
const EMPTY_RAFALE = { subphase: '', currentQuestion: {}, questionTime: 0, askedCount: 0, correctCount: 0, poolRemaining: 0, exhausted: false }

describe('getTypeState — hôte question (cardId vide)', () => {
  it('lit qcmInvalidated directement sur gameState, jamais MEMOTION_ACTIVE', () => {
    const gameState = {
      qcmInvalidated: ['RED', 'YELLOW'],
      MEMOTION_ACTIVE: { CARD_ID: 'mc-9', TYPE: 'QCM', STATE: { QCM_INVALIDATED: ['BLUE'] } },
    }
    const hostContext = { playable: true, revealed: false, timerRunning: true, cardId: '' }
    expect(getTypeState(gameState, hostContext)).toEqual({ qcmInvalidated: ['RED', 'YELLOW'], memory: EMPTY_MEMORY, rafale: EMPTY_RAFALE })
  })

  it('gameState.qcmInvalidated absent → tableau vide, pas d’exception', () => {
    const hostContext = { playable: true, revealed: false, timerRunning: true, cardId: '' }
    expect(getTypeState({}, hostContext)).toEqual({ qcmInvalidated: [], memory: EMPTY_MEMORY, rafale: EMPTY_RAFALE })
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
    expect(getTypeState(gameState, hostContext)).toEqual({ qcmInvalidated: ['GREEN', 'YELLOW'], memory: EMPTY_MEMORY, rafale: EMPTY_RAFALE })
  })

  it('MEMOTION_ACTIVE.CARD_ID ne correspond pas à hostContext.cardId (transition en cours) → état vide, pas l’ancien état', () => {
    const gameState = {
      MEMOTION_ACTIVE: { CARD_ID: 'mc-2', TYPE: 'QCM', STATE: { QCM_INVALIDATED: ['GREEN'] } },
    }
    const hostContext = { playable: true, revealed: false, timerRunning: true, cardId: 'mc-3' }
    expect(getTypeState(gameState, hostContext)).toEqual({ qcmInvalidated: [], memory: EMPTY_MEMORY, rafale: EMPTY_RAFALE })
  })

  it('MEMOTION_ACTIVE absent (champ pas encore câblé côté useWebSocket.js) → état vide, pas d’exception', () => {
    const hostContext = { playable: true, revealed: false, timerRunning: true, cardId: 'mc-3' }
    expect(getTypeState({}, hostContext)).toEqual({ qcmInvalidated: [], memory: EMPTY_MEMORY, rafale: EMPTY_RAFALE })
  })

  it('MEMOTION_ACTIVE.STATE vide ({}) → tableau vide', () => {
    const gameState = { MEMOTION_ACTIVE: { CARD_ID: 'mc-3', TYPE: '', STATE: {} } }
    const hostContext = { playable: false, revealed: false, timerRunning: false, cardId: 'mc-3' }
    expect(getTypeState(gameState, hostContext)).toEqual({ qcmInvalidated: [], memory: EMPTY_MEMORY, rafale: EMPTY_RAFALE })
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

// ---------------------------------------------------------------------------
// #217 (milestone v9.0.0, second passage) — RAFALE nestable en carte
// MEMOTION. Aucun test dédié n'existait encore : les fichiers touchés par
// #217 ne faisaient que mettre à jour les assertions EXISTANTES (memory/QCM)
// pour inclure le nouveau champ `rafale: EMPTY_RAFALE` — le comportement
// RÉEL de cet accesseur pour RAFALE n'était jamais exercé. Point central :
// une manche RAFALE CLASSIQUE (hôte question) est déjà lue directement
// depuis les champs globaux `gameState.RAFALE_*` par PlayerDisplay.jsx —
// elle ne passe JAMAIS par `getTypeState` (contrat rafale.md §14.2). Le
// risque de confusion signalé au second passage est donc : si un
// `gameState` porte À LA FOIS des champs globaux RAFALE_* peuplés (résidu
// d'une manche classique) ET un MEMOTION_ACTIVE pour une carte RAFALE,
// `getTypeState` en hôte carte doit ignorer totalement les champs globaux.
// ---------------------------------------------------------------------------

describe('getTypeState — RAFALE en carte MEMOTION (#217)', () => {
  it('hôte carte : lit RAFALE_* depuis MEMOTION_ACTIVE.STATE, pas les champs globaux', () => {
    const gameState = {
      MEMOTION_ACTIVE: {
        CARD_ID: 'mc-rafale-1',
        TYPE: 'RAFALE',
        STATE: {
          RAFALE_SUBPHASE: 'QUESTION',
          RAFALE_CURRENT_QUESTION: { ID: 'r-9', QUESTION: 'Capitale ?', CATEGORY: 'SCIENCE', DIFFICULTY: 2 },
          RAFALE_QUESTION_TIME: 2,
          RAFALE_ASKED_COUNT: 3,
          RAFALE_CORRECT_COUNT: 2,
          RAFALE_POOL_REMAINING: 5,
          RAFALE_EXHAUSTED: false,
        },
      },
    }
    const hostContext = { playable: true, revealed: false, timerRunning: true, cardId: 'mc-rafale-1' }
    expect(getTypeState(gameState, hostContext).rafale).toEqual({
      subphase: 'QUESTION',
      currentQuestion: { ID: 'r-9', QUESTION: 'Capitale ?', CATEGORY: 'SCIENCE', DIFFICULTY: 2 },
      questionTime: 2,
      askedCount: 3,
      correctCount: 2,
      poolRemaining: 5,
      exhausted: false,
    })
  })

  it('⚠️ non-confusion (second passage) : une manche RAFALE classique peuplée dans les champs globaux ne fuite JAMAIS dans l\'état carte', () => {
    const gameState = {
      // Résidu d'une manche RAFALE CLASSIQUE (hôte question) — jamais lu par
      // getTypeState en hôte carte, contrairement aux champs MEMOTION_ACTIVE.
      RAFALE_CURRENT_QUESTION: { ID: 'classic-999', QUESTION: 'CE CONTENU NE DOIT JAMAIS APPARAÎTRE ICI', CATEGORY: 'HISTORY', DIFFICULTY: 3 },
      RAFALE_ASKED_COUNT: 42,
      RAFALE_SUBPHASE: 'QUESTION',
      MEMOTION_ACTIVE: {
        CARD_ID: 'mc-rafale-2',
        TYPE: 'RAFALE',
        STATE: {
          RAFALE_SUBPHASE: 'QUESTION',
          RAFALE_CURRENT_QUESTION: { ID: 'card-1', QUESTION: 'Question de la carte', CATEGORY: 'SCIENCE', DIFFICULTY: 1 },
          RAFALE_ASKED_COUNT: 1,
        },
      },
    }
    const hostContext = { playable: true, revealed: false, timerRunning: true, cardId: 'mc-rafale-2' }
    const rafale = getTypeState(gameState, hostContext).rafale
    expect(rafale.currentQuestion.ID).toBe('card-1')
    expect(rafale.currentQuestion.QUESTION).not.toContain('NE DOIT JAMAIS APPARAÎTRE')
    expect(rafale.askedCount).toBe(1)
  })

  it('garde-fou CARD_ID (même discipline que MEMORY) : carte différente -> état RAFALE vide, pas l\'ancienne carte', () => {
    const gameState = {
      MEMOTION_ACTIVE: {
        CARD_ID: 'mc-old',
        TYPE: 'RAFALE',
        STATE: { RAFALE_ASKED_COUNT: 7, RAFALE_CURRENT_QUESTION: { ID: 'old-q' } },
      },
    }
    const hostContext = { playable: true, revealed: false, timerRunning: true, cardId: 'mc-new' }
    expect(getTypeState(gameState, hostContext).rafale).toEqual(EMPTY_RAFALE)
  })

  it('hôte question (cardId vide) : rafale toujours vide, même si MEMOTION_ACTIVE porte une carte RAFALE en parallèle', () => {
    const gameState = {
      MEMOTION_ACTIVE: {
        CARD_ID: 'mc-rafale-3',
        TYPE: 'RAFALE',
        STATE: { RAFALE_ASKED_COUNT: 5, RAFALE_CURRENT_QUESTION: { ID: 'card-q' } },
      },
    }
    const hostContext = { playable: true, revealed: false, timerRunning: true, cardId: '' }
    expect(getTypeState(gameState, hostContext).rafale).toEqual(EMPTY_RAFALE)
  })
})
