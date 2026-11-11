import { describe, it, expect } from 'vitest'
import {
  draftFromConfig, draftToPayload, addHost, addGroup, removeHost, moveHost, moveLight, locateLight,
  validateDraft, groupLabelForLight, MAX_HOST_GROUPS, testStepAt, TEST_STEPS,
} from './lightGroups'

const CFG = [
  { id: 'g1', name: 'Scène', role: 'salle', lights: ['A'], near_teams: ['Rouges'] },
  { id: 'h1', name: 'Pupitre', role: 'animateur', lights: ['B'], near_teams: [] },
  { id: 'h2', name: 'Régie', role: 'animateur', lights: ['C'], near_teams: [] },
]

describe('lightGroups — groupes animateur multiples et id stable (#225)', () => {
  it('lit plusieurs groupes animateur et conserve les id au renvoi', () => {
    const d = draftFromConfig(CFG)
    expect(d.hosts.map(h => h.id)).toEqual(['h1', 'h2'])
    expect(draftToPayload(d)).toEqual(CFG)
  })
  it('un groupe créé n\'a pas d\'id (le serveur l\'attribue) ; renommer ou déplacer garde l\'id', () => {
    let d = addGroup(draftFromConfig(CFG))
    expect(draftToPayload(d)[1]).not.toHaveProperty('id')
    d = moveHost(d, 0, 1)
    expect(d.hosts.map(h => h.id)).toEqual(['h2', 'h1'])
  })
  it('4 groupes animateur au plus', () => {
    let d = draftFromConfig([])
    for (let i = 0; i < 6; i += 1) d = addHost(d)
    expect(d.hosts).toHaveLength(MAX_HOST_GROUPS)
    expect(d.hosts.map(h => h.name)).toEqual(['Animateur 1', 'Animateur 2', 'Animateur 3', 'Animateur 4'])
    expect(removeHost(d, 0).hosts).toHaveLength(3)
  })
  it('une ampoule passe d\'un groupe animateur à un autre ; pastille « A<rang> »', () => {
    const d = moveLight(draftFromConfig(CFG), 'B', { kind: 'host', index: 1 })
    expect(locateLight(d, 'B')).toEqual({ kind: 'host', index: 1 })
    expect(groupLabelForLight(CFG, 'C')).toEqual({ kind: 'host', rank: 2, name: 'Régie' })
  })
  it('noms uniques entre groupes de salle et animateur', () => {
    const d = draftFromConfig([{ name: 'X', role: 'salle' }, { name: 'x', role: 'animateur' }])
    expect(validateDraft(d).byGroup).toEqual({ h0: 'Ce nom est déjà utilisé par un autre groupe.' })
  })
  it('pas de l\'essai : une seconde chacun, 6 pas', () => {
    expect(TEST_STEPS).toHaveLength(6)
    expect([0, 999, 1000, 5999, 9000].map(ms => testStepAt(ms))).toEqual([0, 0, 1, 5, 5])
  })
})
