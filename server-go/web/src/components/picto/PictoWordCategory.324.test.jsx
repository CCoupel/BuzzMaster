import { describe, it, expect, vi } from 'vitest'
import { render } from '@testing-library/react'
import { AnimPictoView } from './AnimPicto'
import { createPictoCanvasStore } from './pictoCanvasStore'

// #324 (v13.1.0, LOT-2 T14) — indice PICTO_WORD_CATEGORY : catégorie du mot en cours (pool), sinon celle de la question.
vi.mock('./PictoPanel.css', () => ({}))
vi.mock('./PictoCanvas.css', () => ({}))

const CATS = [{ key: 'ANIMALS', name: 'Animaux', isCustom: true }, { key: 'OBJECTS', name: 'Objets', isCustom: true }]
const gs = (o = {}) => ({
  phase: 'STARTED', timer: 60, question: { ID: 'q', TYPE: 'PICTO', CATEGORY: 'ANIMALS', POINTS: 10, TIME: 90 },
  PICTO_TEAM: 'Bleus', PICTO_DRAWER_NAME: 'Alice', PICTO_CANVAS_EPOCH: 1, ...o,
})
const actions = { found: vi.fn(), reveal: vi.fn(), start: vi.fn(), relaunch: vi.fn(), setTeam: vi.fn(), setDrawer: vi.fn() }
const bar = (g) => render(<AnimPictoView gameState={g} teams={{ Bleus: { COLOR: [0, 0, 255] } }} bumpers={{ b1: { TEAM: 'Bleus' } }} actions={actions} store={createPictoCanvasStore()} customCategories={CATS} />).container.querySelector('.anim-picto-bar')

describe('Anim Picto — indice de catégorie', () => {
  it('PICTO_WORD_CATEGORY (pool) : le chip montre la catégorie du mot tiré', () => {
    const b = bar(gs({ PICTO_WORD_CATEGORY: 'OBJECTS' }))
    expect(b.textContent).toContain('Objets')
    expect(b.textContent).not.toContain('Animaux')
  })
  it('sans indice (mots saisis) : catégorie de la question', () => {
    const b = bar(gs({ PICTO_WORD_CATEGORY: '' }))
    expect(b.textContent).toContain('Animaux')
  })
})
