import { describe, it, expect, vi } from 'vitest'
import { render } from '@testing-library/react'
import AnimConductPanel from './AnimConductPanel'

// ---------------------------------------------------------------------------
// AnimConductPanel — L2, gain affiché sur le bouton de crédit (sous-phase
// REVEAL) selon la règle POINTS_RULE de la carte (#190, LOT-5, v11.2.2,
// contrat question-types.md §6.2 « Parité d'affichage »).
//
// Avant #190, seul STARS_PRORATA était géré : une carte FIXED ou PER_UNIT
// affichait la valeur étoiles PLEINE — montant FAUX par rapport au crédit réel
// du serveur (seule autorité). La règle est désormais résolue par la fonction
// unique miroir de `motionCardPointsForOutcome` :
//   FIXED     → VALUE si Units > 0, sinon 0       (binaire : Units = 1)
//   PER_UNIT  → VALUE × Units
//   STARS_PRORATA → étoiles × Units / UnitsTotal  (inchangé, #187 cycle 4 / C3)
//   STARS     → étoiles pleines                   (inchangé)
// Units / UnitsTotal : MEMORY = paires trouvées / paires de la carte ;
// RAFALE = bonnes réponses / questions posées ; binaire = 1.
//
// Les NR existants (AnimConductPanel.test.jsx :1076 MEMORY, :1195 RAFALE —
// prorata et surcharge STARS) ne sont PAS modifiés ; quelques cas sont repris
// ici en non-régression du remplacement de `isStarsProrataCard`.
// ---------------------------------------------------------------------------

function baseProps(overrides = {}) {
  return {
    phase: 'STOPPED',
    question: null,
    qcmInvalidated: [],
    revealed: false,
    nextQuestion: null,
    onStart: vi.fn(),
    onPause: vi.fn(),
    onContinue: vi.fn(),
    onStop: vi.fn(),
    onReveal: vi.fn(),
    onSelectNext: vi.fn(),
    ...overrides,
  }
}

function motionQuestion(cards, overrides = {}) {
  return { ID: '1', TYPE: 'MEMOTION', MOTION_CARDS: cards, ...overrides }
}

function motionProps(overrides = {}) {
  return {
    subphase: 'REVEAL',
    cardStates: {},
    cardTeams: {},
    currentTeam: 'Les Rouges',
    currentTeamColor: null,
    selectedId: 'c2',
    participatingTeams: [],
    ...overrides,
  }
}

const SPEEDY_FIRST = { ID: 'c1', RECTO_THEME: 'Cinéma', DIFFICULTY: 1 }

// Carte active c2 en REVEAL ; `extra` = props de type (cardMemory / cardRafale).
function renderReveal(card, extra = {}, questionOverrides = {}) {
  return render(
    <AnimConductPanel {...baseProps({
      phase: 'STARTED',
      question: motionQuestion([SPEEDY_FIRST, { ID: 'c2', RECTO_THEME: 'Carte', DIFFICULTY: 2, ...card }], questionOverrides),
      hostContext: { playable: false, revealed: true, cardId: 'c2' },
      motion: motionProps(),
      ...extra,
    })} />
  )
}

function creditButton(container) {
  return Array.from(container.querySelectorAll('.anim-conduct-l2 .anim-conduct-btn'))
    .find(btn => btn.textContent.startsWith('Les Rouges'))
}

const memoryCard = (rule) => ({
  TYPE: 'MEMORY',
  MEMORY_PAIRS: [1, 2, 3].map(id => ({ ID: id, CARD1: { TEXT: 'Q' }, CARD2: { TEXT: 'R' } })),
  ...(rule ? { POINTS_RULE: rule } : {}),
})
const withMatched = (matchedPairs) => ({ cardMemory: { flippedCards: [], matchedPairs, errors: 0 } })

const rafaleCard = (rule) => ({
  TYPE: 'RAFALE',
  RAFALE_CATEGORIES: ['HISTORY'], RAFALE_DIFFICULTIES: [1], RAFALE_MODE: 'SOLO',
  RAFALE_QUESTION_TIME: 3, RAFALE_MAX_QUESTIONS: 10, RAFALE_ROUND_TIME: 60,
  ...(rule ? { POINTS_RULE: rule } : {}),
})
const withProgress = (correctCount, askedCount) => ({
  cardRafale: {
    current: { ID: 'rq1', QUESTION: 'Capitale du Japon ?', CATEGORY: 'HISTORY', DIFFICULTY: 1 },
    catMeta: null, answerValue: '', showNext: false, correctCount, askedCount,
  },
})

const qcmCard = (rule) => ({
  TYPE: 'QCM',
  QCM_ANSWERS: { RED: 'a', GREEN: 'b', YELLOW: 'c', BLUE: 'd' },
  QCM_CORRECT: 'GREEN',
  ...(rule ? { POINTS_RULE: rule } : {}),
})

// Carte à ★★ = 3 pts (repli 1/3/5, aucun MOTION_CONFIG).

