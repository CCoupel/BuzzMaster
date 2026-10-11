import { describe, it, expect } from 'vitest'
import { groupQuestionsByRound, sortQuestionsByRounds, sortQuestionsByOrder, countEntries } from './questionOrder'
import { structureFrom, moveQuestion, moveRound, addRound, renameRound, deleteRound, toRoundsUpdatePayload, cleanRoundName } from './roundsEdit'

// #245 — ordre à deux niveaux (parité avec getNextQuestionPayload) et gestes purs de l'éditeur.
const q = (id, over = {}) => [String(id), { ID: String(id), TYPE: 'QCM', ORDER: id, ...over }]
const QS = Object.fromEntries([q(1, { ROUND_ID: 'r1', ORDER: 1 }), q(2, { ROUND_ID: 'r1', ORDER: 0 }), q(3, { ROUND_ID: 'r2' }), q(4), q(5, { ROUND_ID: 'ghost' }), q(6, { TYPE: 'ENTRACTE', ROUND_ID: 'r2', ORDER: 0 })])
const ROUNDS = [{ ID: 'r1', NAME: 'A' }, { ID: 'r2', NAME: 'B' }]

describe('ordre à deux niveaux (#245)', () => {
  it('Critical — « Sans manche » (dont ROUND_ID inconnu) en premier, puis les manches dans l’ordre, puis ORDER puis ID', () => {
    expect(sortQuestionsByRounds(QS, ROUNDS).map(x => x.ID)).toEqual(['4', '5', '2', '1', '6', '3'])
  })
  it('sans manche : identique à l’ordre v11.1', () => {
    const plain = Object.fromEntries([q(3, { ORDER: 0 }), q(1, { ORDER: 2 }), q(2, { ORDER: 1 })])
    expect(sortQuestionsByRounds(plain, []).map(x => x.ID)).toEqual(sortQuestionsByOrder(plain).map(x => x.ID))
  })
  it('départage par ID numérique à ORDER égal (10 après 9)', () => {
    const m = Object.fromEntries([q(10, { ORDER: 0 }), q(9, { ORDER: 0 })])
    expect(sortQuestionsByRounds(m, []).map(x => x.ID)).toEqual(['9', '10'])
  })
  it('groupQuestionsByRound et countEntries', () => {
    const g = groupQuestionsByRound(QS, ROUNDS)
    expect(g.unassigned.map(x => x.ID)).toEqual(['4', '5'])
    expect(countEntries(g.rounds[1].questions)).toEqual({ questions: 1, entractes: 1 })
  })
})

describe('gestes purs (#245)', () => {
  const S = structureFrom(QS, ROUNDS)
  it('structureFrom', () => {
    expect(S).toEqual({ rounds: [{ id: 'r1', name: 'A', questions: ['2', '1'] }, { id: 'r2', name: 'B', questions: ['6', '3'] }], unassigned: ['4', '5'], deletedRounds: [] })
  })
  it('moveQuestion : avant une cible, ou à la fin ; ne modifie pas l’original', () => {
    expect(moveQuestion(S, '4', 'r1', '1').rounds[0].questions).toEqual(['2', '4', '1'])
    expect(moveQuestion(S, '1', 'un').unassigned).toEqual(['4', '5', '1'])
    expect(S.unassigned).toEqual(['4', '5'])
  })
  it('moveRound', () => {
    expect(moveRound(S, 0, 1).rounds.map(r => r.id)).toEqual(['r2', 'r1'])
    expect(moveRound(S, 0, 0)).toBe(S)
  })
  it('noms : 1 à 60 caractères après trim', () => {
    expect(cleanRoundName('  x ')).toBe('x')
    expect(cleanRoundName('   ')).toBeNull()
    expect(cleanRoundName('a'.repeat(61))).toBeNull()
    expect(addRound(S, '')).toBe(S)
    expect(renameRound(S, 'r1', ' ')).toBe(S)
  })
  it('addRound : ENTRACTES par défaut vrai, absent pour une manche existante', () => {
    const p = toRoundsUpdatePayload(addRound(S, 'C'))
    expect(p.rounds[2]).toEqual({ name: 'C', questions: [], entractes: true })
    expect(p.rounds[0]).toEqual({ id: 'r1', name: 'A', questions: ['2', '1'] })
    expect(toRoundsUpdatePayload(addRound(S, 'C', false)).rounds[2].entractes).toBe(false)
  })
  it('deleteRound KEEP / DELETE', () => {
    const k = deleteRound(S, 'r1', 'KEEP')
    expect(k.unassigned).toEqual(['4', '5', '2', '1'])
    expect(k.deletedRounds).toEqual([{ id: 'r1', questions: 'KEEP' }])
    const d = deleteRound(S, 'r1', 'DELETE')
    expect(d.unassigned).toEqual(['4', '5'])
    expect(d.deletedRounds).toEqual([{ id: 'r1', questions: 'DELETE' }])
  })
})

describe('ORDER non entier tronqué (#245, parité Go)', () => {
  it('1.9 est traité comme 1 (puis départagé par l’ID), 1.5 comme 1', () => {
    const m = Object.fromEntries([q(10, { ORDER: 1.9 }), q(9, { ORDER: 1.5 }), q(8, { ORDER: 2 })])
    expect(sortQuestionsByRounds(m, []).map(x => x.ID)).toEqual(['9', '10', '8'])
  })
})
