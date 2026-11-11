import { describe, it, expect } from 'vitest'
import { effectsPath, configKeyFor, configPatchFor, MOMENTS, speedPath, SPEEDS, DEFAULT_SPEED, formatSpeed, isSpeedCapped } from './lightEffects'
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

  // ---- vitesse (R13 / R14) ----

  it('les cinq vitesses du composant sont exactement celles du serveur (ambiancecfg.SpeedValues), défaut ×1', () => {
    expect([...SPEEDS]).toEqual([0.5, 0.75, 1, 1.5, 2])
    expect(DEFAULT_SPEED).toBe(1)
    expect(SPEEDS.map(formatSpeed)).toEqual(['×0,5', '×0,75', '×1', '×1,5', '×2'])
  })

  it('chemins de vitesse : light.speeds.<moment> et light.host_speeds.<id>.<moment>', () => {
    for (const m of MOMENT_KEYS) {
      expect(speedPath(false, m)).toBe(`light.speeds.${m}`)
      expect(speedPath('g5d7e4', m)).toBe(`light.host_speeds.g5d7e4.${m}`)
    }
  })

  it('configKeyFor / configPatchFor couvrent les vitesses (lighting.speeds, lighting.host_speeds)', () => {
    expect(configKeyFor('light.speeds.chrono')).toEqual({ section: 'speeds', moment: 'chrono' })
    expect(configKeyFor('light.host_speeds.ha1.calm')).toEqual({ section: 'host_speeds', id: 'ha1', moment: 'calm' })
    expect(configKeyFor('light.host_speeds.calm')).toBeNull()
    expect(configKeyFor('lighting.speeds.chrono')).toBeNull()
    expect(configPatchFor({ section: 'speeds', moment: 'chrono' }, 2)).toEqual({ speeds: { chrono: 2 } })
    expect(configPatchFor({ section: 'host_speeds', id: 'ha1', moment: 'score' }, 0.5)).toEqual({ host_speeds: { ha1: { score: 0.5 } } })
  })

  it('aller-retour de tous les chemins de vitesse', () => {
    for (const m of MOMENT_KEYS) {
      for (const host of [false, 'ha1']) {
        const key = configKeyFor(speedPath(host, m))
        expect(key).not.toBeNull()
        expect(key.moment).toBe(m)
        expect(key.section).toBe(host ? 'host_speeds' : 'speeds')
      }
    }
  })

  it('« plafonné » : le tick ÷ la vitesse tombe sous 250 ms (même règle que le serveur : jamais sous 250 ms)', () => {
    expect(isSpeedCapped(400, 2)).toBe(true) // 200 ms
    expect(isSpeedCapped(500, 2)).toBe(false) // 250 ms pile
    expect(isSpeedCapped(1000, 2)).toBe(false)
    expect(isSpeedCapped(250, 1.5)).toBe(true)
    expect(isSpeedCapped(undefined, 2)).toBe(false) // durée de tick inconnue : jamais « plafonné »
  })

  it('une surcharge de vitesse de manche est comptée comme un réglage', () => {
    let partial = setPartialPath({}, 'light.speeds.chrono', 2)
    partial = setPartialPath(partial, 'light.host_speeds.ha1.score', 0.5)
    const paths = overriddenFields(partial).map(f => f.path)
    expect(paths).toEqual(expect.arrayContaining(['light.speeds.chrono', 'light.host_speeds.ha1.score']))
    expect(setPartialPath(partial, 'light.speeds.chrono', null)).not.toHaveProperty('light.speeds.chrono')
  })
})
