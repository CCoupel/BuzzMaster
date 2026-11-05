import { describe, it, expect, afterEach } from 'vitest'
import { render, cleanup } from '@testing-library/react'
import RafaleLetterWheel from './RafaleLetterWheel'

// #282 (v11.4.0, LOT-2) — adaptateur contracts/rafale.md §15.8 : 26 segments A–Z, current = lettre
// courante (« # »/« » ⇒ null), lettres absentes de `letters` grisées SAUF la courante, accent = couleur d'équipe.
afterEach(() => cleanup())

const base = { currentLetter: 'C', letters: ['C', 'D', 'M'], accent: '#3b82f6' }

describe('RafaleLetterWheel (adaptateur de Wheel)', () => {
  it('data-testid="rafale-wheel" et layout strip par défaut quand layout est vide/absent', () => {
    const { getByTestId, rerender } = render(<RafaleLetterWheel {...base} layout="" />)
    expect(getByTestId('rafale-wheel').querySelector('.wheel-strip') || getByTestId('rafale-wheel').classList.contains('wheel-strip')).toBeTruthy()
    rerender(<RafaleLetterWheel {...base} />)
    expect(getByTestId('rafale-wheel')).toBeInTheDocument()
  })

  it('layout "arc" : présentation arc', () => {
    const { getByTestId } = render(<RafaleLetterWheel {...base} layout="arc" />)
    const root = getByTestId('rafale-wheel')
    expect(root.querySelector('.wheel-arc') || root.classList.contains('wheel-arc')).toBeTruthy()
  })

  it('26 lettres A–Z rendues', () => {
    const { container } = render(<RafaleLetterWheel {...base} />)
    const text = container.textContent
    for (let i = 0; i < 26; i++) expect(text).toContain(String.fromCharCode(65 + i))
  })

  it('lettres absentes de RAFALE_LETTERS grisées, sauf la lettre courante', () => {
    const { container } = render(<RafaleLetterWheel {...base} currentLetter="A" letters={['B']} />)
    const disabled = Array.from(container.querySelectorAll('.disabled, [data-state="disabled"]')).map(e => e.textContent.trim())
    expect(disabled).toContain('Z')
    expect(disabled).not.toContain('B') // encore à jouer
    expect(disabled).not.toContain('A') // courante : jamais grisée
  })

  it('lettre « # » ou vide : aucune surbrillance (current null)', () => {
    for (const l of ['#', '']) {
      const { container } = render(<RafaleLetterWheel {...base} currentLetter={l} />)
      expect(container.querySelector('.is-current, [data-current="true"]')).toBeNull()
      cleanup()
    }
  })

  it('aria-label annonce la lettre courante', () => {
    const { getByRole } = render(<RafaleLetterWheel {...base} currentLetter="C" />)
    expect(getByRole('img').getAttribute('aria-label')).toMatch(/Lettre courante\s*:\s*C/)
  })
})
