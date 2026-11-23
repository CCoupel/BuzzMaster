// @vitest-environment node
/**
 * #193 (v13.0.0, LOT-2.1) — pictoCanvasStore : feuille côté client (contracts/picto.md §8.4, §8.5).
 * Époque (une trame d'une autre époque est ignorée, un changement d'époque efface), SEQ (un trou demande une
 * resynchronisation `PICTO_CANVAS_SYNC`), rattrapage par la feuille entière `PICTO_CANVAS`.
 */
import { describe, it, expect } from 'vitest'
import { createPictoCanvasStore } from './pictoCanvasStore'

const frame = (epoch, seq, id, pts, end = true) => ({ EPOCH: epoch, SEQ: seq, STROKES: [{ ID: id, PTS: pts, END: end }] })

describe('pictoCanvasStore — époque (§8.4)', () => {
  it('ignore toute trame tant que l\'époque est inconnue', () => {
    const s = createPictoCanvasStore()
    expect(s.applyFrame(frame(1, 1, 1, [[0, 0]]))).toBe('ignored')
    expect(s.getStrokes()).toEqual([])
  })

  it('ignore une trame d\'une autre époque, sans demander de synchronisation', () => {
    const s = createPictoCanvasStore()
    s.setEpoch(3)
    expect(s.applyFrame(frame(2, 1, 1, [[0, 0]]))).toBe('ignored')
    expect(s.applyFrame(frame(4, 1, 1, [[0, 0]]))).toBe('ignored')
    expect(s.getStrokes()).toEqual([])
    expect(s.isSyncing()).toBe(false)
  })

  it('un changement d\'époque efface la feuille et repart à SEQ 0', () => {
    const s = createPictoCanvasStore()
    s.setEpoch(1)
    s.applyFrame(frame(1, 1, 1, [[10, 10], [20, 20]]))
    expect(s.getStrokes()).toHaveLength(1)
    s.setEpoch(2)
    expect(s.getStrokes()).toEqual([])
    expect(s.getSeq()).toBe(0)
    expect(s.applyFrame(frame(2, 1, 1, [[5, 5]]))).toBe('ok')
  })

  it('la même époque redonnée par un UPDATE ne efface rien', () => {
    const s = createPictoCanvasStore()
    s.setEpoch(1)
    s.applyFrame(frame(1, 1, 1, [[10, 10]]))
    s.setEpoch(1)
    expect(s.getStrokes()).toHaveLength(1)
  })

  it('ignore une époque non entière dans un UPDATE', () => {
    const s = createPictoCanvasStore()
    s.setEpoch(1)
    s.setEpoch(undefined)
    s.setEpoch('2')
    expect(s.getEpoch()).toBe(1)
  })
})

