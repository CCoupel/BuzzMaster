import { describe, it, expect } from 'vitest'
import {
  draftFromConfig, draftToPayload, addHost, addGroup, removeHost, moveHost, moveGroup, moveLight, locateLight,
  validateDraft, groupLabelForLight, orderChanged, groupName, hostName, MAX_HOST_GROUPS, testStepAt, TEST_STEPS,
} from './lightGroups'

const CFG = [
  { id: 'g1', role: 'salle', lights: ['A'] },
  { id: 'g2', role: 'salle', lights: [] },
  { id: 'h1', role: 'animateur', lights: ['B'] },
  { id: 'h2', role: 'animateur', lights: ['C'] },
]

describe('lightGroups — groupes numérotés sans nom, animateur multiples, id stable (#225, révision 3)', () => {
  it('lit les groupes, ignore name / near_teams d\'un config.json antérieur, conserve les id au renvoi', () => {
    const old = CFG.map(g => ({ ...g, name: 'Ancien', near_teams: ['Rouges'] }))
    const d = draftFromConfig(old)
    expect(d.hosts.map(h => h.id)).toEqual(['h1', 'h2'])
    expect(draftToPayload(d)).toEqual(CFG)
  })
  it('un groupe créé n\'a pas d\'id (le serveur l\'attribue) ; déplacer un groupe garde son id', () => {
    let d = addGroup(draftFromConfig(CFG))
    expect(draftToPayload(d)[2]).toEqual({ role: 'salle', lights: [] })
    d = moveHost(moveGroup(d, 0, 1), 0, 1)
    expect(d.groups.map(g => g.id)).toEqual(['g2', 'g1', ''])
    expect(d.hosts.map(h => h.id)).toEqual(['h2', 'h1'])
  })
  it('4 groupes animateur au plus', () => {
    let d = draftFromConfig([])
    for (let i = 0; i < 6; i += 1) d = addHost(d)
    expect(d.hosts).toHaveLength(MAX_HOST_GROUPS)
    expect(removeHost(d, 0).hosts).toHaveLength(3)
    expect(validateDraft(d).global).toEqual([])
  })
  it('libellés d\'après le rang ; pastille par ampoule', () => {
    expect(groupName(2)).toBe('Groupe 3')
    expect(hostName(1)).toBe('Groupe animateur 2')
    expect(groupLabelForLight(CFG, 'A')).toEqual({ kind: 'salle', n: 1 })
    expect(groupLabelForLight(CFG, 'C')).toEqual({ kind: 'host', rank: 2 })
    expect(groupLabelForLight(CFG, 'Z')).toBeNull()
  })
  it('une ampoule passe d\'un groupe animateur à un autre', () => {
    const d = moveLight(draftFromConfig(CFG), 'B', { kind: 'host', index: 1 })
    expect(locateLight(d, 'B')).toEqual({ kind: 'host', index: 1 })
  })
  it('orderChanged : vrai seulement si l\'ordre des groupes enregistrés change', () => {
    const saved = draftFromConfig(CFG)
    expect(orderChanged(saved, saved)).toBe(false)
    expect(orderChanged(addGroup(saved), saved)).toBe(false)
    expect(orderChanged(moveGroup(saved, 0, 1), saved)).toBe(true)
  })
  it('pas de l\'essai : une seconde chacun, 6 pas', () => {
    expect(TEST_STEPS).toHaveLength(6)
    expect([0, 999, 1000, 5999, 9000].map(ms => testStepAt(ms))).toEqual([0, 0, 1, 5, 5])
  })
})
