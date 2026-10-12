import { describe, it, expect } from 'vitest'
import * as re from './roundsEdit'

// #245 — l'éditeur produit à chaque geste la structure COMPLÈTE (contrat ROUNDS_UPDATE : le serveur refuse INCOMPLETE si une
// question existante n'est pas citée). Scénarios 14 et 31 (fonctions pures). `critical` : complétude après chaque geste.
const S = () => ({ rounds: [{ id: 'r1', name: 'Un', questions: ['1', '2'] }, { id: 'r2', name: 'Deux', questions: ['3'] }], unassigned: ['4', '5'], deletedRounds: [] })
const all = (s) => [...s.unassigned, ...s.rounds.flatMap((r) => r.questions)].sort()

describe('roundsEdit — chaque geste conserve toutes les questions (contrat INCOMPLETE)', () => {
  it('Critical — moveQuestion : même ensemble, nouvelle place (autre manche, Sans manche, avant une question)', () => {
    const base = all(S())
    let s = re.moveQuestion(S(), '1', 'r2')
    expect(s.rounds[1].questions).toEqual(['3', '1'])
    s = re.moveQuestion(S(), '3', re.UNASSIGNED_KEY, '4')
    expect(s.unassigned).toEqual(['3', '4', '5'])
    s = re.moveQuestion(S(), '5', 'r1', '2')
    expect(s.rounds[0].questions).toEqual(['1', '5', '2'])
    for (const x of [re.moveQuestion(S(), '1', 'r2'), re.moveQuestion(S(), '3', re.UNASSIGNED_KEY), re.moveQuestion(S(), '5', 'r1', '2')]) expect(all(x)).toEqual(base)
  })

  it('moveRound : réordonne les manches, rien ne se perd ; indices hors bornes = inchangé', () => {
    const s = re.moveRound(S(), 0, 1)
    expect(s.rounds.map((r) => r.id)).toEqual(['r2', 'r1'])
    expect(all(s)).toEqual(all(S()))
    expect(re.moveRound(S(), 0, 5)).toEqual(S())
  })

  it('addRound : nouvelle manche SANS id, entractes vrai par défaut ; nom validé (1 à 60, espaces de bord retirés)', () => {
    const s = re.addRound(S(), '  Finale  ')
    const r = s.rounds[2]
    expect(r.id).toBeUndefined()
    expect(r.name).toBe('Finale')
    expect(r.entractes).toBe(true)
    expect(re.addRound(S(), 'X', false).rounds[2].entractes).toBe(false)
    expect(re.addRound(S(), '   ')).toEqual(S())
    expect(re.addRound(S(), 'x'.repeat(61))).toEqual(S())
    expect(re.addRound(S(), 'x'.repeat(60)).rounds).toHaveLength(3)
  })

  it('renameRound : renomme, refuse un nom vide, ne touche pas aux questions', () => {
    expect(re.renameRound(S(), 'r1', ' Nouveau ').rounds[0].name).toBe('Nouveau')
    expect(re.renameRound(S(), 'r1', '')).toEqual(S())
    expect(re.renameRound(S(), 'r1', 'Nouveau').rounds[0].questions).toEqual(['1', '2'])
  })

  it('Critical — deleteRound KEEP : ses questions vont à la FIN de « Sans manche », dans l’ordre ; DELETED_ROUNDS{KEEP}', () => {
    const s = re.deleteRound(S(), 'r1', 'KEEP')
    expect(s.rounds.map((r) => r.id)).toEqual(['r2'])
    expect(s.unassigned).toEqual(['4', '5', '1', '2'])
    expect(s.deletedRounds).toEqual([{ id: 'r1', questions: 'KEEP' }])
  })

  it('Critical — deleteRound DELETE : ses questions ne sont plus citées (le serveur les retrouve par ROUND_ID) ; DELETED_ROUNDS{DELETE}', () => {
    const s = re.deleteRound(S(), 'r1', 'DELETE')
    expect(all(s)).toEqual(['3', '4', '5'])
    expect(s.deletedRounds).toEqual([{ id: 'r1', questions: 'DELETE' }])
    expect(re.deletedQuestionIds(S(), s).sort()).toEqual(['1', '2'])
  })

  it('toRoundsUpdatePayload : id et entractes seulement pour ce qui s’applique (ENTRACTES = nouvelle manche seulement)', () => {
    const s = re.addRound(S(), 'Finale', false)
    const p = re.toRoundsUpdatePayload(s)
    expect(p.rounds[0]).toEqual({ id: 'r1', name: 'Un', questions: ['1', '2'] })
    expect(p.rounds[2]).toEqual({ name: 'Finale', questions: [], entractes: false })
    expect(p.unassigned).toEqual(['4', '5'])
    expect(p.deletedRounds).toEqual([])
  })

  it('structureFrom : reconstruit la structure à partir des questions et des manches du serveur (ROUND_ID inconnu = Sans manche)', () => {
    const qs = { 1: { ID: '1', ORDER: 0, ROUND_ID: 'r1' }, 2: { ID: '2', ORDER: 1 }, 3: { ID: '3', ORDER: 0, ROUND_ID: 'r9' } }
    const st = re.structureFrom(qs, [{ ID: 'r1', NAME: 'Un' }])
    expect(st.rounds).toEqual([{ id: 'r1', name: 'Un', questions: ['1'] }])
    expect(st.unassigned).toEqual(['3', '2'])
  })
})
