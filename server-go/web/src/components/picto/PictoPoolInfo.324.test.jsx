import { describe, it, expect, vi } from 'vitest'
import { render } from '@testing-library/react'
import PictoPanel from './PictoPanel'
import { AnimPictoView } from './AnimPicto'
import { pictoWordsInfo } from './pictoState'
import { createPictoCanvasStore } from './pictoCanvasStore'

// #324 (v13.1.0, LOT-2) — question sur Pool Picto : « Pool Picto · N mots disponibles », jamais « 0 mot dans la question ».
vi.mock('./PictoPanel.css', () => ({}))
vi.mock('./PictoCanvas.css', () => ({}))

const gs = (q = {}, o = {}) => ({
  phase: 'STARTED', timer: 60, question: { ID: 'q', TYPE: 'PICTO', CATEGORY: 'ANIMALS', POINTS: 10, TIME: 90, ...q },
  PICTO_TEAM: 'Bleus', PICTO_DRAWER_NAME: 'Alice', PICTO_CANVAS_EPOCH: 1, PICTO_WORD_CATEGORY: 'ANIMALS', ...o,
})
const actions = { found: vi.fn(), reveal: vi.fn(), start: vi.fn(), relaunch: vi.fn(), setTeam: vi.fn(), setDrawer: vi.fn() }
const CATS = [{ key: 'ANIMALS', name: 'Animaux', isCustom: true }]
const panel = (g) => render(<PictoPanel gameState={g} actions={actions} customCategories={CATS} />).container.textContent
const anim = (g) => render(<AnimPictoView gameState={g} teams={{ Bleus: { COLOR: [0, 0, 255] } }} bumpers={{ b1: { TEAM: 'Bleus' } }} actions={actions} store={createPictoCanvasStore()} customCategories={CATS} />).container.textContent

describe('pictoWordsInfo', () => {
  it('RESERVOIR : fromPool et mots disponibles', () => {
    expect(pictoWordsInfo(gs({ PICTO_SOURCE: 'RESERVOIR' }, { PICTO_POOL_AVAILABLE: 42 }))).toMatchObject({ fromPool: true, available: 42 })
  })
  it('source absente ou WORDS : mots saisis', () => {
    expect(pictoWordsInfo(gs({ PICTO_WORDS: ['a', 'b'] }))).toMatchObject({ fromPool: false, words: 2 })
    expect(pictoWordsInfo(gs({ PICTO_SOURCE: 'WORDS' }, { PICTO_WORDS_COUNT: 3 }))).toMatchObject({ fromPool: false, words: 3 })
  })
})

describe('Admin (PictoPanel) et anim — source des mots', () => {
  it('Pool Picto : « Pool Picto · 42 mots disponibles », pas de « 0 mot dans la question »', () => {
    const g = gs({ PICTO_SOURCE: 'RESERVOIR' }, { PICTO_POOL_AVAILABLE: 42 })
    for (const t of [panel(g), anim(g)]) {
      expect(t).toContain('Pool Picto · 42 mots disponibles')
      expect(t).not.toMatch(/0 mot/)
      expect(t).not.toContain('dans la question')
    }
    expect(anim(g)).toContain('tiré dans le Pool Picto')
  })
  it('Pool Picto : singulier à 1 mot, 0 disponible en toutes lettres', () => {
    expect(panel(gs({ PICTO_SOURCE: 'RESERVOIR' }, { PICTO_POOL_AVAILABLE: 1 }))).toContain('1 mot disponible')
    expect(anim(gs({ PICTO_SOURCE: 'RESERVOIR' }, { PICTO_POOL_AVAILABLE: 0 }))).toContain('0 mot disponible')
  })
  it('mots saisis : affichage actuel conservé', () => {
    const g = gs({ PICTO_WORDS: ['chat', 'chien', 'souris'] }, { PICTO_WORDS_COUNT: 3 })
    expect(panel(g)).toContain('3 mots dans la question')
    expect(anim(g)).toContain('3 mots dans la question')
    expect(anim(g)).toContain('tiré parmi les 3 mots de la question')
  })
})
