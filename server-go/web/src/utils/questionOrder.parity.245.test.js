import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, resolve } from 'node:path'
import { sortQuestionsByRounds, groupQuestionsByRound, sortQuestionsByOrder, countEntries } from './questionOrder'

// #245 scénario 12 — PARITÉ JS / Go : fixture partagée server-go/tests/fixtures/rounds_order_245.json, lue aussi par le test Go
// (cmd/server/rounds_order_245_test.go). EXPECTED_ORDER y est écrit à la main, indépendamment des deux implémentations.
// Contrat : models.md §Manches (« Sans manche » en premier, puis les manches, ORDER puis ID numérique). `critical`.
const here = dirname(fileURLToPath(import.meta.url))
const fixture = JSON.parse(readFileSync(resolve(here, '../../../tests/fixtures/rounds_order_245.json'), 'utf8'))
const byId = Object.fromEntries(fixture.QUESTIONS.map((q) => [q.ID, q]))

describe('questionOrder — parité avec le Go (fixture partagée)', () => {
  it('Critical — sortQuestionsByRounds donne exactement EXPECTED_ORDER', () => {
    expect(sortQuestionsByRounds(byId, fixture.ROUNDS).map((q) => q.ID)).toEqual(fixture.EXPECTED_ORDER)
  })

  it('« Sans manche » en premier ; ROUND_ID inconnu = Sans manche ; ORDER absent = ID ; égalité d’ORDER tranchée par l’ID numérique', () => {
    const g = groupQuestionsByRound(byId, fixture.ROUNDS)
    expect(g.unassigned.map((q) => q.ID)).toEqual(['12', '40', '10', '11', '41'])
    expect(g.rounds.map((r) => [r.ID, r.questions.map((q) => q.ID)])).toEqual([['r1', ['31', '30', '32']], ['r2', ['21', '22', '20']]])
  })

  it('sans aucune manche : ordre identique à sortQuestionsByOrder (v11.1)', () => {
    const plain = Object.fromEntries(['5', '3', '9', '1'].map((id, i) => [id, { ID: id, ORDER: [4, 2, 0, 7][i] }]))
    expect(sortQuestionsByRounds(plain, []).map((q) => q.ID)).toEqual(sortQuestionsByOrder(plain).map((q) => q.ID))
    expect(sortQuestionsByRounds(plain, undefined).map((q) => q.ID)).toEqual(['9', '3', '5', '1'])
  })

  it('countEntries : les ENTRACTE ne comptent pas comme questions', () => {
    const g = groupQuestionsByRound(byId, fixture.ROUNDS)
    expect(countEntries(g.rounds[0].questions)).toEqual({ questions: 2, entractes: 1 })
  })
})