describe('AnimConductPanel — gain REVEAL, carte de type binaire (#190)', () => {
  it('QCM, Montant fixe 10 : le bouton affiche +10 pts, pas la valeur étoiles (+3 pts)', () => {
    const { container } = renderReveal(qcmCard({ MODE: 'FIXED', VALUE: 10 }))
    const btn = creditButton(container)
    expect(btn).not.toBeUndefined()
    expect(btn.textContent).toContain('+10 pt')
    expect(btn.textContent).not.toMatch(/\+3 pts?/)
  })

  it('SPEEDY (sans TYPE), Montant fixe 4 : +4 pts', () => {
    const { container } = renderReveal({ POINTS_RULE: { MODE: 'FIXED', VALUE: 4 } })
    expect(creditButton(container).textContent).toContain('+4 pt')
  })

  it('Montant fixe ignore MOTION_CONFIG de la manche', () => {
    const { container } = renderReveal(qcmCard({ MODE: 'FIXED', VALUE: 10 }), {}, { MOTION_CONFIG: { POINTS_1_STAR: 9, POINTS_2_STAR: 9, POINTS_3_STAR: 9 } })
    expect(creditButton(container).textContent).toContain('+10 pt')
  })

  it('NR — QCM sans règle : valeur étoiles (★★ = +3 pts)', () => {
    const { container } = renderReveal(qcmCard())
    expect(creditButton(container).textContent).toContain('+3 pt')
  })

  it('NR — QCM avec STARS explicite : valeur étoiles (+3 pts)', () => {
    const { container } = renderReveal(qcmCard({ MODE: 'STARS' }))
    expect(creditButton(container).textContent).toContain('+3 pt')
  })
})

describe('AnimConductPanel — gain REVEAL, carte MEMORY (#190)', () => {
  it('Par unité 2, 3 paires trouvées : +6 pts', () => {
    const { container } = renderReveal(memoryCard({ MODE: 'PER_UNIT', VALUE: 2 }), withMatched([1, 2, 3]))
    expect(creditButton(container).textContent).toContain('+6 pt')
  })

  it('Par unité 2, 1 paire trouvée : +2 pts (VALUE × paires trouvées, pas le prorata)', () => {
    const { container } = renderReveal(memoryCard({ MODE: 'PER_UNIT', VALUE: 2 }), withMatched([1]))
    expect(creditButton(container).textContent).toContain('+2 pt')
  })

  it('Par unité, aucune paire trouvée : +0 pt', () => {
    const { container } = renderReveal(memoryCard({ MODE: 'PER_UNIT', VALUE: 2 }), withMatched([]))
    expect(creditButton(container).textContent).toContain('+0 pt')
  })

  it('Montant fixe 7, au moins une paire trouvée : +7 pts', () => {
    const { container } = renderReveal(memoryCard({ MODE: 'FIXED', VALUE: 7 }), withMatched([1]))
    expect(creditButton(container).textContent).toContain('+7 pt')
  })

  it('Montant fixe 7, aucune paire trouvée : +0 pt (FIXED = VALUE seulement si Units > 0)', () => {
    const { container } = renderReveal(memoryCard({ MODE: 'FIXED', VALUE: 7 }), withMatched([]))
    expect(creditButton(container).textContent).toContain('+0 pt')
  })

  it('NR — défaut (aucune règle), 1/3 paires : prorata inchangé (+1 pt)', () => {
    const { container } = renderReveal(memoryCard(), withMatched([1]))
    const text = creditButton(container).textContent
    expect(text).toContain('+1 pt')
    expect(text).not.toMatch(/\+3 pts?/)
  })

  it('NR — STARS_PRORATA explicite, 1/3 paires : même montant que le défaut (+1 pt)', () => {
    const { container } = renderReveal(memoryCard({ MODE: 'STARS_PRORATA' }), withMatched([1]))
    expect(creditButton(container).textContent).toContain('+1 pt')
  })

  it('NR — surcharge explicite STARS (tout-ou-rien), 1/3 paires : montant plein (+3 pts)', () => {
    const { container } = renderReveal(memoryCard({ MODE: 'STARS' }), withMatched([1]))
    expect(creditButton(container).textContent).toContain('+3 pt')
  })
})

describe('AnimConductPanel — gain REVEAL, carte RAFALE (#190)', () => {
  it('Par unité 2, 5 bonnes réponses sur 7 posées : +10 pts', () => {
    const { container } = renderReveal(rafaleCard({ MODE: 'PER_UNIT', VALUE: 2 }), withProgress(5, 7))
    expect(creditButton(container).textContent).toContain('+10 pt')
  })

  it('Par unité 2, aucune bonne réponse : +0 pt', () => {
    const { container } = renderReveal(rafaleCard({ MODE: 'PER_UNIT', VALUE: 2 }), withProgress(0, 4))
    expect(creditButton(container).textContent).toContain('+0 pt')
  })

  it('Montant fixe 10, une bonne réponse : +10 pts', () => {
    const { container } = renderReveal(rafaleCard({ MODE: 'FIXED', VALUE: 10 }), withProgress(1, 3))
    expect(creditButton(container).textContent).toContain('+10 pt')
  })

  it('Montant fixe 10, aucune bonne réponse : +0 pt', () => {
    const { container } = renderReveal(rafaleCard({ MODE: 'FIXED', VALUE: 10 }), withProgress(0, 3))
    expect(creditButton(container).textContent).toContain('+0 pt')
  })

  it('NR — défaut (aucune règle), 2 bonnes réponses sur 3 : prorata inchangé (+2 pts)', () => {
    const { container } = renderReveal(rafaleCard(), withProgress(2, 3))
    expect(creditButton(container).textContent).toContain('+2 pt')
  })

  it('NR — surcharge explicite STARS, round incomplet : montant plein (+3 pts)', () => {
    const { container } = renderReveal(rafaleCard({ MODE: 'STARS' }), withProgress(1, 3))
    expect(creditButton(container).textContent).toContain('+3 pt')
  })

  it('NR — UnitsTotal = 0 (aucune question posée), prorata par défaut : +0 pt, jamais de division par zéro', () => {
    const { container } = renderReveal(rafaleCard(), withProgress(0, 0))
    expect(creditButton(container).textContent).toContain('+0 pt')
  })
})
