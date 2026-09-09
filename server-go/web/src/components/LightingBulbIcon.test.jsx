import { describe, it, expect, vi } from 'vitest'
import { render } from '@testing-library/react'
import LightingBulbIcon from './LightingBulbIcon'
import { lightingStateTitle, lightingStateLabel, normalizeLightingState } from '../utils/lightingState'

// ---------------------------------------------------------------------------
// #207 — ampoule de l'entrée « Ambiance » (contrat hue-bridge.md §7.1,
// maquette §01 rév. 4). Trois glyphes DISTINCTS, pas une forme recolorée :
//   ok                   → ampoule pleine + rayons        (lit)
//   unreachable/refused  → contour + pastille d'alerte    (alert)
//   disabled             → contour nu, AUCUNE pastille    (off)
// ---------------------------------------------------------------------------

vi.mock('./LightingBulbIcon.css', () => ({}))

const renderIcon = (state) => render(<LightingBulbIcon state={state} />).container.querySelector('svg')

describe('LightingBulbIcon — trois glyphes distincts (forme = état)', () => {
  it('ok : ampoule pleine avec rayons', () => {
    const svg = renderIcon('ok')
    expect(svg.dataset.glyph).toBe('lit')
    expect(svg.classList.contains('lighting-bulb-lit')).toBe(true)
    expect(svg.querySelectorAll('.lighting-bulb-rays rect')).toHaveLength(5)
    expect(svg.querySelector('path').getAttribute('fill')).toBe('currentColor')
    expect(svg.querySelector('circle')).toBeNull()
  })

  it('unreachable : contour + pastille d\'alerte', () => {
    const svg = renderIcon('unreachable')
    expect(svg.dataset.glyph).toBe('alert')
    expect(svg.classList.contains('lighting-bulb-alert')).toBe(true)
    expect(svg.querySelector('.lighting-bulb-rays')).toBeNull()
    expect(svg.querySelector('path').getAttribute('fill')).toBe('none')
    expect(svg.querySelector('path').getAttribute('stroke')).toBe('currentColor')
    expect(svg.querySelector('circle.lighting-bulb-alert-dot')).not.toBeNull()
  })

  it('refused : même glyphe d\'alerte qu\'unreachable (même geste : ouvrir la page)', () => {
    const svg = renderIcon('refused')
    expect(svg.dataset.glyph).toBe('alert')
    expect(svg.querySelector('circle.lighting-bulb-alert-dot')).not.toBeNull()
  })

  it('disabled : contour nu — JAMAIS de pastille (ne doit pas ressembler à une alerte)', () => {
    const svg = renderIcon('disabled')
    expect(svg.dataset.glyph).toBe('off')
    expect(svg.classList.contains('lighting-bulb-off')).toBe(true)
    expect(svg.querySelector('.lighting-bulb-rays')).toBeNull()
    expect(svg.querySelector('circle')).toBeNull()
    expect(svg.querySelector('path').getAttribute('fill')).toBe('none')
  })

  it('état inconnu ou absent : traité comme non configuré', () => {
    expect(renderIcon(undefined).dataset.glyph).toBe('off')
    expect(renderIcon('bogus').dataset.glyph).toBe('off')
  })

  it('les trois glyphes sont distincts entre eux (marqueurs DOM différents)', () => {
    const signature = (state) => {
      const svg = renderIcon(state)
      return [svg.dataset.glyph, !!svg.querySelector('.lighting-bulb-rays'), !!svg.querySelector('circle')].join('|')
    }
    const set = new Set([signature('ok'), signature('unreachable'), signature('disabled')])
    expect(set.size).toBe(3)
  })

  it('est aria-hidden et tracé en currentColor (couleur pilotée par CSS)', () => {
    const svg = renderIcon('ok')
    expect(svg.getAttribute('aria-hidden')).toBe('true')
    expect(svg.innerHTML).not.toMatch(/#[0-9a-f]{3,6}/i)
  })
})

describe('lightingState — libellés et normalisation', () => {
  it('title en toutes lettres pour chacun des quatre états', () => {
    expect(lightingStateTitle('ok')).toBe('Éclairage : pont connecté')
    expect(lightingStateTitle('unreachable')).toBe('Éclairage : pont injoignable')
    expect(lightingStateTitle('refused')).toBe('Éclairage : association refusée')
    expect(lightingStateTitle('disabled')).toBe('Éclairage : non configuré')
  })

  it('libellés du badge de page — quatre valeurs, injoignable ≠ refusée', () => {
    expect(lightingStateLabel('ok')).toBe('Pont connecté')
    expect(lightingStateLabel('unreachable')).toBe('Pont injoignable')
    expect(lightingStateLabel('refused')).toBe('Association refusée')
    expect(lightingStateLabel('disabled')).toBe('Non configuré')
    expect(lightingStateLabel('unreachable')).not.toBe(lightingStateLabel('refused'))
  })

  it('normalise toute valeur inconnue vers disabled', () => {
    expect(normalizeLightingState('ok')).toBe('ok')
    expect(normalizeLightingState('')).toBe('disabled')
    expect(normalizeLightingState(null)).toBe('disabled')
    expect(normalizeLightingState('error')).toBe('disabled')
  })
})
