import { describe, it, expect } from 'vitest'
import { effectsPath, configKeyFor, configPatchFor, MOMENTS } from './lightEffects'
import { overriddenFields, setPartialPath } from './ambianceSettings'

// #223 (révision du GATE LOT-1) — parité des chemins éditables avec le serveur (contracts/light-effects.md §6,
// ambiance-settings.md §10) : `light.effects.<m>` et `light.host_effects.<id>.<m>` côté modèle/manche/API,
// `lighting.effects.<m>` et `lighting.host_effects.<id>.<m>` côté config.json. Le serveur (ambiancecfg.InvalidPaths)
// formate exactement les mêmes chemins ; ce test fige le côté React.

const MOMENT_KEYS = ['chrono', 'calm', 'score']

describe('chemins éditables des effets — parité front / serveur', () => {
  it('les trois moments du composant sont ceux du serveur', () => {
    expect(MOMENTS.map(m => m.key)).toEqual(MOMENT_KEYS)
  })

  it('salle : light.effects.<moment>', () => {
    for (const m of MOMENT_KEYS) expect(effectsPath(false, m)).toBe(`light.effects.${m}`)
  })

  it('groupe animateur : light.host_effects.<id>.<moment> (clé = id stable du groupe)', () => {
    for (const m of MOMENT_KEYS) expect(effectsPath('g5d7e4', m)).toBe(`light.host_effects.g5d7e4.${m}`)
  })

  it('configKeyFor convertit une fois light.* → lighting.* (jamais l\'inverse)', () => {
    expect(configKeyFor('light.effects.calm')).toEqual({ section: 'effects', moment: 'calm' })
    expect(configKeyFor('light.host_effects.ha1.score')).toEqual({ section: 'host_effects', id: 'ha1', moment: 'score' })
    expect(configKeyFor('lighting.effects.calm')).toBeNull()
    expect(configKeyFor('light.host_effects.score')).toBeNull() // ancienne forme à plat : plus un chemin éditable
    expect(configKeyFor('light.host_effects.HA1.score')).toBeNull() // id [a-z0-9]{1,12}
    expect(configKeyFor('light.host_effects.abcdefghijklm.score')).toBeNull()
    expect(configKeyFor('light.effects.bidule')).toBeNull()
  })

  it('configPatchFor : un moment à la fois (patch fusionné par le serveur)', () => {
    expect(configPatchFor({ section: 'effects', moment: 'chrono' }, 'chenillard')).toEqual({ effects: { chrono: 'chenillard' } })
    expect(configPatchFor({ section: 'host_effects', id: 'ha1', moment: 'score' }, 'couleur-equipe')).toEqual({ host_effects: { ha1: { score: 'couleur-equipe' } } })
    expect(configPatchFor({ section: 'effects', moment: 'calm' }, '')).toEqual({ effects: { calm: '' } })
  })

  it('aller-retour : chaque chemin de salle et d\'animateur est reconnu et reconstruit à l\'identique', () => {
    for (const m of MOMENT_KEYS) {
      for (const host of [false, 'ha1', 'zz9']) {
        const path = effectsPath(host, m)
        const key = configKeyFor(path)
        expect(key).not.toBeNull()
        expect(key.moment).toBe(m)
        expect(key.section).toBe(host ? 'host_effects' : 'effects')
        if (host) expect(key.id).toBe(host)
      }
    }
  })

  it('une surcharge de manche d\'un groupe animateur est comptée comme un réglage (badge « N réglages »)', () => {
    let partial = {}
    partial = setPartialPath(partial, 'light.host_effects.ha1.score', 'couleur-equipe')
    partial = setPartialPath(partial, 'light.host_effects.hb2.chrono', '')
    partial = setPartialPath(partial, 'light.effects.calm', 'scintillement')
    const paths = overriddenFields(partial).map(f => f.path)
    expect(paths).toEqual(expect.arrayContaining(['light.host_effects.ha1.score', 'light.host_effects.hb2.chrono', 'light.effects.calm']))
    expect(paths).toHaveLength(3)
  })

  it('« Hérité » (null) retire la clé, et le groupe quand il n\'a plus de moment surchargé', () => {
    let partial = setPartialPath({}, 'light.host_effects.ha1.score', 'couleur-equipe')
    partial = setPartialPath(partial, 'light.host_effects.ha1.score', null)
    expect(overriddenFields(partial)).toHaveLength(0)
    expect(JSON.stringify(partial)).not.toContain('ha1')
  })
})
