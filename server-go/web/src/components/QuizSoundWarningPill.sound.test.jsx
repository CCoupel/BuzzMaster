import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render } from '@testing-library/react'
import QuizSoundWarningPill from './QuizSoundWarningPill'

// ---------------------------------------------------------------------------
// QuizSoundWarningPill — puce d'alerte globale (v11.1, addendum média
// indisponible, #219/#236/#237, plan _work/reports/plan-20260923-101500.md
// §6 tâche 13, contrat sound.md §10.8, maquette question-sound-219.html
// §07). État entièrement dérivé — aucune prop, aucun appel serveur direct :
// useGame()/useSoundStatus() mockés, patron Navbar.ambiance.test.jsx pour
// useSoundStatus.
//
// CA28 : visible SEULEMENT si le quiz a des sons ET l'audio est inactif ;
// invisible dans les 3 autres combinaisons ; disparaît sur bascule audio ET
// sur retrait du dernier son.
// ---------------------------------------------------------------------------

vi.mock('./QuizSoundWarningPill.css', () => ({}))

const gameMock = { questions: {} }
vi.mock('../hooks/GameContext', () => ({
  useGame: () => gameMock,
}))

const soundMock = { status: { active: false }, refresh: vi.fn() }
vi.mock('../hooks/useSoundStatus', () => ({
  useSoundStatus: () => soundMock,
}))

function withSound(id, sound = '/question/1/sound_1.wav') {
  return { ID: id, SOUND: sound }
}
function withoutSound(id) {
  return { ID: id, SOUND: '' }
}

beforeEach(() => {
  gameMock.questions = {}
  soundMock.status = { active: false }
})

function getPill(container) {
  return container.querySelector('.quiz-sound-warning-pill')
}

// ---------------------------------------------------------------------------
// CA28 — la table de vérité des 4 combinaisons (maquette §07).
// ---------------------------------------------------------------------------

describe('QuizSoundWarningPill — table de vérité (CA28)', () => {
  it('quiz avec sons, audio actif → masquée', () => {
    gameMock.questions = { q1: withSound('q1') }
    soundMock.status = { active: true }
    const { container } = render(<QuizSoundWarningPill />)
    expect(getPill(container)).toBeNull()
  })

  it('quiz avec sons, audio inactif → affichée', () => {
    gameMock.questions = { q1: withSound('q1') }
    soundMock.status = { active: false }
    const { container } = render(<QuizSoundWarningPill />)
    expect(getPill(container)).not.toBeNull()
  })

  it('quiz sans aucun son, audio inactif → masquée', () => {
    gameMock.questions = { q1: withoutSound('q1'), q2: withoutSound('q2') }
    soundMock.status = { active: false }
    const { container } = render(<QuizSoundWarningPill />)
    expect(getPill(container)).toBeNull()
  })

  it('quiz sans aucun son, audio actif → masquée', () => {
    gameMock.questions = { q1: withoutSound('q1') }
    soundMock.status = { active: true }
    const { container } = render(<QuizSoundWarningPill />)
    expect(getPill(container)).toBeNull()
  })

  it('aucune question du tout (quiz vide) → masquée, ne lève pas', () => {
    gameMock.questions = {}
    soundMock.status = { active: false }
    const { container } = render(<QuizSoundWarningPill />)
    expect(getPill(container)).toBeNull()
  })

  it('questions = null/undefined (avant chargement) → masquée, ne lève pas', () => {
    gameMock.questions = null
    soundMock.status = { active: false }
    const { container } = render(<QuizSoundWarningPill />)
    expect(getPill(container)).toBeNull()
  })
})

// ---------------------------------------------------------------------------
// Contenu — le nombre de questions et le remède nommé (maquette §07).
// ---------------------------------------------------------------------------

describe('QuizSoundWarningPill — contenu du message', () => {
  it('une seule question sonore : singulier, "1 question sonore"', () => {
    gameMock.questions = { q1: withSound('q1') }
    soundMock.status = { active: false }
    const { container } = render(<QuizSoundWarningPill />)
    expect(container.textContent).toContain('1 question sonore')
    expect(container.textContent).not.toContain('questions sonores')
  })

  it('plusieurs questions sonores : pluriel, nombre exact', () => {
    gameMock.questions = { q1: withSound('q1'), q2: withSound('q2'), q3: withSound('q3') }
    soundMock.status = { active: false }
    const { container } = render(<QuizSoundWarningPill />)
    expect(container.textContent).toContain('3 questions sonores')
  })

  it('ne compte que les questions AVEC son (SOUND non vide), pas le total du quiz', () => {
    gameMock.questions = { q1: withSound('q1'), q2: withoutSound('q2'), q3: withoutSound('q3') }
    soundMock.status = { active: false }
    const { container } = render(<QuizSoundWarningPill />)
    expect(container.textContent).toContain('1 question sonore')
  })

  it('nomme le remède réel : redémarrer le serveur', () => {
    gameMock.questions = { q1: withSound('q1') }
    soundMock.status = { active: false }
    const { container } = render(<QuizSoundWarningPill />)
    expect(container.textContent).toMatch(/redémarrer le serveur/i)
  })
})

// ---------------------------------------------------------------------------
// Disparition automatique (CA28) — état dérivé, pas stocké : un second
// rendu avec des props/mocks différents doit refléter le nouvel état
// immédiatement, sans geste d'acquittement.
// ---------------------------------------------------------------------------

describe('QuizSoundWarningPill — disparition automatique (CA28)', () => {
  it('audio réactivé entre deux rendus → la puce disparaît', () => {
    gameMock.questions = { q1: withSound('q1') }
    soundMock.status = { active: false }
    const { container, rerender } = render(<QuizSoundWarningPill />)
    expect(getPill(container)).not.toBeNull()

    soundMock.status = { active: true }
    rerender(<QuizSoundWarningPill />)
    expect(getPill(container)).toBeNull()
  })

  it('dernier son retiré du quiz entre deux rendus (audio toujours inactif) → la puce disparaît', () => {
    gameMock.questions = { q1: withSound('q1') }
    soundMock.status = { active: false }
    const { container, rerender } = render(<QuizSoundWarningPill />)
    expect(getPill(container)).not.toBeNull()

    gameMock.questions = { q1: withoutSound('q1') }
    rerender(<QuizSoundWarningPill />)
    expect(getPill(container)).toBeNull()
  })
})

// ---------------------------------------------------------------------------
// Composant partagé, sans props — même instance montée sur deux pages
// (patron RafalePoolAlert).
// ---------------------------------------------------------------------------

describe('QuizSoundWarningPill — composant sans props, réutilisable tel quel', () => {
  it('accepte className optionnel sans casser le comportement dérivé', () => {
    gameMock.questions = { q1: withSound('q1') }
    soundMock.status = { active: false }
    const { container } = render(<QuizSoundWarningPill className="extra-class" />)
    const pill = getPill(container)
    expect(pill).not.toBeNull()
    expect(pill.className).toContain('extra-class')
  })
})
