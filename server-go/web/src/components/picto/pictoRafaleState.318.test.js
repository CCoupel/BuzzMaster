import { describe, it, expect } from 'vitest'
import { advanceThread, closeThread, canPictoPass, canRafaleFound, foundLabel, passUsedLabel, passesLeftLabel, passLimitLabel, roundPointsFormula, PICTO_FOUND_FLASH_MS } from './pictoRafaleState'

// #318 (v13.1.0, LOT-3) — helpers purs de la manche Rafale Picto (picto.md §17).
describe('constantes et libellés', () => {
  it('le mot trouvé reste 1 s sur le dessin de la TV', () => expect(PICTO_FOUND_FLASH_MS).toBe(1000))
  it('foundLabel / passUsedLabel : accord', () => {
    expect([foundLabel(0), foundLabel(1), foundLabel(3)]).toEqual(['0 mot trouvé', '1 mot trouvé', '3 mots trouvés'])
    expect([passUsedLabel(0), passUsedLabel(1), passUsedLabel(2)]).toEqual(['0 passe', '1 passe', '2 passes'])
  })
  it('passesLeftLabel : −1 = illimitées', () => {
    expect(passesLeftLabel({ PICTO_PASSES_LEFT: -1 })).toBe('illimitées')
    expect(passesLeftLabel({})).toBe('illimitées')
    expect(passesLeftLabel({ PICTO_PASSES_LEFT: 1 })).toBe('1 restante')
    expect(passesLeftLabel({ PICTO_PASSES_LEFT: 0 })).toBe('0 restante')
  })
  it('passLimitLabel : 1 passe / illimitées', () => {
    expect(passLimitLabel({ RAFALE_PICTO_PASS_LIMIT: 1 })).toBe('1 passe')
    expect(passLimitLabel({ RAFALE_PICTO_PASS_LIMIT: 0 })).toBe('illimitées')
    expect(passLimitLabel({})).toBe('illimitées')
  })
  it('roundPointsFormula : trouvés × points − passes × coût', () => {
    expect(roundPointsFormula({ PICTO_FOUND_COUNT: 3, PICTO_PASS_COUNT: 1 }, { POINTS: 10, RAFALE_PICTO_PASS_COST: 5 })).toBe('3 × 10 − 1 × 5')
    expect(roundPointsFormula({ PICTO_FOUND_COUNT: 2, PICTO_PASS_COUNT: 0 }, { POINTS: 10 })).toBe('2 × 10')
  })
})

describe('boutons proposés', () => {
  it('Passer : STARTED seulement, grisé quand il ne reste aucune passe (0), jamais en pause ni arrêté', () => {
    expect(canPictoPass({ phase: 'STARTED', PICTO_PASSES_LEFT: -1 })).toBe(true)
    expect(canPictoPass({ phase: 'STARTED', PICTO_PASSES_LEFT: 1 })).toBe(true)
    expect(canPictoPass({ phase: 'STARTED', PICTO_PASSES_LEFT: 0 })).toBe(false)
    expect(canPictoPass({ phase: 'PAUSED', PICTO_PASSES_LEFT: -1 })).toBe(false)
    expect(canPictoPass({ phase: 'STOPPED', PICTO_PASSES_LEFT: -1 })).toBe(false)
  })
  it('Trouvé : STARTED, PAUSED, et STOPPED après temps écoulé (gong)', () => {
    expect(canRafaleFound({ phase: 'STARTED' })).toBe(true)
    expect(canRafaleFound({ phase: 'PAUSED' })).toBe(true)
    expect(canRafaleFound({ phase: 'STOPPED', PICTO_OUTCOME: 'TIMEOUT' })).toBe(true)
    expect(canRafaleFound({ phase: 'STOPPED', PICTO_OUTCOME: 'STOPPED' })).toBe(false)
    expect(canRafaleFound({ phase: 'STOPPED', PICTO_OUTCOME: 'POOL_END' })).toBe(false)
    expect(canRafaleFound({ phase: 'REVEALED' })).toBe(false)
  })
})

describe('fil de la manche (advanceThread / closeThread)', () => {
  const ev = (word, epoch, found, passes) => ({ word, epoch, found, passes })
  it('le premier mot est « en cours »', () => {
    const r = advanceThread([], ev('Chat', 1, 0, 0))
    expect(r.thread).toEqual([{ word: 'Chat', epoch: 1, status: 'current' }])
  })
  it('compteur de trouvés monté → mot précédent ✅ ; passes montées → ⏭', () => {
    let r = advanceThread([], ev('Chat', 1, 0, 0))
    r = advanceThread(r.thread, ev('Soleil', 2, 1, 0), r.seen)
    r = advanceThread(r.thread, ev('Girafe', 3, 1, 0), { found: 1, passes: 0 }) // inchangé : un mot n'arrive que sur trouvé/passe
    expect(r.thread.map(t => [t.word, t.status])).toEqual([['Chat', 'found'], ['Soleil', 'passed'], ['Girafe', 'current']])
  })
  it('même époque : pas de doublon (re-rendu, reconnexion)', () => {
    let r = advanceThread([], ev('Chat', 1, 0, 0))
    r = advanceThread(r.thread, ev('Chat', 1, 0, 0), r.seen)
    expect(r.thread).toHaveLength(1)
  })
  it('un mot passé qui revient (rebouclage) apparaît comme une nouvelle ligne', () => {
    let r = advanceThread([], ev('Girafe', 1, 0, 0))
    r = advanceThread(r.thread, ev('Chat', 2, 0, 1), r.seen)
    r = advanceThread(r.thread, ev('Girafe', 3, 1, 1), r.seen)
    expect(r.thread.map(t => [t.word, t.status])).toEqual([['Girafe', 'passed'], ['Chat', 'found'], ['Girafe', 'current']])
  })
  it('aucun mot à venir : le fil ne contient que des mots déjà montrés', () => {
    const r = advanceThread([], ev('Chat', 1, 0, 0))
    expect(r.thread.every(t => ['found', 'passed', 'current', 'revealed'].includes(t.status))).toBe(true)
  })
  it('fin de manche : le mot en cours devient ⌛ ; trouvé sur le gong → ✅', () => {
    const thread = [{ word: 'Chat', epoch: 1, status: 'found' }, { word: 'Maison', epoch: 2, status: 'current' }]
    expect(closeThread(thread, { found: 1 }, { found: 1, passes: 0 })[1].status).toBe('revealed')
    expect(closeThread(thread, { found: 2 }, { found: 1, passes: 0 })[1].status).toBe('found')
  })
  it('fin de pool (aucun mot en cours) : rien à clore', () => {
    const thread = [{ word: 'Chat', epoch: 1, status: 'found' }]
    expect(closeThread(thread, { found: 1 }, { found: 1, passes: 0 })).toEqual(thread)
  })
})
