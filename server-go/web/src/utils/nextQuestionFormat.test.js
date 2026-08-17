import { describe, it, expect } from 'vitest'
import { formatNextQuestionStatement, formatNextQuestionMeta } from './nextQuestionFormat'

// ---------------------------------------------------------------------------
// nextQuestionFormat — formatage partagé de la question suivante (#163/F2,
// #165), format imposé GATE 2 (D1) : "#<ID> <type>: <énoncé> <points>pt
// <délai>s" (_work/handoff/gate2-decisions-163-164.md, verbatim utilisateur).
// Consommé par AnimPage.jsx (puce "Suivante") ET AnimConductPanel.jsx
// (bouton "À suivre", #165) — testé une seule fois ici plutôt que dupliqué
// dans AnimPage.test.jsx/AnimConductPanel.test.jsx (qui, eux, vérifient
// uniquement le câblage : la sortie de ces fonctions est bien affichée aux
// bons endroits).
// ---------------------------------------------------------------------------

describe('formatNextQuestionStatement', () => {
  it('compose "#<ID> <type>: <énoncé>"', () => {
    expect(formatNextQuestionStatement({ ID: '2', TYPE: 'QCM', QUESTION: 'Qui a peint la nuit étoilée?' }))
      .toBe('#2 QCM: Qui a peint la nuit étoilée?')
  })

  it('replie sur SPEEDY quand TYPE est absent', () => {
    expect(formatNextQuestionStatement({ ID: '5', QUESTION: 'Q ?' })).toBe('#5 SPEEDY: Q ?')
  })

  it('énoncé vide quand QUESTION est absent (pas de "undefined" affiché)', () => {
    expect(formatNextQuestionStatement({ ID: '5', TYPE: 'SPEEDY' })).toBe('#5 SPEEDY: ')
  })

  it('chaîne vide quand il n\'y a pas de question suivante (ID absent)', () => {
    expect(formatNextQuestionStatement(null)).toBe('')
    expect(formatNextQuestionStatement(undefined)).toBe('')
    expect(formatNextQuestionStatement({})).toBe('')
  })
})

describe('formatNextQuestionMeta', () => {
  it('compose "<points>pt <délai>s"', () => {
    expect(formatNextQuestionMeta({ ID: '2', POINTS: '20', TIME: '30' })).toBe('20pt 30s')
  })

  it('replie sur 0 quand POINTS ou TIME sont absents', () => {
    expect(formatNextQuestionMeta({ ID: '2' })).toBe('0pt 0s')
    expect(formatNextQuestionMeta({ ID: '2', POINTS: '5' })).toBe('5pt 0s')
    expect(formatNextQuestionMeta({ ID: '2', TIME: '15' })).toBe('0pt 15s')
  })

  it('conserve POINTS explicitement à 0 (valeur légitime, pas un "absent")', () => {
    expect(formatNextQuestionMeta({ ID: '2', POINTS: '0', TIME: '10' })).toBe('0pt 10s')
  })

  it('chaîne vide quand il n\'y a pas de question suivante (ID absent)', () => {
    expect(formatNextQuestionMeta(null)).toBe('')
    expect(formatNextQuestionMeta(undefined)).toBe('')
    expect(formatNextQuestionMeta({})).toBe('')
  })
})
