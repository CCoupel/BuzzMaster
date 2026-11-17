// @vitest-environment node
import { describe, it, expect } from 'vitest'
import { participantsConform, prepareWaitReasons } from './prepareWaitReason'

// #144 (v12.0.0, LOT-1.2) — miroir client-side du prédicat serveur participantsConform pour le type ROUE
// (contracts/roue.md §13 item 4 ; internal/game engine.go) : SOLO = exactement 1 équipe, autres modes = 2 au moins.
// Fichier distinct : prepareWaitReason.test.js existant reste immuable.

const q = (mode) => ({ TYPE: 'ROUE', ...(mode ? { ROUE_MODE: mode } : {}) })

describe('participantsConform — ROUE', () => {
  it('SOLO : exactement 1 équipe', () => {
    expect(participantsConform(q('SOLO'), [])).toBe(false)
    expect(participantsConform(q('SOLO'), ['a'])).toBe(true)
    expect(participantsConform(q('SOLO'), ['a', 'b'])).toBe(false)
  })
  it.each(['CHACUN_SON_TOUR', 'TANT_QUE_JE_GAGNE'])('%s : 2 équipes au moins', (mode) => {
    expect(participantsConform(q(mode), ['a'])).toBe(false)
    expect(participantsConform(q(mode), ['a', 'b'])).toBe(true)
    expect(participantsConform(q(mode), ['a', 'b', 'c'])).toBe(true)
  })
  it('mode absent = TANT_QUE_JE_GAGNE (même défaut que le serveur) : 2 équipes au moins', () => {
    expect(participantsConform(q(), ['a'])).toBe(false)
    expect(participantsConform(q(), ['a', 'b'])).toBe(true)
  })
})

describe('prepareWaitReasons — ROUE', () => {
  const ready = [{ READY: true }, { READY: true }]
  it('lit ROUE_TEAMS (et non les équipes MEMORY / RAFALE)', () => {
    const gs = { ROUE_TEAMS: ['a', 'b'], MEMORY_PARTICIPATING_TEAMS: [], RAFALE_PARTICIPATING_TEAMS: [] }
    expect(prepareWaitReasons('PREPARE', q('CHACUN_SON_TOUR'), ready, gs)).toEqual([])
  })
  it('équipes insuffisantes : motif lisible (SOLO / multi)', () => {
    expect(prepareWaitReasons('PREPARE', q('SOLO'), ready, { ROUE_TEAMS: [] })).toEqual(['sélectionnez une équipe'])
    expect(prepareWaitReasons('PREPARE', q('TANT_QUE_JE_GAGNE'), ready, { ROUE_TEAMS: ['a'] })).toEqual(['sélectionnez au moins deux équipes participantes'])
    expect(prepareWaitReasons('PREPARE', q('SOLO'), ready, { ROUE_TEAMS: [] }, { short: true })).toEqual(['1 équipe'])
    expect(prepareWaitReasons('PREPARE', q('CHACUN_SON_TOUR'), ready, { ROUE_TEAMS: [] }, { short: true })).toEqual(['2 équipes'])
  })
  it('se cumule avec « buzzers en attente »', () => {
    const r = prepareWaitReasons('PREPARE', q('CHACUN_SON_TOUR'), [{ READY: true }, { READY: false }], { ROUE_TEAMS: ['a'] })
    expect(r).toHaveLength(2)
    expect(r[0]).toMatch(/Buzzers en attente : 1\/2/)
  })
  it('hors PREPARE : aucun motif', () => {
    expect(prepareWaitReasons('READY', q('SOLO'), ready, { ROUE_TEAMS: [] })).toEqual([])
  })
})

describe('motif ROUE SOLO — libellé selon le nombre d\'équipes cochées', () => {
  const ready = [{ READY: true }, { READY: true }]
  it('aucune équipe : « sélectionnez une équipe »', () => {
    expect(prepareWaitReasons('PREPARE', q('SOLO'), ready, { ROUE_TEAMS: [] })).toEqual(['sélectionnez une équipe'])
  })
  it('plus d\'une équipe : « sélectionnez une seule équipe »', () => {
    expect(prepareWaitReasons('PREPARE', q('SOLO'), ready, { ROUE_TEAMS: ['a', 'b'] })).toEqual(['sélectionnez une seule équipe'])
  })
})
