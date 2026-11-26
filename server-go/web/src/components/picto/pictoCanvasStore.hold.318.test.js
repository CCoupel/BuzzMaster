import { describe, it, expect, vi, afterEach } from 'vitest'
import { createPictoCanvasStore } from './pictoCanvasStore'

// #318 (v13.1.0, LOT-3 T24) — tenue de l'ancienne feuille 1 s à « Trouvé » (picto.md §17.5, §17.12) : aucun trait d'un
// mot précédent sur la feuille suivante, y compris pendant la seconde d'affichage du mot trouvé sur la TV.
afterEach(() => { vi.useRealTimers() })

const frame = (epoch, seq, id, x) => ({ EPOCH: epoch, SEQ: seq, STROKES: [{ ID: id, PTS: [[x, x]], END: true }] })
const setup = () => {
  const s = createPictoCanvasStore()
  s.setEpoch(1)
  s.applyFrame(frame(1, 1, 1, 100))
  return s
}

describe('pictoCanvasStore — tenue (armHold)', () => {
  it('désarmée par défaut : le changement d\'époque efface aussitôt (comportement v13.0.0 inchangé)', () => {
    const s = setup()
    s.setEpoch(2)
    expect(s.getStrokes()).toEqual([])
    expect(s.isHolding()).toBe(false)
  })

  it('armée : l\'ancienne feuille reste visible, les traits de la nouvelle époque s\'accumulent derrière', () => {
    vi.useFakeTimers()
    const s = setup()
    s.armHold(1000)
    s.setEpoch(2)
    expect(s.isHolding()).toBe(true)
    expect(s.getStrokes()).toHaveLength(1)
    expect(s.getStrokes()[0].ID).toBe(1) // ancien trait (époque 1)
    s.applyFrame(frame(2, 1, 7, 200)) // trait reçu pendant la seconde
    expect(s.getStrokes()[0].ID).toBe(1) // toujours l'ancienne feuille à l'écran
  })

  it('après 1 s : la feuille du mot suivant s\'affiche avec les traits déjà reçus, sans trait du mot précédent', () => {
    vi.useFakeTimers()
    const s = setup()
    s.armHold(1000)
    s.setEpoch(2)
    s.applyFrame(frame(2, 1, 7, 200))
    vi.advanceTimersByTime(1000)
    expect(s.isHolding()).toBe(false)
    expect(s.getStrokes().map(t => t.ID)).toEqual([7])
  })

  it('releaseHold (passe) : la feuille se vide tout de suite', () => {
    vi.useFakeTimers()
    const s = setup()
    s.armHold(1000)
    s.setEpoch(2)
    s.releaseHold()
    expect(s.isHolding()).toBe(false)
    expect(s.getStrokes()).toEqual([])
  })

  it('un trait de l\'ancienne époque reçu pendant la tenue est ignoré', () => {
    vi.useFakeTimers()
    const s = setup()
    s.armHold(1000)
    s.setEpoch(2)
    expect(s.applyFrame(frame(1, 2, 9, 300))).toBe('ignored')
  })

  it('feuille vierge (feuille vide à l\'époque précédente) : rien à tenir', () => {
    const s = createPictoCanvasStore()
    s.setEpoch(1)
    s.armHold(1000)
    s.setEpoch(2)
    expect(s.isHolding()).toBe(false)
  })

  it('clearSheet et reset lèvent la tenue', () => {
    vi.useFakeTimers()
    const s = setup()
    s.armHold(1000)
    s.setEpoch(2)
    s.clearSheet()
    expect(s.isHolding()).toBe(false)
    const s2 = setup()
    s2.armHold(1000); s2.setEpoch(2); s2.reset()
    expect(s2.isHolding()).toBe(false)
  })

  it('désarmer (armHold(0)) lève la tenue en cours', () => {
    vi.useFakeTimers()
    const s = setup()
    s.armHold(1000)
    s.setEpoch(2)
    s.armHold(0)
    expect(s.isHolding()).toBe(false)
  })
})
