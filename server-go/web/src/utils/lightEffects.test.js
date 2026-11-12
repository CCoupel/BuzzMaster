import { describe, it, expect } from 'vitest'
import {
  buildOptions, effectWarning, normalizeCatalog, globalEffect, globalSpeed, zoneGroupCounts, visibleZones, isQuestionRunning, configKeyFor,
  configPatchFor, effectsPath, speedPath, effectFieldPaths, defaultEffect, formatSpeed, isSpeedCapped, inapplicableReason, INHERIT,
} from './lightEffects'
import { overriddenFields, setPartialPath, describeOverride, SETTINGS_FIELD_PATHS } from './ambianceSettings'

const CAT = normalizeCatalog({
  groups: { ambiance: 2, animateur: 1, public: 0 },
  effects: [
    { id: 'zzz', name: 'Zèbre', moments: ['chrono'], origin: 'user', min_groups: 1, tick_ms: 1000, applicable: { ambiance: true, animateur: true, public: false } },
    { id: 'chenillard', name: 'Chenillard', moments: ['chrono'], origin: 'native', min_groups: 3, tick_ms: 400, applicable: { ambiance: false, animateur: false, public: false } },
    { id: 'pulsation', name: 'Pulsation', moments: ['chrono', 'calm'], origin: 'native', min_groups: 0, applicable: { ambiance: true, animateur: true, public: false } },
    { id: 'aaa', name: 'Arc', moments: ['chrono'], origin: 'override', min_groups: 1, applicable: { ambiance: true, animateur: true, public: false } },
  ],
})

