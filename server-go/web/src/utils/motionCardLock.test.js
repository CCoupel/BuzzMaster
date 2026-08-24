import { describe, it, expect } from 'vitest'
import { isMotionCardTypeLocked, motionCardLockReason } from './motionCardLock'

// Valeurs de création réelles d'une carte SPEEDY/QCM neuve — reproduites de
// QuestionsPage.jsx (`formData` initial / `handleAddMotionCard`), PAS
// réinventées, pour que ce test retombe si l'une des deux dérive de l'autre.
const NEW_SPEEDY_CARD = {
  id: 'mc-1',
  type: 'SPEEDY',
  rectoTheme: '',
  rectoImage: null,
  difficulty: 1,
  questionText: '',
  questionImage: null,
  answerText: '',
  answerImage: null,
}

const NEW_QCM_CARD = {
  id: 'mc-1',
  type: 'QCM',
  rectoTheme: '',
  rectoImage: null,
  difficulty: 1,
  questionText: '',
  questionImage: null,
  qcmAnswers: { RED: '', GREEN: '', YELLOW: '', BLUE: '' },
  qcmCorrect: '',
  qcmHintsEnabled: false,
  qcmHintThreshold1: 0.25,
  qcmHintThreshold2: 0.125,
  qcmPenalty1: 0.67,
  qcmPenalty2: 0.33,
}

describe('isMotionCardTypeLocked — carte neuve, déverrouillée', () => {
  it('carte SPEEDY neuve : déverrouillée', () => {
    expect(isMotionCardTypeLocked(NEW_SPEEDY_CARD)).toBe(false)
  })

  // Piège explicite du contrat §3.2 : QCM_HINT_THRESHOLD_1 (0.25) et
  // QCM_PENALTY_1 (0.67) naissent NON vides — un prédicat par non-nullité
  // verrouillerait cette carte dès sa création.
  it('carte QCM neuve : déverrouillée malgré des OwnedFields non vides (thresholds/pénalités par défaut)', () => {
    expect(isMotionCardTypeLocked(NEW_QCM_CARD)).toBe(false)
  })

  it('carte sans `type` explicite (legacy) : traitée comme SPEEDY, déverrouillée si vierge', () => {
    const legacyCard = { ...NEW_SPEEDY_CARD, type: undefined }
    expect(isMotionCardTypeLocked(legacyCard)).toBe(false)
  })
})

describe('isMotionCardTypeLocked — champs communs, ne verrouillent jamais', () => {
  it('thème + difficulté renseignés (SPEEDY) : toujours déverrouillée', () => {
    const card = { ...NEW_SPEEDY_CARD, rectoTheme: 'Capitales', difficulty: 3 }
    expect(isMotionCardTypeLocked(card)).toBe(false)
  })

  it('énoncé (questionText) renseigné (QCM) : toujours déverrouillée', () => {
    const card = { ...NEW_QCM_CARD, questionText: 'Quelle est la capitale ?' }
    expect(isMotionCardTypeLocked(card)).toBe(false)
  })
})

describe('isMotionCardTypeLocked — contenu propre au type, verrouille', () => {
  it('SPEEDY : answerText renseigné → verrouillée', () => {
    const card = { ...NEW_SPEEDY_CARD, answerText: 'Ljubljana' }
    expect(isMotionCardTypeLocked(card)).toBe(true)
  })

  it('SPEEDY : answerImage renseignée → verrouillée', () => {
    const card = { ...NEW_SPEEDY_CARD, answerImage: 'data:image/png;base64,xxx' }
    expect(isMotionCardTypeLocked(card)).toBe(true)
  })

  it('QCM : une réponse saisie → verrouillée', () => {
    const card = { ...NEW_QCM_CARD, qcmAnswers: { ...NEW_QCM_CARD.qcmAnswers, RED: 'Bratislava' } }
    expect(isMotionCardTypeLocked(card)).toBe(true)
  })

  it('QCM : désignation de la bonne réponse (qcmCorrect) → verrouillée', () => {
    const card = { ...NEW_QCM_CARD, qcmCorrect: 'GREEN' }
    expect(isMotionCardTypeLocked(card)).toBe(true)
  })

  it('QCM : seuil d\'indice déplacé → verrouillée', () => {
    const card = { ...NEW_QCM_CARD, qcmHintThreshold1: 0.4 }
    expect(isMotionCardTypeLocked(card)).toBe(true)
  })

  it('QCM : indices activés (qcmHintsEnabled) → verrouillée', () => {
    const card = { ...NEW_QCM_CARD, qcmHintsEnabled: true }
    expect(isMotionCardTypeLocked(card)).toBe(true)
  })
})

describe('isMotionCardTypeLocked — verrou réactif', () => {
  it('retour aux valeurs de création : redevient déverrouillée', () => {
    const locked = { ...NEW_SPEEDY_CARD, answerText: 'Ljubljana' }
    expect(isMotionCardTypeLocked(locked)).toBe(true)
    const cleared = { ...locked, answerText: '' }
    expect(isMotionCardTypeLocked(cleared)).toBe(false)
  })
})

describe('motionCardLockReason', () => {
  it('SPEEDY : mentionne la face RÉPONSE', () => {
    expect(motionCardLockReason(NEW_SPEEDY_CARD)).toMatch(/réponse/i)
  })

  it('QCM : mentionne les réponses QCM', () => {
    expect(motionCardLockReason(NEW_QCM_CARD)).toMatch(/QCM/)
  })
})
