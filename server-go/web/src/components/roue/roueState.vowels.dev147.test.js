// #147 (v12.1.0) — portes d'achat de voyelles (lecture d'affichage du contrat §15.1) ; le serveur reste seul juge.
import { describe, it, expect } from 'vitest'
import { roueGates, vowelInfo, roueAnnouncement, makeRoueActions } from './roueState'

const gs = (o = {}) => ({
  ROUE_SUBPHASE: 'TURN', ROUE_CURRENT_TEAM: 'Rouges', ROUE_TEAM_BANK: { Rouges: 900 }, ROUE_CONSONANTS_LEFT: true,
  ROUE_BUY_VOWELS: true, ROUE_VOWEL_TIMING: 'BEFORE', ROUE_VOWELS_BEFORE: 'MANY', ROUE_VOWELS_AFTER: 'MANY',
  ROUE_VOWEL_PRICE: 250, ROUE_VOWEL_GAIN: 'NONE', ROUE_VOWELS_LEFT: true, ROUE_TURN_VOWEL: false, ROUE_STOP_VOWEL: false, ...o,
})

describe('vowelInfo / roueGates — achat de voyelles', () => {
  it('GameState v12.0 (sans ROUE_BUY_VOWELS) : aucun achat', () => {
    const v = vowelInfo({ ROUE_SUBPHASE: 'TURN' })
    expect(v.enabled).toBe(false)
    expect(v.canBuyBefore || v.canBuyAfter).toBe(false)
  })
  it('BEFORE en TURN : achat possible ; ONE + ROUE_TURN_VOWEL : limite atteinte', () => {
    expect(vowelInfo(gs()).canBuyBefore).toBe(true)
    const v = vowelInfo(gs({ ROUE_VOWELS_BEFORE: 'ONE', ROUE_TURN_VOWEL: true }))
    expect(v.canBuyBefore).toBe(false)
    expect(v.limitBefore).toBe(true)
  })
  it('cagnotte < prix ou plus de voyelle : achat grisé', () => {
    expect(vowelInfo(gs({ ROUE_TEAM_BANK: { Rouges: 100 } })).canBuyBefore).toBe(false)
    expect(vowelInfo(gs({ ROUE_VOWELS_LEFT: false })).canBuyBefore).toBe(false)
  })
  it('AFTER : LETTER achetable ; ANSWER seulement avec ROUE_STOP_VOWEL et « plusieurs »', () => {
    expect(vowelInfo(gs({ ROUE_VOWEL_TIMING: 'AFTER', ROUE_SUBPHASE: 'LETTER' })).canBuyAfter).toBe(true)
    expect(vowelInfo(gs({ ROUE_VOWEL_TIMING: 'AFTER', ROUE_SUBPHASE: 'ANSWER' })).showAfter).toBe(false)
    expect(vowelInfo(gs({ ROUE_VOWEL_TIMING: 'BOTH', ROUE_SUBPHASE: 'ANSWER', ROUE_STOP_VOWEL: true })).canBuyAfter).toBe(true)
    const one = vowelInfo(gs({ ROUE_VOWEL_TIMING: 'AFTER', ROUE_VOWELS_AFTER: 'ONE', ROUE_SUBPHASE: 'ANSWER', ROUE_STOP_VOWEL: true }))
    expect(one.canBuyAfter).toBe(false)
    expect(one.limitAfter).toBe(true)
  })
  it('« Réponse validée » ouverte en TURN après une voyelle avant de tourner ; FLIP ouvert en VOWEL_LIT', () => {
    expect(roueGates(gs({ ROUE_TURN_VOWEL: true })).canAnswer).toBe(true)
    expect(roueGates(gs()).canAnswer).toBe(false)
    expect(roueGates(gs({ ROUE_SUBPHASE: 'VOWEL_LIT' })).canFlip).toBe(true)
  })
  it('sans consonne : la roue reste lançable seulement si un achat après l\'arrêt est possible', () => {
    expect(roueGates(gs({ ROUE_CONSONANTS_LEFT: false, ROUE_VOWEL_TIMING: 'AFTER' })).canSpin).toBe(true)
    expect(roueGates(gs({ ROUE_CONSONANTS_LEFT: false })).canSpin).toBe(false)
  })
  it('action ROUE_BUY_VOWEL et annonce VOWEL_LIT', () => {
    const sent = []
    makeRoueActions((a, p) => sent.push([a, p])).buyVowel('E')
    expect(sent).toEqual([['ROUE_BUY_VOWEL', { LETTER: 'E' }]])
    const txt = roueAnnouncement(gs({ ROUE_SUBPHASE: 'VOWEL_LIT', ROUE_LAST_EVENT: { KIND: 'VOWEL_HIT', LETTER: 'E', COUNT: 4 } })).map(p => p.text ?? p.strong).join('')
    expect(txt).toContain('E')
    expect(txt).toContain('4 cases')
  })
})

describe('annonce de retournement avec le FLIP réel du moteur (VALUE 0) — review LOT-1.2 M1', () => {
  const txt = (g) => roueAnnouncement(g).map(p => p.text ?? p.strong).join('')
  const seg = { SEGMENTS: [{ KIND: 'value', VALUE: 500 }], CURRENT: 0 }
  it('voyelle, gain NONE : pas de « × 0 = +0 »', () => {
    expect(txt(gs({ ROUE_SUBPHASE: 'VOWEL_REVEALING', ROUE_LAST_EVENT: { KIND: 'FLIP', LETTER: 'E', COUNT: 3, VALUE: 0, GAIN: 0 } }))).toBe('E : 3 cases')
  })
  it('voyelle avec gain : « E : 3 cases = +750 »', () => {
    expect(txt(gs({ ROUE_SUBPHASE: 'REVEALING', ROUE_LAST_EVENT: { KIND: 'FLIP', LETTER: 'E', COUNT: 3, VALUE: 0, GAIN: 750 } }))).toBe('E : 3 cases = +750')
  })
  it('consonne (FLIP VALUE 0, GAIN 0) : valeur de la case et gain du tour', () => {
    expect(txt(gs({ ROUE_SUBPHASE: 'REVEALING', ROUE_WHEEL: seg, ROUE_TURN_GAIN: 1500, ROUE_LAST_EVENT: { KIND: 'FLIP', LETTER: 'R', COUNT: 3, VALUE: 0, GAIN: 0 } }))).toBe('R : 3 × 500 = +1 500')
  })
  it('TURN sans consonne, achat après possible : annonce « ou achetez une voyelle »', () => {
    expect(txt(gs({ ROUE_CONSONANTS_LEFT: false, ROUE_VOWEL_TIMING: 'AFTER' }))).toBe('Plus de consonne : proposez la phrase ou achetez une voyelle')
  })
})