describe('lightEffects — listes par moment et par zone (#223, révision 4)', () => {
  it('Aucun effet, natifs puis personnalisés par nom, mention « (personnalisé) »', () => {
    const o = buildOptions(CAT, 'chrono')
    expect(o.map(x => x.value)).toEqual(['', 'chenillard', 'pulsation', 'aaa', 'zzz'])
    expect(o.find(x => x.value === 'aaa').label).toBe('Arc (personnalisé)')
  })
  it('un effet inapplicable à la zone reste visible, grisé, avec « demande n groupes, m dans cette zone »', () => {
    const c = buildOptions(CAT, 'chrono', { zone: 'ambiance' }).find(x => x.value === 'chenillard')
    expect(c).toMatchObject({ disabled: true, text: 'Chenillard — demande 3 groupes, 2 dans cette zone' })
    expect(buildOptions(CAT, 'chrono', { zone: 'animateur' }).find(x => x.value === 'chenillard').text).toBe('Chenillard — demande 3 groupes, 1 dans cette zone')
    expect(buildOptions(CAT, 'chrono', { zone: 'public' }).find(x => x.value === 'pulsation').disabled).toBe(true)
  })
  it('la raison « anime plusieurs groupes » n\'existe plus : la zone Animateur obéit à applicable.animateur', () => {
    const host = buildOptions(CAT, 'chrono', { zone: 'animateur' })
    expect(host[0].label).toBe('Aucun effet (blanc fixe)')
    expect(host.find(x => x.value === 'pulsation').disabled).toBe(false)
    expect(JSON.stringify(host)).not.toMatch(/anime plusieurs/)
  })
  it('valeur absente du catalogue : « (introuvable) » ; défaut marqué « (par défaut) » sauf zone Animateur', () => {
    expect(buildOptions(CAT, 'calm', { selected: 'mon-effet' }).pop().label).toBe('mon-effet (introuvable)')
    expect(buildOptions(CAT, 'chrono', { defaultValue: 'pulsation' }).find(x => x.value === 'pulsation').label).toBe('Pulsation (par défaut)')
    expect(buildOptions(CAT, 'chrono', { zone: 'animateur', defaultValue: '' })[0].label).toBe('Aucun effet (blanc fixe)')
  })
  it('avertissements par zone : repli pulsation (ambiance, chrono), blanc fixe, zone public, introuvable', () => {
    expect(effectWarning(CAT, 'chrono', 'chenillard').text).toBe('Cet effet demande 3 groupes et la zone en a 2 : la pulsation est jouée à la place.')
    expect(effectWarning(CAT, 'chrono', 'chenillard', { zone: 'animateur' }).text).toMatch(/la zone reste en blanc fixe/)
    expect(effectWarning(CAT, 'chrono', 'pulsation', { zone: 'public' }).text).toMatch(/rien n'est envoyé aux lampes du public/)
    expect(effectWarning(CAT, 'chrono', 'nope', { zone: 'animateur' })).toMatchObject({ kind: 'missing' })
    expect(effectWarning(CAT, 'chrono', 'pulsation')).toBeNull()
    expect(effectWarning(CAT, 'chrono', '')).toBeNull()
  })
  it('catalogue antérieur : groups numérique ⇒ zone ambiance, applicable booléen ⇒ toutes les zones, repli N ≥ min_groups', () => {
    const old = normalizeCatalog({ groups: 2, effects: [{ id: 'x', name: 'X', moments: ['calm'], origin: 'native', min_groups: 3 }] })
    expect(old.groups).toEqual({ ambiance: 2, animateur: 0, public: 0 })
    expect(inapplicableReason(old.effects[0], 'ambiance', old.groups)).toBe('demande 3 groupes, 2 dans cette zone')
    expect(inapplicableReason({ id: 'y', applicable: true }, 'public', old.groups)).toBe('')
  })
})

describe('lightEffects — réglages par zone (#223, révision 4)', () => {
  it('chemins light.* et patch config.json lighting.*', () => {
    expect(effectsPath('animateur', 'calm')).toBe('light.effects.animateur.calm')
    expect(speedPath('public', 'score')).toBe('light.speeds.public.score')
    const key = configKeyFor('light.effects.animateur.score')
    expect(key).toEqual({ section: 'effects', zone: 'animateur', moment: 'score' })
    expect(configPatchFor(key, 'couleur-equipe')).toEqual({ effects: { animateur: { score: 'couleur-equipe' } } })
    expect(configPatchFor(configKeyFor('light.speeds.public.chrono'), 1.5)).toEqual({ speeds: { public: { chrono: 1.5 } } })
    expect(configKeyFor('light.effects.chrono')).toBeNull()
    expect(configKeyFor('light.host_effects.a1.calm')).toBeNull()
  })
  it('valeurs globales par défaut et lecture tolérante de la forme à plat (zone ambiance)', () => {
    expect(globalEffect({}, 'ambiance', 'chrono')).toBe('pulsation')
    expect(defaultEffect('public', 'chrono')).toBe('')
    expect(globalEffect({ effects: { animateur: { score: 'couleur-equipe' } } }, 'animateur', 'score')).toBe('couleur-equipe')
    expect(globalEffect({ effects: { animateur: { score: 'x' } } }, 'public', 'score')).toBe('')
    expect(globalEffect({ effects: { chrono: 'chenillard' } }, 'ambiance', 'chrono')).toBe('chenillard')
    expect(globalEffect({ effects: { chrono: 'chenillard' } }, 'animateur', 'chrono')).toBe('')
    expect(globalSpeed({}, 'ambiance', 'chrono')).toBe(1)
    expect(globalSpeed({ speeds: { public: { calm: 0.75 } } }, 'public', 'calm')).toBe(0.75)
    expect(globalSpeed({ speeds: { ambiance: { chrono: 3 } } }, 'ambiance', 'chrono')).toBe(1)
  })
  it('zones qui existent : Ambiance toujours, Animateur et Public s\'ils ont un groupe', () => {
    const counts = zoneGroupCounts([
      { role: 'salle', lights: ['A'] }, { role: 'animateur', lights: ['B'] }, { role: 'animateur', lights: ['C'] },
      { role: 'public', lights: [] }, { role: 'salle', lights: [] },
    ])
    // un groupe VIDE ne compte pas : la zone Public (groupe vide seulement) n'existe pas
    expect(counts).toEqual({ ambiance: 1, animateur: 2, public: 0 })
    expect(visibleZones(counts)).toEqual(['ambiance', 'animateur'])
    expect(visibleZones({ ambiance: 0, animateur: 0, public: 1 })).toEqual(['ambiance', 'public'])
  })
  it('question en cours : toute phase sauf STOPPED / NEW_GAME / ENROLL', () => {
    expect(['STARTED', 'PAUSED', 'REVEALED', 'COUNTDOWN', 'PREPARE', 'READY'].every(isQuestionRunning)).toBe(true)
    expect(['STOPPED', 'NEW_GAME', 'ENROLL', undefined, ''].some(isQuestionRunning)).toBe(false)
  })
  it('vitesse : format et plafond', () => {
    expect(formatSpeed(0.75)).toBe('×0,75')
    expect(isSpeedCapped(400, 2)).toBe(true)
    expect(isSpeedCapped(1000, 2)).toBe(false)
    expect(isSpeedCapped(undefined, 2)).toBe(false)
    expect(INHERIT).toBe('__inherit__')
  })
})

describe('ambianceSettings — champs d\'effet et de vitesse par zone (#223)', () => {
  it('parité : 18 chemins éditables, identiques à ceux de lightEffects', () => {
    const light = SETTINGS_FIELD_PATHS.filter(p => /^light\.(effects|speeds)\./.test(p))
    expect(light).toHaveLength(18)
    expect([...light].sort()).toEqual([...effectFieldPaths()].sort())
  })
  it('compte effet et vitesse séparément par zone et moment ; null retire la surcharge', () => {
    let p = setPartialPath({}, 'light.effects.ambiance.calm', 'degrade')
    p = setPartialPath(p, 'light.speeds.animateur.chrono', 2)
    p = setPartialPath(p, 'light.effects.public.score', '')
    expect(overriddenFields(p).map(f => f.path)).toEqual([
      'light.effects.ambiance.calm', 'light.effects.public.score', 'light.speeds.animateur.chrono',
    ])
    expect(describeOverride({ path: 'light.effects.ambiance.calm', value: 'degrade' })).toBe('Effet Ambiance (phases calmes) : degrade')
    expect(describeOverride({ path: 'light.speeds.animateur.chrono', value: 1.5 })).toBe('Vitesse Animateur (chronomètre) : ×1,5')
    expect(describeOverride({ path: 'light.effects.public.score', value: '' })).toBe('Effet Public (équipe qui marque) : aucun')
    expect(setPartialPath(p, 'light.effects.ambiance.calm', null)).toEqual({ light: { effects: { public: { score: '' } }, speeds: { animateur: { chrono: 2 } } } })
  })
})
