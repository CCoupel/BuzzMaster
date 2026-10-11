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

import { answeredColor } from './pointsAward'
describe('answeredColor (#255, ANSWERED_COLOR + repli)', () => {
  it('ANSWERED_COLOR présent : fait foi, même si TIME > 0 et ANSWER_COLOR non vide', () => {
    expect(answeredColor({ ANSWERED_COLOR: '', ANSWER_COLOR: 'RED', TIME: 5 })).toBe('')
    expect(answeredColor({ ANSWERED_COLOR: 'BLUE', ANSWER_COLOR: 'RED', TIME: 5 })).toBe('BLUE')
  })
  it('clé absente : repli TIME > 0 ? ANSWER_COLOR : vide', () => {
    expect(answeredColor({ ANSWER_COLOR: 'RED', TIME: 5 })).toBe('RED')
    expect(answeredColor({ ANSWER_COLOR: 'RED', TIME: 0 })).toBe('')
    expect(answeredColor(null)).toBe('')
  })
  it('hasAnswered suit ANSWERED_COLOR', () => {
    expect(hasAnswered({ TEAM: 'A', ANSWERED_COLOR: 'RED', ANSWER_COLOR: 'RED', TIME: 5 })).toBe(true)
    expect(hasAnswered({ TEAM: 'A', ANSWERED_COLOR: '', ANSWER_COLOR: 'RED', TIME: 5 })).toBe(false)
  })
})
