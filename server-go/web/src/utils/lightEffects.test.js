import { describe, it, expect } from 'vitest'
import { buildOptions, effectWarning, normalizeCatalog, globalEffect, findHostGroup, configKeyFor, INHERIT } from './lightEffects'
import { overriddenFields, setPartialPath, describeOverride } from './ambianceSettings'

const CAT = normalizeCatalog({
  groups: 2,
  effects: [
    { id: 'zzz', name: 'Zèbre', moments: ['chrono'], origin: 'user', min_groups: 1, applicable: true },
    { id: 'chenillard', name: 'Chenillard', moments: ['chrono'], origin: 'native', min_groups: 3, applicable: false, reason: '' },
    { id: 'pulsation', name: 'Pulsation', moments: ['chrono', 'calm'], origin: 'native', min_groups: 1, applicable: true },
    { id: 'aaa', name: 'Arc', moments: ['chrono'], origin: 'override', min_groups: 1, applicable: true },
  ],
})

describe('lightEffects — listes par moment (#223)', () => {
  it('Aucun effet, natifs puis personnalisés par nom, mention « (personnalisé) »', () => {
    const o = buildOptions(CAT, 'chrono')
    expect(o.map(x => x.value)).toEqual(['', 'chenillard', 'pulsation', 'aaa', 'zzz'])
    expect(o.find(x => x.value === 'aaa').label).toBe('Arc (personnalisé)')
  })
  it('un effet inapplicable reste visible, grisé, avec sa raison', () => {
    const c = buildOptions(CAT, 'chrono').find(x => x.value === 'chenillard')
    expect(c.disabled).toBe(true)
    expect(c.text).toBe('Chenillard — demande 3 groupes, 2 configurés')
  })
  it('groupe animateur : « Blanc fixe », effets multi-groupes grisés « demande plusieurs groupes »', () => {
    const o = buildOptions(CAT, 'chrono', { host: true })
    expect(o[0].label).toBe('Blanc fixe')
    expect(o.find(x => x.value === 'chenillard')).toMatchObject({ disabled: true, reason: 'demande plusieurs groupes' })
    expect(o.find(x => x.value === 'pulsation').disabled).toBe(false)
  })
  it('valeur absente du catalogue : « (introuvable) » ; défaut marqué « (actuel) »', () => {
    expect(buildOptions(CAT, 'calm', { selected: 'mon-effet' }).pop().label).toBe('mon-effet (introuvable)')
    expect(buildOptions(CAT, 'chrono', { defaultValue: 'pulsation' }).find(x => x.value === 'pulsation').label).toBe('Pulsation (actuel)')
  })
  it('avertissements : inapplicable (repli pulsation au chrono) et introuvable', () => {
    expect(effectWarning(CAT, 'chrono', 'chenillard').text).toBe('Cet effet demande 3 groupes et 2 sont configurés : la pulsation est jouée à la place.')
    expect(effectWarning(CAT, 'chrono', 'nope').kind).toBe('missing')
    expect(effectWarning(CAT, 'chrono', 'pulsation')).toBeNull()
    expect(effectWarning(CAT, 'chrono', '')).toBeNull()
  })
  it('valeurs globales par défaut, correspondance config.json et groupe animateur', () => {
    expect(globalEffect({}, false, 'chrono')).toBe('pulsation')
    expect(globalEffect({ host_effects: { score: 'couleur-equipe' } }, true, 'score')).toBe('couleur-equipe')
    expect(globalEffect({}, true, 'chrono')).toBe('')
    expect(configKeyFor('light.host_effects.calm')).toEqual({ section: 'host_effects', moment: 'calm' })
    expect(configKeyFor('light.mode')).toBeNull()
    expect(findHostGroup([{ name: 'S', role: 'salle' }, { name: 'P', role: 'animateur' }])).toEqual({ name: 'P' })
    expect(INHERIT).toBe('__inherit__')
  })
})

describe('ambianceSettings — champs d\'effet (#223)', () => {
  it('compte et décrit les effets surchargés, retire la surcharge avec null', () => {
    let p = setPartialPath({}, 'light.effects.calm', 'scintillement')
    p = setPartialPath(p, 'light.host_effects.score', '')
    expect(overriddenFields(p).map(f => f.path)).toEqual(['light.effects.calm', 'light.host_effects.score'])
    expect(describeOverride({ path: 'light.effects.calm', value: 'scintillement' })).toContain('phases calmes')
    expect(setPartialPath(p, 'light.effects.calm', null)).toEqual({ light: { host_effects: { score: '' } } })
  })
})
