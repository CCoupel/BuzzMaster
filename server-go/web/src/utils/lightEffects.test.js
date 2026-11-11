import { describe, it, expect } from 'vitest'
import { buildOptions, effectWarning, normalizeCatalog, globalEffect, findHostGroups, configKeyFor, configPatchFor, effectsPath, INHERIT } from './lightEffects'
import { overriddenFields, setPartialPath, describeOverride } from './ambianceSettings'

const CAT = normalizeCatalog({
  groups: 2,
  effects: [
    { id: 'zzz', name: 'Zèbre', moments: ['chrono'], origin: 'user', min_groups: 1, applicable: true },
    { id: 'chenillard', name: 'Chenillard', moments: ['chrono'], origin: 'native', min_groups: 3, applicable: false, reason: '' },
    { id: 'pulsation', name: 'Pulsation', moments: ['chrono', 'calm'], origin: 'native', min_groups: 1, applicable: true },
    { id: 'tout', name: 'Tout', moments: ['calm'], origin: 'native', target: 'all', min_groups: 0, applicable: true, host_applicable: true },
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
    expect(o.find(x => x.value === 'chenillard')).toMatchObject({ disabled: true, reason: 'anime plusieurs groupes' })
    expect(o.find(x => x.value === 'pulsation').disabled).toBe(false)
  })
  it('valeur absente du catalogue : « (introuvable) » ; défaut marqué « (par défaut) »', () => {
    expect(buildOptions(CAT, 'calm', { selected: 'mon-effet' }).pop().label).toBe('mon-effet (introuvable)')
    expect(buildOptions(CAT, 'chrono', { defaultValue: 'pulsation' }).find(x => x.value === 'pulsation').label).toBe('Pulsation (par défaut)')
  })
  it('avertissements : inapplicable (repli pulsation au chrono) et introuvable', () => {
    expect(effectWarning(CAT, 'chrono', 'chenillard').text).toBe('Cet effet demande 3 groupes et 2 sont configurés : la pulsation est jouée à la place.')
    expect(effectWarning(CAT, 'chrono', 'nope').kind).toBe('missing')
    expect(effectWarning(CAT, 'chrono', 'pulsation')).toBeNull()
    expect(effectWarning(CAT, 'chrono', '')).toBeNull()
  })
  it('host_applicable filtre le groupe animateur ; target all toujours applicable en salle', () => {
    const cat = normalizeCatalog({ groups: 0, effects: [
      { id: 'a', name: 'A', moments: ['calm'], origin: 'native', target: 'all', min_groups: 0, host_applicable: true },
      { id: 'b', name: 'B', moments: ['calm'], origin: 'native', target: 'groups', min_groups: 1, host_applicable: false },
    ] })
    expect(buildOptions(cat, 'calm').find(o => o.value === 'a').disabled).toBe(false)
    const host = buildOptions(cat, 'calm', { host: true })
    expect(host.find(o => o.value === 'a').disabled).toBe(false)
    expect(host.find(o => o.value === 'b')).toMatchObject({ disabled: true, reason: 'anime plusieurs groupes' })
  })
  it('valeurs globales par défaut, correspondance config.json et groupe animateur', () => {
    expect(globalEffect({}, false, 'chrono')).toBe('pulsation')
    expect(globalEffect({ host_effects: { g5d7e4: { score: 'couleur-equipe' } } }, 'g5d7e4', 'score')).toBe('couleur-equipe')
    expect(globalEffect({ host_effects: { g5d7e4: { score: 'x' } } }, 'g0b6c9', 'score')).toBe('')
    expect(globalEffect({}, 'g5d7e4', 'chrono')).toBe('')
    expect(effectsPath('g5d7e4', 'calm')).toBe('light.host_effects.g5d7e4.calm')
    expect(effectsPath(null, 'calm')).toBe('light.effects.calm')
    const key = configKeyFor('light.host_effects.g5d7e4.calm')
    expect(key).toEqual({ section: 'host_effects', id: 'g5d7e4', moment: 'calm' })
    expect(configPatchFor(key, 'pulsation')).toEqual({ host_effects: { g5d7e4: { calm: 'pulsation' } } })
    expect(configPatchFor(configKeyFor('light.effects.chrono'), '')).toEqual({ effects: { chrono: '' } })
    expect(configKeyFor('light.host_effects.calm')).toBeNull()
    expect(configKeyFor('light.mode')).toBeNull()
    expect(findHostGroups([
      { id: 's1', name: 'S', role: 'salle' }, { id: 'a1', name: 'P', role: 'animateur', lights: ['L'] },
      { name: 'sans id', role: 'animateur' }, { id: 'a2', name: 'R', role: 'animateur', lights: [] },
    ])).toEqual([{ id: 'a1', rank: 1, empty: false }, { id: 'a2', rank: 3, empty: true }])
    expect(INHERIT).toBe('__inherit__')
  })
})

