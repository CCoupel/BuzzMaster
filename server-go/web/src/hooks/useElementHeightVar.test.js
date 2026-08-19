import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import useElementHeightVar from './useElementHeightVar'

// ---------------------------------------------------------------------------
// useElementHeightVar — mesure réelle de la hauteur d'un élément, propagée
// via une variable CSS sur document.documentElement (v6.4.x, #179 F1).
//
// Plan : _work/reports/plan-20260818-212304.md. Extraction PURE (aucun
// changement de comportement) de la mécanique déjà en place dans
// RegieMessageBar.jsx depuis #177 — voir RegieMessageBar.test.jsx pour la
// garantie de non-régression (AC9, T3 : ce fichier reste vert SANS
// modification après l'extraction, garde-fou dédié).
//
// Signature : useElementHeightVar(ref, varName) — ref = un objet
// `{ current: HTMLElement }` (pas nécessairement un vrai ref React, le hook
// ne lit que `.current`) ; varName = le nom de la variable CSS à écrire
// (ex. '--regie-bar-h', '--navbar-h').
//
// ⚠️ Ce fichier vérifie UNIQUEMENT le MÉCANISME (la variable posée/mise à
// jour/nettoyée) — jamais le RÉSULTAT visuel (absence de scrollbar) :
// jsdom ne fait aucun calcul de mise en page réel (`getBoundingClientRect()`
// y renvoie toujours une hauteur nulle). Le résultat relève de la recette
// visuelle (tests/procedures/), exécutée par l'utilisateur.
//
// ResizeObserver n'existe pas nativement en jsdom — mocké ici, capturant le
// callback passé au constructeur pour le déclencher manuellement. Entrée au
// format `{ contentRect: { height } }` (forme standard de l'API) : le plan
// prévoit explicitement `borderBoxSize?.[0]?.blockSize ?? contentRect.height`
// (patron copié de RegieMessageBar.jsx), donc l'absence de borderBoxSize
// dans le mock retombe correctement sur contentRect.height.
// ---------------------------------------------------------------------------

class ResizeObserverMock {
  constructor(callback) {
    this.callback = callback
    this.observed = []
    ResizeObserverMock.instances.push(this)
  }
  observe(target) { this.observed.push(target) }
  unobserve() {}
  disconnect() { this.disconnected = true }
  fire(height) {
    this.callback([{ contentRect: { height } }], this)
  }
}
ResizeObserverMock.instances = []

const VAR_NAME = '--test-element-h'

function makeElementRef() {
  const el = document.createElement('div')
  document.body.appendChild(el)
  return { current: el }
}

beforeEach(() => {
  ResizeObserverMock.instances = []
  global.ResizeObserver = ResizeObserverMock
  document.documentElement.style.removeProperty(VAR_NAME)
})

afterEach(() => {
  document.documentElement.style.removeProperty(VAR_NAME)
  document.body.innerHTML = ''
})

describe('useElementHeightVar — observation au montage', () => {
  it('observe la cible désignée par ref.current', () => {
    const ref = makeElementRef()
    renderHook(() => useElementHeightVar(ref, VAR_NAME))

    expect(ResizeObserverMock.instances).toHaveLength(1)
    expect(ResizeObserverMock.instances[0].observed).toEqual([ref.current])
  })

  it('ne plante pas si ref.current est null au montage (élément pas encore rendu)', () => {
    const ref = { current: null }
    expect(() => renderHook(() => useElementHeightVar(ref, VAR_NAME))).not.toThrow()
  })
})

