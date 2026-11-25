import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, act } from '@testing-library/react'
import PictoTv from './PictoTv'
import { createPictoCanvasStore } from './pictoCanvasStore'

vi.mock('../Timer', () => ({ default: ({ currentTime }) => <div data-testid="timer">{currentTime}</div> }))
beforeEach(() => { HTMLCanvasElement.prototype.getContext = () => null; vi.useFakeTimers() })
afterEach(() => { vi.useRealTimers() })

// #318 (v13.1.0, LOT-3 T24, maquette M4) — TV Rafale Picto : grand compteur, mot trouvé 1 s sur le dessin, repère de passe.
const gs = (o = {}) => ({
  phase: 'STARTED', timer: 42, totalTime: 90,
  question: { ID: 'q', TYPE: 'RAFALE_PICTO', CATEGORY: 'ANIMALS', POINTS: 10, TIME: 90 },
  PICTO_TEAM: 'Bleue', PICTO_DRAWER: 'b1', PICTO_DRAWER_NAME: 'Alice', PICTO_CANVAS_EPOCH: 2,
  PICTO_FOUND_COUNT: 3, PICTO_FOUND_WORDS: ['Chat', 'Soleil', 'Vélo'], PICTO_PASS_COUNT: 1, PICTO_PASSES_LEFT: -1, PICTO_ROUND_POINTS: 25,
  PICTO_REVEALED_WORD: '', ...o,
})
const tv = (g, store = createPictoCanvasStore()) => render(<PictoTv gameState={g} catMeta={{ icon: '🐾', label: 'Animaux' }} store={store} />)

describe('PictoTv — Rafale Picto', () => {
  it('grand compteur « 3 mots trouvés » et « ⏭ 1 passe »', () => {
    tv(gs())
    expect(screen.getByTestId('picto-counter').textContent).toContain('3')
    expect(screen.getByTestId('picto-counter').textContent).toContain('mots trouvés')
    expect(screen.getByTestId('picto-counter-pass').textContent).toBe('⏭ 1 passe')
  })
  it('pas de repère de passe tant qu\'aucune passe n\'est utilisée ; pluriel dès 2', () => {
    const { rerender } = tv(gs({ PICTO_PASS_COUNT: 0 }))
    expect(screen.queryByTestId('picto-counter-pass')).toBeNull()
    rerender(<PictoTv gameState={gs({ PICTO_PASS_COUNT: 2 })} catMeta={null} store={createPictoCanvasStore()} />)
    expect(screen.getByTestId('picto-counter-pass').textContent).toBe('⏭ 2 passes')
  })
  it('le type Picto (un mot) n\'a ni compteur ni repère', () => {
    tv(gs({ question: { ID: 'q', TYPE: 'PICTO', CATEGORY: 'ANIMALS' } }))
    expect(screen.queryByTestId('picto-counter')).toBeNull()
  })
  it('« Trouvé » : le mot trouvé s\'affiche 1 s sur le dessin, sans « Trouvé par » ni points', () => {
    const store = createPictoCanvasStore()
    const { rerender } = tv(gs({ PICTO_FOUND_COUNT: 3, PICTO_FOUND_WORDS: ['Chat', 'Soleil', 'Vélo'] }), store)
    expect(screen.queryByTestId('picto-found-flash')).toBeNull()
    rerender(<PictoTv gameState={gs({ PICTO_FOUND_COUNT: 4, PICTO_FOUND_WORDS: ['Chat', 'Soleil', 'Vélo', 'Maison'], PICTO_CANVAS_EPOCH: 3 })} catMeta={null} store={store} />)
    const flash = screen.getByTestId('picto-found-flash')
    expect(flash.textContent).toBe('✅ Maison')
    expect(document.body.textContent).not.toMatch(/Trouvé par|pts/)
    act(() => { vi.advanceTimersByTime(999) })
    expect(screen.queryByTestId('picto-found-flash')).not.toBeNull()
    act(() => { vi.advanceTimersByTime(2) })
    expect(screen.queryByTestId('picto-found-flash')).toBeNull()
  })
  it('passe : repère ⏭ discret, le mot passé n\'est jamais montré', () => {
    const store = createPictoCanvasStore()
    const { rerender } = tv(gs({ PICTO_PASS_COUNT: 1 }), store)
    rerender(<PictoTv gameState={gs({ PICTO_PASS_COUNT: 2, PICTO_CANVAS_EPOCH: 3 })} catMeta={null} store={store} />)
    expect(screen.getByTestId('picto-pass-mark')).toBeTruthy()
    expect(screen.queryByTestId('picto-found-flash')).toBeNull()
    act(() => { vi.advanceTimersByTime(1001) })
    expect(screen.queryByTestId('picto-pass-mark')).toBeNull()
  })
  it('la TV arme la tenue de la feuille (1 s) en Rafale Picto seulement', () => {
    const store = createPictoCanvasStore()
    const spy = vi.spyOn(store, 'armHold')
    tv(gs(), store)
    expect(spy).toHaveBeenCalledWith(1000)
    const store2 = createPictoCanvasStore()
    const spy2 = vi.spyOn(store2, 'armHold')
    tv(gs({ question: { ID: 'q', TYPE: 'PICTO', CATEGORY: 'ANIMALS' } }), store2)
    expect(spy2).toHaveBeenCalledWith(0)
  })
  it('révélation : mot révélé + récapitulatif des mots trouvés (jamais les passés)', () => {
    tv(gs({ phase: 'REVEALED', PICTO_REVEALED_WORD: 'Maison' }))
    expect(screen.getByTestId('picto-revealed-word').textContent).toBe('Maison')
    const rec = screen.getByTestId('picto-tv-found').textContent
    expect(rec).toContain('3 mots trouvés')
    ;['Chat', 'Soleil', 'Vélo'].forEach(w => expect(rec).toContain(w))
    expect(rec).not.toContain('Girafe')
  })
  it('fin de pool : aucun mot révélé, le récapitulatif reste affiché', () => {
    tv(gs({ phase: 'REVEALED', PICTO_REVEALED_WORD: '', PICTO_OUTCOME: 'POOL_END' }))
    expect(screen.queryByTestId('picto-revealed-word')).toBeNull()
    expect(screen.getByTestId('picto-tv-found')).toBeTruthy()
  })
  it('TV statique : aucun overflow auto/scroll dans les règles du compteur', async () => {
    const fs = await import('node:fs')
    const css = fs.readFileSync('src/components/picto/PictoTv.css', 'utf8')
    expect(css).not.toMatch(/overflow\s*:\s*(auto|scroll)/)
  })
})
