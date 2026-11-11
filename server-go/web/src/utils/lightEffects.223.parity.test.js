import { describe, it, expect } from 'vitest'
import { effectsPath, configKeyFor, configPatchFor, MOMENTS, ZONES, speedPath, SPEEDS, DEFAULT_SPEED, formatSpeed, isSpeedCapped } from './lightEffects'
import { overriddenFields, setPartialPath } from './ambianceSettings'

// #223 (révision 4) — parité des chemins éditables avec le serveur (contracts/light-effects.md §6, ambiance-settings.md §10) :
// `light.effects.<zone>.<moment>` et `light.speeds.<zone>.<moment>` côté modèle/manche/API (3 zones × 3 moments),
// `lighting.effects.<zone>.<moment>` et `lighting.speeds.<zone>.<moment>` côté config.json. Le serveur
// (ambiancecfg.InvalidPaths) formate exactement les mêmes chemins ; ce test fige le côté React.
// Plus de `host_effects` ni d'id de groupe : le groupe n'est la clé d'aucun réglage.

const MOMENT_KEYS = ['chrono', 'calm', 'score']
const ZONE_KEYS = ['ambiance', 'animateur', 'public']

describe('chemins éditables des effets — parité front / serveur', () => {
  it('les trois moments et les trois zones du composant sont ceux du serveur', () => {
    expect(MOMENTS.map(m => m.key)).toEqual(MOMENT_KEYS)
    expect([...ZONES]).toEqual(ZONE_KEYS)
  })

  it('light.effects.<zone>.<moment> pour chacune des neuf cases', () => {
    for (const z of ZONE_KEYS) for (const m of MOMENT_KEYS) expect(effectsPath(z, m)).toBe(`light.effects.${z}.${m}`)
  })

  it('configKeyFor convertit une fois light.* → lighting.* (jamais l\'inverse)', () => {
    expect(configKeyFor('light.effects.ambiance.calm')).toEqual({ section: 'effects', zone: 'ambiance', moment: 'calm' })
    expect(configKeyFor('light.effects.public.score')).toEqual({ section: 'effects', zone: 'public', moment: 'score' })
    expect(configKeyFor('lighting.effects.ambiance.calm')).toBeNull()
    expect(configKeyFor('light.effects.calm')).toBeNull() // ancienne forme à plat : plus un chemin éditable
    expect(configKeyFor('light.host_effects.ha1.score')).toBeNull() // host_effects n'existe plus
    expect(configKeyFor('light.effects.salle.calm')).toBeNull() // zone inconnue
    expect(configKeyFor('light.effects.ambiance.bidule')).toBeNull()
  })

  it('configPatchFor : une zone et un moment à la fois (patch fusionné par le serveur)', () => {
    expect(configPatchFor({ section: 'effects', zone: 'ambiance', moment: 'chrono' }, 'chenillard')).toEqual({ effects: { ambiance: { chrono: 'chenillard' } } })
    expect(configPatchFor({ section: 'effects', zone: 'animateur', moment: 'score' }, 'couleur-equipe')).toEqual({ effects: { animateur: { score: 'couleur-equipe' } } })
    expect(configPatchFor({ section: 'effects', zone: 'public', moment: 'calm' }, '')).toEqual({ effects: { public: { calm: '' } } })
  })

  it('aller-retour : chacun des neuf chemins est reconnu et reconstruit à l\'identique', () => {
    for (const z of ZONE_KEYS) {
      for (const m of MOMENT_KEYS) {
        const key = configKeyFor(effectsPath(z, m))
        expect(key).toEqual({ section: 'effects', zone: z, moment: m })
      }
    }
  })

  it('une surcharge de manche est comptée comme un réglage (badge « N réglages »), par zone et par moment', () => {
    let partial = {}
    partial = setPartialPath(partial, 'light.effects.animateur.score', 'couleur-equipe')
    partial = setPartialPath(partial, 'light.effects.public.chrono', '')
    partial = setPartialPath(partial, 'light.effects.ambiance.calm', 'scintillement')
    const paths = overriddenFields(partial).map(f => f.path)
    expect(paths).toEqual(expect.arrayContaining(['light.effects.animateur.score', 'light.effects.public.chrono', 'light.effects.ambiance.calm']))
    expect(paths).toHaveLength(3)
  })

  it('« Hérité » (null) retire la clé, et la zone quand elle n\'a plus de moment surchargé', () => {
    let partial = setPartialPath({}, 'light.effects.animateur.score', 'couleur-equipe')
    partial = setPartialPath(partial, 'light.effects.animateur.score', null)
    expect(overriddenFields(partial)).toHaveLength(0)
    expect(JSON.stringify(partial)).not.toContain('animateur')
  })

  // ---- vitesse (R13 / R14) ----

  it('les cinq vitesses du composant sont exactement celles du serveur (ambiancecfg.SpeedValues), défaut ×1', () => {
    expect([...SPEEDS]).toEqual([0.5, 0.75, 1, 1.5, 2])
    expect(DEFAULT_SPEED).toBe(1)
    expect(SPEEDS.map(formatSpeed)).toEqual(['×0,5', '×0,75', '×1', '×1,5', '×2'])
  })

  it('chemins de vitesse : light.speeds.<zone>.<moment>', () => {
    for (const z of ZONE_KEYS) for (const m of MOMENT_KEYS) expect(speedPath(z, m)).toBe(`light.speeds.${z}.${m}`)
  })

  it('configKeyFor / configPatchFor couvrent les vitesses (lighting.speeds.<zone>.<moment>)', () => {
    expect(configKeyFor('light.speeds.ambiance.chrono')).toEqual({ section: 'speeds', zone: 'ambiance', moment: 'chrono' })
    expect(configKeyFor('light.speeds.public.calm')).toEqual({ section: 'speeds', zone: 'public', moment: 'calm' })
    expect(configKeyFor('light.speeds.chrono')).toBeNull()
    expect(configKeyFor('light.host_speeds.ha1.calm')).toBeNull()
    expect(configKeyFor('lighting.speeds.ambiance.chrono')).toBeNull()
    expect(configPatchFor({ section: 'speeds', zone: 'ambiance', moment: 'chrono' }, 2)).toEqual({ speeds: { ambiance: { chrono: 2 } } })
    expect(configPatchFor({ section: 'speeds', zone: 'animateur', moment: 'score' }, 0.5)).toEqual({ speeds: { animateur: { score: 0.5 } } })
  })

  it('aller-retour de tous les chemins de vitesse', () => {
    for (const z of ZONE_KEYS) {
      for (const m of MOMENT_KEYS) {
        expect(configKeyFor(speedPath(z, m))).toEqual({ section: 'speeds', zone: z, moment: m })
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
    let partial = setPartialPath({}, 'light.speeds.ambiance.chrono', 2)
    partial = setPartialPath(partial, 'light.speeds.animateur.score', 0.5)
    const paths = overriddenFields(partial).map(f => f.path)
    expect(paths).toEqual(expect.arrayContaining(['light.speeds.ambiance.chrono', 'light.speeds.animateur.score']))
    expect(overriddenFields(setPartialPath(partial, 'light.speeds.ambiance.chrono', null)).map(f => f.path)).toEqual(['light.speeds.animateur.score'])
  })
})