describe('useElementHeightVar — pose la variable CSS (AC2)', () => {
  it('écrit varName sur document.documentElement quand une mesure arrive', () => {
    const ref = makeElementRef()
    renderHook(() => useElementHeightVar(ref, VAR_NAME))
    const observer = ResizeObserverMock.instances[0]

    act(() => { observer.fire(62) })

    expect(document.documentElement.style.getPropertyValue(VAR_NAME)).toBe('62px')
  })

  it('met à jour la variable quand ResizeObserver rapporte une nouvelle hauteur (AC3)', () => {
    const ref = makeElementRef()
    renderHook(() => useElementHeightVar(ref, VAR_NAME))
    const observer = ResizeObserverMock.instances[0]

    act(() => { observer.fire(72) })
    expect(document.documentElement.style.getPropertyValue(VAR_NAME)).toBe('72px')

    act(() => { observer.fire(96) }) // ex. franchissement d'un point de rupture responsive
    expect(document.documentElement.style.getPropertyValue(VAR_NAME)).toBe('96px')
  })

  it('arrondit la hauteur mesurée', () => {
    const ref = makeElementRef()
    renderHook(() => useElementHeightVar(ref, VAR_NAME))
    const observer = ResizeObserverMock.instances[0]

    act(() => { observer.fire(71.6) })

    expect(document.documentElement.style.getPropertyValue(VAR_NAME)).toBe('72px')
  })

  it('lit borderBoxSize[0].blockSize quand présent, prioritaire sur contentRect.height', () => {
    const ref = makeElementRef()
    renderHook(() => useElementHeightVar(ref, VAR_NAME))
    const observer = ResizeObserverMock.instances[0]

    act(() => {
      observer.callback(
        [{ contentRect: { height: 999 }, borderBoxSize: [{ blockSize: 80 }] }],
        observer
      )
    })

    expect(document.documentElement.style.getPropertyValue(VAR_NAME)).toBe('80px')
  })
})

describe("useElementHeightVar — n'écrit pas si la valeur arrondie est inchangée", () => {
  it('deux mesures qui arrondissent à la même valeur ne réécrivent la variable qu\'une fois', () => {
    const ref = makeElementRef()
    renderHook(() => useElementHeightVar(ref, VAR_NAME))
    const observer = ResizeObserverMock.instances[0]
    const setPropertySpy = vi.spyOn(document.documentElement.style, 'setProperty')

    act(() => { observer.fire(44.4) }) // arrondit à 44
    act(() => { observer.fire(44.2) }) // arrondit aussi à 44 — pas de nouvelle écriture attendue

    const calls = setPropertySpy.mock.calls.filter(call => call[0] === VAR_NAME)
    expect(calls).toHaveLength(1)
    expect(document.documentElement.style.getPropertyValue(VAR_NAME)).toBe('44px')

    setPropertySpy.mockRestore()
  })
})

describe('useElementHeightVar — nettoyage au démontage', () => {
  it('remet la variable à 0px au démontage', () => {
    const ref = makeElementRef()
    const { unmount } = renderHook(() => useElementHeightVar(ref, VAR_NAME))
    const observer = ResizeObserverMock.instances[0]
    act(() => { observer.fire(62) })
    expect(document.documentElement.style.getPropertyValue(VAR_NAME)).toBe('62px')

    unmount()

    expect(document.documentElement.style.getPropertyValue(VAR_NAME)).toBe('0px')
  })

  it('déconnecte le ResizeObserver au démontage (pas de fuite de callback)', () => {
    const ref = makeElementRef()
    const { unmount } = renderHook(() => useElementHeightVar(ref, VAR_NAME))
    const observer = ResizeObserverMock.instances[0]

    unmount()

    expect(observer.disconnected).toBe(true)
  })
})

describe('useElementHeightVar — plusieurs consommateurs indépendants', () => {
  it('deux instances avec des varName différents écrivent chacune leur propre variable, sans interférence', () => {
    const refA = makeElementRef()
    const refB = makeElementRef()
    renderHook(() => useElementHeightVar(refA, '--var-a'))
    renderHook(() => useElementHeightVar(refB, '--var-b'))

    act(() => { ResizeObserverMock.instances[0].fire(40) })
    act(() => { ResizeObserverMock.instances[1].fire(90) })

    expect(document.documentElement.style.getPropertyValue('--var-a')).toBe('40px')
    expect(document.documentElement.style.getPropertyValue('--var-b')).toBe('90px')

    document.documentElement.style.removeProperty('--var-a')
    document.documentElement.style.removeProperty('--var-b')
  })
})
