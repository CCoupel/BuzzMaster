import { describe, it, expect, vi } from 'vitest'
import { render } from '@testing-library/react'
import SoundSpeakerIcon from './SoundSpeakerIcon'
import { soundStateGlyph, soundStateLabel, soundStateTitle, normalizeSoundActive } from '../utils/soundState'

// ---------------------------------------------------------------------------
// #234 — haut-parleur de l'entrée « Ambiance » (même patron que
// LightingBulbIcon.test.jsx, #207). DEUX glyphes DISTINCTS, jamais une
// troisième forme d'alerte :
//   actif   → haut-parleur + ondes   (on)
//   inactif → haut-parleur nu, AUCUNE barre oblique (off)
// ---------------------------------------------------------------------------

vi.mock('./SoundSpeakerIcon.css', () => ({}))

const renderIcon = (active) => render(<SoundSpeakerIcon active={active} />).container.querySelector('svg')

describe('SoundSpeakerIcon — deux glyphes distincts (forme = état)', () => {
  it('actif : haut-parleur avec ondes', () => {
    const svg = renderIcon(true)
    expect(svg.dataset.glyph).toBe('on')
    expect(svg.classList.contains('sound-speaker-on')).toBe(true)
    expect(svg.querySelector('.sound-speaker-waves')).not.toBeNull()
    expect(svg.querySelectorAll('.sound-speaker-waves path')).toHaveLength(2)
    expect(svg.querySelector('path[fill="currentColor"]')).not.toBeNull()
  })

  it('inactif : haut-parleur nu — JAMAIS de barre oblique (un choix normal, pas une panne)', () => {
    const svg = renderIcon(false)
    expect(svg.dataset.glyph).toBe('off')
    expect(svg.classList.contains('sound-speaker-off')).toBe(true)
    expect(svg.querySelector('.sound-speaker-waves')).toBeNull()
    // Aucune ligne diagonale (barre "mute") : pas de <line>, pas de path
    // supplémentaire au-delà du corps du haut-parleur.
    expect(svg.querySelectorAll('path')).toHaveLength(1)
    expect(svg.querySelector('line')).toBeNull()
  })

  it('valeur absente ou fausse-y : traitée comme inactif', () => {
    expect(renderIcon(undefined).dataset.glyph).toBe('off')
    expect(renderIcon(null).dataset.glyph).toBe('off')
    expect(renderIcon(0).dataset.glyph).toBe('off')
  })

  it('les deux glyphes sont distincts entre eux (marqueurs DOM différents)', () => {
    const signature = (active) => {
      const svg = renderIcon(active)
      return [svg.dataset.glyph, !!svg.querySelector('.sound-speaker-waves')].join('|')
    }
    const set = new Set([signature(true), signature(false)])
    expect(set.size).toBe(2)
  })

  it('est aria-hidden et tracé en currentColor (couleur pilotée par CSS)', () => {
    const svg = renderIcon(true)
    expect(svg.getAttribute('aria-hidden')).toBe('true')
    expect(svg.innerHTML).not.toMatch(/#[0-9a-f]{3,6}/i)
  })
})

describe('soundState — libellés, title et glyphe', () => {
  it('soundStateGlyph : on/off uniquement', () => {
    expect(soundStateGlyph(true)).toBe('on')
    expect(soundStateGlyph(false)).toBe('off')
    expect(soundStateGlyph(undefined)).toBe('off')
  })

  it('soundStateLabel : Son actif / Son inactif', () => {
    expect(soundStateLabel(true)).toBe('Son actif')
    expect(soundStateLabel(false)).toBe('Son inactif')
  })

  it('soundStateTitle : en toutes lettres, même patron que lightingStateTitle', () => {
    expect(soundStateTitle(true)).toBe('Son : actif')
    expect(soundStateTitle(false)).toBe('Son : inactif')
  })

  it('normalizeSoundActive : seule la valeur true est active', () => {
    expect(normalizeSoundActive(true)).toBe(true)
    expect(normalizeSoundActive(false)).toBe(false)
    expect(normalizeSoundActive(undefined)).toBe(false)
    expect(normalizeSoundActive('true')).toBe(false)
  })
})
