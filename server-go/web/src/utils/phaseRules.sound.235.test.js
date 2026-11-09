// @vitest-environment node
import { describe, it, expect } from 'vitest'
import { showSoundRow } from './phaseRules'

// ---------------------------------------------------------------------------
// #235 (v11.6.0, LOT-2) — ligne son de la conduite pendant une carte MEMOTION sonore.
// Contrat sound.md §14.4 « Conduite » : `phaseRules.showSoundRow` vrai pendant une carte sonore (admin et anim),
// même si l'hôte n'a pas de son. Hypothèse de signature à confirmer par dev-frontend :
// showSoundRow(phase, question, selectedCardId) — 3e paramètre = ID de la carte sélectionnée/retournée.
// Non-régression : les 2 premiers paramètres gardent leur sémantique v11.1 (phaseRules.sound.test.js).
// ---------------------------------------------------------------------------

const memotion = (cards, host = {}) => ({ ...host, TYPE: 'MEMOTION', MOTION_CARDS: cards })

describe('showSoundRow — carte MEMOTION sonore (#235)', () => {
  it('carte sélectionnée AVEC son, hôte muet, phase STARTED → visible', () => {
    const q = memotion([{ ID: 'c1', SOUND: '/question/1/motion_c1_sound_1234.wav' }, { ID: 'c2' }])
    expect(showSoundRow('STARTED', q, 'c1')).toBe(true)
  })

  it('carte sélectionnée SANS son, hôte muet → non visible', () => {
    const q = memotion([{ ID: 'c1', SOUND: '/x.wav' }, { ID: 'c2' }])
    expect(showSoundRow('STARTED', q, 'c2')).toBe(false)
  })

  it('aucune carte sélectionnée, hôte muet → non visible (même si une autre carte a un son)', () => {
    const q = memotion([{ ID: 'c1', SOUND: '/x.wav' }])
    expect(showSoundRow('STARTED', q, null)).toBe(false)
    expect(showSoundRow('STARTED', q)).toBe(false)
  })

  it('hôte AVEC son → visible quelle que soit la carte (comportement v11.1 conservé)', () => {
    const q = memotion([{ ID: 'c1' }], { SOUND: '/question/1/sound_1234.wav' })
    expect(showSoundRow('STARTED', q, null)).toBe(true)
    expect(showSoundRow('STARTED', q, 'c1')).toBe(true)
  })

  it.each(['STOPPED', 'READY', 'COUNTDOWN', 'REVEALED', 'PREPARE', 'NEW_GAME'])('phase %s : jamais visible, même carte sonore', (phase) => {
    const q = memotion([{ ID: 'c1', SOUND: '/x.wav' }])
    expect(showSoundRow(phase, q, 'c1')).toBe(false)
  })

  it('phase PAUSED + carte sonore → visible', () => {
    expect(showSoundRow('PAUSED', memotion([{ ID: 'c1', SOUND: '/x.wav' }]), 'c1')).toBe(true)
  })

  it('ID de carte inconnu / MOTION_CARDS absent / question nulle → non visible, sans exception', () => {
    expect(showSoundRow('STARTED', memotion([{ ID: 'c1', SOUND: '/x.wav' }]), 'zzz')).toBe(false)
    expect(showSoundRow('STARTED', { TYPE: 'MEMOTION' }, 'c1')).toBe(false)
    expect(showSoundRow('STARTED', null, 'c1')).toBe(false)
  })
})
