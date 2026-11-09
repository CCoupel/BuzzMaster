// @vitest-environment node
import { describe, it, expect } from 'vitest'
import * as re from './roundsEdit'

// #290 (v11.6.0, LOT-4) — la surcharge d'ambiance d'une manche (`AMBIANCE`) est TRANSPORTÉE telle quelle par l'éditeur de
// manches : chaque geste (renommage, déplacement, réordonnancement, suppression d'une AUTRE manche) envoie un ROUNDS_UPDATE
// COMPLET ; l'AMBIANCE ne doit ni disparaître, ni migrer d'une manche à l'autre. Contrats : models.md §Manches (la
// surcharge « suit la manche (renommage, déplacement) et disparaît avec elle »), websocket-actions.md (AMBIANCE absent ⇒
// conservé ; {} explicite ⇒ effacé). Fonctions pures.
const AMB1 = { sound: { muted: true }, light: { mode: 'ON' } }
const rounds = () => [
  { ID: 'r1', NAME: 'Un', AMBIANCE: AMB1 },
  { ID: 'r2', NAME: 'Deux', AMBIANCE: {} },
  { ID: 'r3', NAME: 'Trois' }, // serveur antérieur : AMBIANCE absente
]
const qs = { 1: { ID: '1', ROUND_ID: 'r1', ORDER: 0 }, 2: { ID: '2', ROUND_ID: 'r2', ORDER: 1 }, 3: { ID: '3', ROUND_ID: 'r3', ORDER: 2 }, 4: { ID: '4', ORDER: 3 } }
const struct = () => re.structureFrom(qs, rounds())
const byId = (payload, id) => payload.rounds.find((r) => r.id === id)

describe('roundsEdit — AMBIANCE transportée (#290)', () => {
  it('structureFrom puis toRoundsUpdatePayload : la surcharge de chaque manche est transportée (AMBIANCE présente ⇒ envoyée)', () => {
    const p = re.toRoundsUpdatePayload(struct())
    expect(byId(p, 'r1').ambiance).toEqual(AMB1)
    expect(byId(p, 'r2').ambiance).toEqual({})
  })

  it('une manche sans AMBIANCE (serveur antérieur) n\'envoie AUCUNE clé ambiance ⇒ le serveur conserve l\'existant', () => {
    const p = re.toRoundsUpdatePayload(struct())
    expect('ambiance' in byId(p, 'r3')).toBe(false)
  })

  it('renommer une manche conserve sa surcharge (et pas celle des autres)', () => {
    const p = re.toRoundsUpdatePayload(re.renameRound(struct(), 'r1', 'Blind-test'))
    expect(byId(p, 'r1').name).toBe('Blind-test')
    expect(byId(p, 'r1').ambiance).toEqual(AMB1)
    expect(byId(p, 'r2').ambiance).toEqual({})
  })

  it('déplacer une question entre manches ne déplace aucune surcharge', () => {
    const p = re.toRoundsUpdatePayload(re.moveQuestion(struct(), '1', 'r2'))
    expect(byId(p, 'r1').ambiance).toEqual(AMB1)
    expect(byId(p, 'r2').ambiance).toEqual({})
  })

  it('réordonner les manches : la surcharge suit SA manche, pas sa position', () => {
    const s = re.moveRound(struct(), 0, 1)
    expect(s.rounds.map((r) => r.id)).toEqual(['r2', 'r1', 'r3'])
    const p = re.toRoundsUpdatePayload(s)
    expect(byId(p, 'r1').ambiance).toEqual(AMB1)
    expect(p.rounds[0].ambiance).toEqual({})
  })

  it('supprimer une AUTRE manche (KEEP) conserve la surcharge des manches restantes', () => {
    const p = re.toRoundsUpdatePayload(re.deleteRound(struct(), 'r2', 'KEEP'))
    expect(p.rounds.map((r) => r.id)).toEqual(['r1', 'r3'])
    expect(byId(p, 'r1').ambiance).toEqual(AMB1)
  })

  it('une manche NOUVELLE n\'envoie pas d\'AMBIANCE (le serveur la crée à {} — tout hérité)', () => {
    const p = re.toRoundsUpdatePayload(re.addRound(struct(), 'Finale'))
    const created = p.rounds.find((r) => !r.id)
    expect(created.name).toBe('Finale')
    expect('ambiance' in created).toBe(false)
  })
})
