import { describe, it, expect } from 'vitest'
import { sortArdoiseEntries } from './ardoiseOrder'

// #249 — identité = clé : la clé de la map `teams` (portée par `name`) prime sur `NAME`, qui peut être
// périmé dans un ancien état (reproduction harness246_rename_gamepage : « — » à la place de la réponse).
describe('sortArdoiseEntries — identité = clé (#249)', () => {
  it('retrouve la réponse sous la clé même si NAME porte l’ancien nom', () => {
    const teams = [{ name: 'Les Bleus', NAME: 'Equipe 1' }, { name: 'Equipe 2', NAME: 'Equipe 2' }]
    const out = sortArdoiseEntries(teams, { 'Les Bleus': { TEXT: 'NAPOLEON', STARTED_AT: 3, SUBMITTED_AT: 4 } })
    expect(out[0].teamName).toBe('Les Bleus')
    expect(out[0].answer.TEXT).toBe('NAPOLEON')
    expect(out[1].answer).toBeNull()
  })

  it('une réponse enregistrée sous l’ancien NAME n’est PAS attribuée à l’équipe renommée', () => {
    const teams = [{ name: 'Les Bleus', NAME: 'Equipe 1' }]
    const out = sortArdoiseEntries(teams, { 'Equipe 1': { TEXT: 'ANCIEN', STARTED_AT: 1, SUBMITTED_AT: 1 } })
    expect(out[0].answer).toBeNull()
  })

  it('sans `name` (entrée ancienne), repli sur NAME', () => {
    const out = sortArdoiseEntries([{ NAME: 'Les Rouges' }], { 'Les Rouges': { TEXT: 'X', STARTED_AT: 1, SUBMITTED_AT: 1 } })
    expect(out[0].answer.TEXT).toBe('X')
  })
})
