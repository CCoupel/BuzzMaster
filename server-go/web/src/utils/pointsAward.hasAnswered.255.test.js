import { describe, it, expect } from 'vitest'
import { hasAnswered } from './pointsAward'

// #255 — réponse effective = équipe + couleur + buzz enregistré (TIME > 0).
describe('hasAnswered (#255)', () => {
  it('vrai : équipe, couleur et TIME > 0', () => {
    expect(hasAnswered({ TEAM: 'A', ANSWER_COLOR: 'RED', TIME: 5 })).toBe(true)
  })
  it.each([
    [{ TEAM: 'A', ANSWER_COLOR: 'RED', TIME: 0 }],
    [{ TEAM: 'A', ANSWER_COLOR: 'RED' }],
    [{ TEAM: 'A', ANSWER_COLOR: '', TIME: 5 }],
    [{ TEAM: '', ANSWER_COLOR: 'RED', TIME: 5 }],
    [null],
    [undefined],
  ])('faux : %j', (b) => { expect(hasAnswered(b)).toBe(false) })
})
