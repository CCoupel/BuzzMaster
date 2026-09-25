import { describe, it, expect, vi } from 'vitest'
import { render } from '@testing-library/react'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, resolve } from 'node:path'
import BrandLogo from './BrandLogo'

// ---------------------------------------------------------------------------
// #239 — logo A1 (mot-symbole « Buzz / Control ⚡ ») remplaçant l'abeille du
// bouton de menu de la Navbar. Plan : _work/handoff/plan-239-v7-20260925-160000.md
// (partie A). Couvre AC1, AC4, AC5, AC6, AC9.
// ---------------------------------------------------------------------------

vi.mock('./BrandLogo.css', () => ({}))

const here = dirname(fileURLToPath(import.meta.url))
const readSrc = (rel) => readFileSync(resolve(here, rel), 'utf8')

describe('BrandLogo — structure (#239 AC1, AC9)', () => {
  it('rend « Buzz » et « Control » dans deux éléments distincts', () => {
    const { container } = render(<BrandLogo />)
    const buzz = container.querySelector('.brand-logo-buzz')
    const control = container.querySelector('.brand-logo-control')
    expect(buzz).not.toBeNull()
    expect(control).not.toBeNull()
    expect(buzz).not.toBe(control)
    expect(buzz.textContent).toBe('Buzz')
    expect(control.textContent).toBe('Control')
  })

  it('« Buzz » précède « Control » dans l\'ordre du DOM (Buzz au-dessus)', () => {
    const { container } = render(<BrandLogo />)
    const buzz = container.querySelector('.brand-logo-buzz')
    const control = container.querySelector('.brand-logo-control')
    expect(buzz.compareDocumentPosition(control) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  })

  it('rend le ⚡ dans .brand-logo-bolt', () => {
    const { container } = render(<BrandLogo />)
    expect(container.querySelector('.brand-logo-bolt').textContent).toBe('⚡')
  })

  it('le conteneur .brand-logo-wordmark est aria-hidden="true"', () => {
    const { container } = render(<BrandLogo />)
    const wordmark = container.querySelector('.brand-logo-wordmark')
    expect(wordmark).not.toBeNull()
    expect(wordmark.getAttribute('aria-hidden')).toBe('true')
    // Buzz / Control / ⚡ sont tous à l'intérieur du conteneur masqué
    expect(wordmark.querySelector('.brand-logo-buzz')).not.toBeNull()
    expect(wordmark.querySelector('.brand-logo-control')).not.toBeNull()
    expect(wordmark.querySelector('.brand-logo-bolt')).not.toBeNull()
  })

  it('ne rend plus d\'abeille 🐝', () => {
    const { container } = render(<BrandLogo />)
    expect(container.textContent).not.toContain('🐝')
  })
})

describe('BrandLogo — styles (#239 AC4, AC5, AC6)', () => {
  it('AC4 — tokens de marque définis dans index.css via les variables existantes', () => {
    const css = readSrc('../styles/index.css')
    expect(css).toMatch(/--brand-logo-buzz:\s*var\(--primary-800\)/)
    expect(css).toMatch(/--brand-logo-control:\s*var\(--accent-pink\)/)
  })

  it('AC4 — BrandLogo.css utilise les tokens, sans hex en dur', () => {
    const css = readSrc('./BrandLogo.css')
    expect(css).toMatch(/var\(--brand-logo-buzz\)/)
    expect(css).toMatch(/var\(--brand-logo-control\)/)
    expect(css).not.toMatch(/#[0-9a-fA-F]{3,8}\b/)
  })

  it('AC5 — Fredoka via --font-display (index.css), sans ressource réseau dans BrandLogo.css', () => {
    const css = readSrc('./BrandLogo.css')
    expect(css).toMatch(/font-family:\s*var\(--font-display\)/)
    expect(readSrc('../styles/index.css')).toMatch(/--font-display:\s*'Fredoka'/)
    expect(css).not.toMatch(/https?:\/\//)
    expect(css).not.toMatch(/@import\s+url/)
  })

  it('maquette — text-shadow uniquement sur .brand-logo-control, pas sur .brand-logo-buzz', () => {
    const css = readSrc('./BrandLogo.css')
    const block = (sel) => {
      const m = css.match(new RegExp(sel.replace('.', '\\.') + '\\s*\\{([^}]*)\\}'))
      return m ? m[1] : ''
    }
    expect(block('.brand-logo-buzz')).not.toMatch(/text-shadow/)
    expect(block('.brand-logo-control')).toMatch(/text-shadow/)
  })

  it('AC6 — taille pilotée par --brand-logo-size (défaut 1.5rem)', () => {
    const css = readSrc('./BrandLogo.css')
    expect(css).toMatch(/var\(--brand-logo-size,\s*1\.5rem\)/)
  })

  it('AC6 — Navbar.css réduit --brand-logo-size à 1.25rem sous 768px', () => {
    const css = readSrc('./Navbar.css')
    const media = css.slice(css.indexOf('@media (max-width: 768px)') >= 0
      ? css.indexOf('@media (max-width: 768px)')
      : css.indexOf('@media (max-width:768px)'))
    expect(media).toMatch(/\.brand-logo-button\s*\{[^}]*--brand-logo-size:\s*1\.25rem/)
  })
})
