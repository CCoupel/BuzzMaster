import { describe, it, expect } from 'vitest'
import {
  draftFromConfig, draftToPayload, addGroup, removeGroup, moveGroup, moveLight, locateLight, validateDraft,
  groupLabelForLight, groupPillText, orderChanged, groupName, testRef, unassignedLights,
  MAX_GROUPS_PER_ZONE, testStepAt, TEST_STEPS,
} from './lightGroups'

const CFG = [
  { id: 'g1', role: 'salle', lights: ['A'] },
  { id: 'g2', role: 'salle', lights: [] },
  { id: 'h1', role: 'animateur', lights: ['B'] },
  { id: 'p1', role: 'public', lights: ['C', 'D'] },
]

describe('lightGroups — trois zones, groupes numérotés sans nom, id stable (#225, révision 4)', () => {
  it('lit les trois zones, ignore name / near_teams anciens, renvoie les id dans l\'ordre ambiance, animateur, public', () => {
    const old = CFG.map(g => ({ ...g, name: 'Ancien', near_teams: ['Rouges'] }))
    const d = draftFromConfig(old)
    expect(d.ambiance).toHaveLength(2)
    expect(d.animateur).toHaveLength(1)
    expect(d.public).toEqual([{ id: 'p1', lights: ['C', 'D'] }])
    expect(draftToPayload(d)).toEqual(CFG)
    expect(draftToPayload(draftFromConfig([{ role: 'public', lights: [] }, { role: 'salle', lights: [] }]))).toEqual([
      { role: 'salle', lights: [] }, { role: 'public', lights: [] },
    ])
  })
  it('un groupe créé n\'a pas d\'id (le serveur l\'attribue) ; réordonner garde l\'id', () => {
    let d = addGroup(draftFromConfig(CFG), 'public')
    expect(draftToPayload(d).at(-1)).toEqual({ role: 'public', lights: [] })
    d = moveGroup(d, 'ambiance', 0, 1)
    expect(d.ambiance.map(g => g.id)).toEqual(['g2', 'g1'])
  })
  it('12 groupes au plus par zone (pas au total)', () => {
    let d = draftFromConfig([])
    for (let i = 0; i < 15; i += 1) { d = addGroup(d, 'animateur'); d = addGroup(d, 'public') }
    expect(d.animateur).toHaveLength(MAX_GROUPS_PER_ZONE)
    expect(d.public).toHaveLength(MAX_GROUPS_PER_ZONE)
    expect(addGroup(d, 'ambiance').ambiance).toHaveLength(1)
    expect(removeGroup(d, 'public', 0).public).toHaveLength(11)
    expect(validateDraft(d).global).toEqual([])
  })
  it('une ampoule passe d\'un groupe à un autre, y compris d\'une zone à l\'autre ; jamais dans deux groupes', () => {
    const d = moveLight(draftFromConfig(CFG), 'B', { kind: 'group', zone: 'public', index: 0 })
    expect(locateLight(d, 'B')).toEqual({ kind: 'group', zone: 'public', index: 0 })
    expect(d.animateur[0].lights).toEqual([])
    expect(moveLight(d, 'B', { kind: 'group', zone: 'public', index: 9 })).toBe(d)
    expect(unassignedLights(draftFromConfig(CFG), ['A', 'B', 'X'])).toEqual(['X'])
  })
  it('libellés d\'après le rang, pastilles par zone, références d\'essai', () => {
    expect(groupName(2)).toBe('Groupe 3')
    expect(groupPillText(groupLabelForLight(CFG, 'A'))).toBe('Ambiance · g1')
    expect(groupPillText(groupLabelForLight(CFG, 'B'))).toBe('🎤 Animateur · g1')
    expect(groupPillText(groupLabelForLight(CFG, 'D'))).toBe('👥 Public · g1')
    expect(groupLabelForLight(CFG, 'Z')).toBeNull()
    expect([testRef('ambiance', 1), testRef('animateur', 0), testRef('public', 2)]).toEqual([2, 'host1', 'public3'])
  })
  it('orderChanged : vrai seulement si l\'ordre d\'une zone enregistrée change', () => {
    const saved = draftFromConfig(CFG)
    expect(orderChanged(saved, saved)).toBe(false)
    expect(orderChanged(addGroup(saved, 'public'), saved)).toBe(false)
    expect(orderChanged(moveGroup(saved, 'ambiance', 0, 1), saved)).toBe(true)
  })
  it('pas de l\'essai : une seconde chacun, 6 pas', () => {
    expect(TEST_STEPS).toHaveLength(6)
    expect([0, 999, 1000, 5999, 9000].map(ms => testStepAt(ms))).toEqual([0, 0, 1, 5, 5])
  })
})
