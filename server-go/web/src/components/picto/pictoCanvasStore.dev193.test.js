import { describe, it, expect, vi } from 'vitest'
import { createPictoCanvasStore } from './pictoCanvasStore'

const frame = (EPOCH, SEQ, ...strokes) => ({ EPOCH, SEQ, STROKES: strokes })
const st = (ID, PTS, END = false) => ({ ID, PTS, END })

describe('pictoCanvasStore — époque, SEQ, rattrapage (contrat §8.4/§8.5)', () => {
  it('une trame d\'une autre époque est ignorée en silence', () => {
    const s = createPictoCanvasStore(); s.setEpoch(2)
    expect(s.applyFrame(frame(1, 1, st(1, [[1, 1]])))).toBe('ignored')
    expect(s.getStrokes()).toHaveLength(0)
  })
  it('les trames contiguës s\'ajoutent ; un trait de même ID est prolongé', () => {
    const s = createPictoCanvasStore(); s.setEpoch(1)
    expect(s.applyFrame(frame(1, 1, st(1, [[0, 0], [10, 10]])))).toBe('ok')
    expect(s.applyFrame(frame(1, 2, st(1, [[20, 20]], true), st(2, [[5, 5]])))).toBe('ok')
    expect(s.getStrokes()).toEqual([st(1, [[0, 0], [10, 10], [20, 20]], true), st(2, [[5, 5]])])
    expect(s.getSeq()).toBe(2)
  })
  it('un trou de SEQ renvoie « gap » une seule fois, puis la feuille entière rattrape et rejoue la suite', () => {
    const s = createPictoCanvasStore(); s.setEpoch(1)
    s.applyFrame(frame(1, 1, st(1, [[0, 0]])))
    expect(s.applyFrame(frame(1, 4, st(2, [[4, 4]])))).toBe('gap')
    expect(s.applyFrame(frame(1, 5, st(2, [[5, 5]])))).toBe('ignored') // en attente de la feuille : pas de nouvelle demande
    s.applyCanvas({ EPOCH: 1, SEQ: 3, STROKES: [st(1, [[0, 0], [1, 1], [2, 2]])] })
    expect(s.getSeq()).toBe(5)
    expect(s.getStrokes().find(x => x.ID === 2).PTS).toEqual([[4, 4], [5, 5]])
  })
  it('un changement d\'époque (UPDATE) efface la feuille', () => {
    const s = createPictoCanvasStore(); s.setEpoch(1)
    s.applyFrame(frame(1, 1, st(1, [[0, 0]])))
    s.setEpoch(2)
    expect(s.getStrokes()).toHaveLength(0)
    expect(s.getSeq()).toBe(0)
  })
  it('au rechargement, la feuille entière précède l\'UPDATE : son époque est adoptée', () => {
    const s = createPictoCanvasStore()
    s.applyCanvas({ EPOCH: 3, SEQ: 7, STROKES: [st(1, [[1, 1]])] })
    s.setEpoch(3)
    expect(s.getStrokes()).toHaveLength(1)
    expect(s.applyFrame(frame(3, 8, st(1, [[2, 2]])))).toBe('ok')
  })
  it('une feuille d\'une autre époque est ignorée', () => {
    const s = createPictoCanvasStore(); s.setEpoch(2)
    expect(s.applyCanvas({ EPOCH: 1, SEQ: 4, STROKES: [st(1, [[1, 1]])] })).toBe('ignored')
  })
  it('les abonnés sont notifiés ; le trait local ne compte pas dans SEQ', () => {
    const s = createPictoCanvasStore(); s.setEpoch(1)
    const fn = vi.fn(); s.subscribe(fn)
    s.addLocal(1, [[3, 3]], false)
    expect(fn).toHaveBeenCalled()
    expect(s.getSeq()).toBe(0)
    expect(s.maxStrokeId()).toBe(1)
  })

  it('clearSheet : feuille vierge hors manche sans changer d\'époque (relance PREPARE/READY)', () => {
    const s = createPictoCanvasStore(); s.setEpoch(2)
    s.applyFrame(frame(2, 1, st(1, [[0, 0]])))
    s.addLocal(2, [[5, 5]], true)
    s.clearSheet()
    expect(s.getStrokes()).toHaveLength(0)
    expect(s.getSeq()).toBe(0)
    expect(s.getEpoch()).toBe(2)
    expect(s.applyFrame(frame(2, 1, st(1, [[9, 9]])))).toBe('ok') // la manche suivante repart de SEQ 1 dans la même époque
  })
})