describe('pictoCanvasStore — SEQ et rattrapage (§8.5)', () => {
  it('SEQ consécutifs : les trames sont appliquées dans l\'ordre', () => {
    const s = createPictoCanvasStore()
    s.setEpoch(1)
    expect(s.applyFrame(frame(1, 1, 1, [[1, 1]]))).toBe('ok')
    expect(s.applyFrame(frame(1, 2, 2, [[2, 2]]))).toBe('ok')
    expect(s.getSeq()).toBe(2)
    expect(s.getStrokes().map(x => x.ID)).toEqual([1, 2])
  })

  it('un trou de SEQ renvoie « gap » (le hook émet alors PICTO_CANVAS_SYNC) et n\'applique pas la trame', () => {
    const s = createPictoCanvasStore()
    s.setEpoch(1)
    s.applyFrame(frame(1, 1, 1, [[1, 1]]))
    expect(s.applyFrame(frame(1, 3, 2, [[3, 3]]))).toBe('gap')
    expect(s.isSyncing()).toBe(true)
    expect(s.getStrokes()).toHaveLength(1)
  })

  it('une seule demande de synchronisation par trou : les trames suivantes sont mises de côté', () => {
    const s = createPictoCanvasStore()
    s.setEpoch(1)
    s.applyFrame(frame(1, 1, 1, [[1, 1]]))
    expect(s.applyFrame(frame(1, 3, 2, [[3, 3]]))).toBe('gap')
    expect(s.applyFrame(frame(1, 4, 3, [[4, 4]]))).toBe('ignored')
    expect(s.applyFrame(frame(1, 5, 4, [[5, 5]]))).toBe('ignored')
  })

  it('PICTO_CANVAS remplace la feuille, fixe SEQ, et rejoue les trames mises de côté', () => {
    const s = createPictoCanvasStore()
    s.setEpoch(1)
    s.applyFrame(frame(1, 1, 1, [[1, 1]]))
    s.applyFrame(frame(1, 3, 3, [[3, 3]])) // gap
    s.applyFrame(frame(1, 4, 4, [[4, 4]])) // mise de côté
    const r = s.applyCanvas({ EPOCH: 1, SEQ: 3, STROKES: [{ ID: 1, PTS: [[1, 1]], END: true }, { ID: 2, PTS: [[2, 2]], END: true }, { ID: 3, PTS: [[3, 3]], END: true }] })
    expect(r).toBe('ok')
    expect(s.isSyncing()).toBe(false)
    expect(s.getSeq()).toBe(4) // SEQ 4 rejoué après la feuille à SEQ 3
    expect(s.getStrokes().map(x => x.ID)).toEqual([1, 2, 3, 4])
  })

  it('une trame déjà vue (SEQ ≤ dernier) est ignorée', () => {
    const s = createPictoCanvasStore()
    s.setEpoch(1)
    s.applyFrame(frame(1, 1, 1, [[1, 1]]))
    s.applyFrame(frame(1, 2, 2, [[2, 2]]))
    expect(s.applyFrame(frame(1, 2, 2, [[2, 2]]))).toBe('ignored')
    expect(s.applyFrame(frame(1, 1, 1, [[1, 1]]))).toBe('ignored')
    expect(s.getStrokes()).toHaveLength(2)
  })

  it('PICTO_CANVAS d\'une autre époque est ignoré ; au rechargement (époque inconnue) il fixe l\'époque', () => {
    const s = createPictoCanvasStore()
    s.setEpoch(2)
    expect(s.applyCanvas({ EPOCH: 1, SEQ: 5, STROKES: [] })).toBe('ignored')
    const fresh = createPictoCanvasStore() // page rechargée : la feuille précède l'UPDATE
    expect(fresh.applyCanvas({ EPOCH: 7, SEQ: 2, STROKES: [{ ID: 1, PTS: [[1, 1]], END: true }] })).toBe('ok')
    expect(fresh.getEpoch()).toBe(7)
    expect(fresh.getSeq()).toBe(2)
  })
})

describe('pictoCanvasStore — fusion des traits', () => {
  it('une trame qui poursuit un trait ouvert (même ID) ajoute ses points et met END à jour', () => {
    const s = createPictoCanvasStore()
    s.setEpoch(1)
    s.applyFrame(frame(1, 1, 1, [[1, 1], [2, 2]], false))
    s.applyFrame(frame(1, 2, 1, [[3, 3]], true))
    const strokes = s.getStrokes()
    expect(strokes).toHaveLength(1)
    expect(strokes[0].PTS).toEqual([[1, 1], [2, 2], [3, 3]])
    expect(strokes[0].END).toBe(true)
  })

  it('maxStrokeId permet au dessinateur de continuer la numérotation après une resynchronisation', () => {
    const s = createPictoCanvasStore()
    s.setEpoch(1)
    s.applyFrame(frame(1, 1, 4, [[1, 1]]))
    s.applyFrame(frame(1, 2, 9, [[2, 2]]))
    expect(s.maxStrokeId()).toBe(9)
  })

  it('un abonné est notifié à chaque changement visible', () => {
    const s = createPictoCanvasStore()
    let n = 0
    const off = s.subscribe(() => { n += 1 })
    s.setEpoch(1)
    s.applyFrame(frame(1, 1, 1, [[1, 1]]))
    off()
    s.applyFrame(frame(1, 2, 2, [[2, 2]]))
    expect(n).toBe(2)
  })
})