describe('ambianceSettings — champs d\'effet (#223)', () => {
  it('compte et décrit les effets surchargés, retire la surcharge avec null', () => {
    let p = setPartialPath({}, 'light.effects.calm', 'scintillement')
    p = setPartialPath(p, 'light.host_effects.a1.score', '')
    expect(overriddenFields(p).map(f => f.path)).toEqual(['light.effects.calm', 'light.host_effects.a1.score'])
    expect(describeOverride({ path: 'light.effects.calm', value: 'scintillement' })).toContain('phases calmes')
    expect(describeOverride({ path: 'light.host_effects.a1.score', value: '' }, c => c, id => `🎤 ${id}`)).toContain('🎤 a1')
    expect(setPartialPath(p, 'light.effects.calm', null)).toEqual({ light: { host_effects: { a1: { score: '' } } } })
  })
})

describe('lightEffects — vitesses (#223 R14)', () => {
  it('chemins, patch config.json, valeur globale et plafond', async () => {
    const m = await import('./lightEffects')
    expect(m.speedPath(null, 'calm')).toBe('light.speeds.calm')
    expect(m.speedPath('g5d7e4', 'score')).toBe('light.host_speeds.g5d7e4.score')
    expect(m.configPatchFor(m.configKeyFor('light.speeds.chrono'), 1.5)).toEqual({ speeds: { chrono: 1.5 } })
    expect(m.configPatchFor(m.configKeyFor('light.host_speeds.g5d7e4.calm'), 0.5)).toEqual({ host_speeds: { g5d7e4: { calm: 0.5 } } })
    expect(m.globalSpeed({}, false, 'chrono')).toBe(1)
    expect(m.globalSpeed({ speeds: { chrono: 2 }, host_speeds: { a: { calm: 0.75 } } }, false, 'chrono')).toBe(2)
    expect(m.globalSpeed({ host_speeds: { a: { calm: 0.75 } } }, 'a', 'calm')).toBe(0.75)
    expect(m.globalSpeed({ speeds: { chrono: 3 } }, false, 'chrono')).toBe(1)
    expect(m.formatSpeed(0.75)).toBe('×0,75')
    expect(m.isSpeedCapped(400, 2)).toBe(true)
    expect(m.isSpeedCapped(1000, 2)).toBe(false)
    expect(m.isSpeedCapped(undefined, 2)).toBe(false)
  })
})

describe('ambianceSettings — vitesses surchargées (#223 R14)', () => {
  it('compte effet et vitesse séparément, salle et groupe animateur', () => {
    const p = { light: { speeds: { chrono: 1.5 }, host_speeds: { a1: { calm: 2 } }, host_effects: { a1: { calm: 'pulsation' } } } }
    expect(overriddenFields(p).map(f => f.path)).toEqual(['light.speeds.chrono', 'light.host_effects.a1.calm', 'light.host_speeds.a1.calm'])
    expect(describeOverride({ path: 'light.speeds.chrono', value: 1.5 })).toBe('Vitesse (chronomètre) : ×1,5')
    expect(describeOverride({ path: 'light.host_speeds.a1.calm', value: 2 }, c => c, () => '🎤 Groupe animateur 1')).toBe('🎤 Groupe animateur 1 — vitesse (phases calmes) : ×2')
  })
})
