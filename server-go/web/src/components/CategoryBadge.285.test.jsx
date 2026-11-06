import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import CategoryBadge from './CategoryBadge'

vi.mock('./CategoryBadge.css', () => ({}))

// #285 — CategoryBadge suit la résolution API-first : couleur custom, intégrée recolorée, aucune couleur ⇒ gris.
const API = [
  { key: 'CINEMA', name: 'Cinéma', imageURL: '/files/categories/CINEMA.png', color: '#0ea5e9', isCustom: true },
  { key: 'NUE', name: 'Nue', imageURL: '/files/categories/NUE.png', color: '', isCustom: true },
  { key: 'GEOGRAPHY', name: 'Géo', imageURL: '', color: '#ff0000', isCustom: false },
]

describe('CategoryBadge (#285)', () => {
  it('personnalisée colorée : fond = sa couleur, image de l\'icône', () => {
    render(<CategoryBadge catKey="CINEMA" customCategories={API} />)
    const badge = screen.getByTitle('Cinéma')
    expect(badge).toHaveStyle({ backgroundColor: '#0ea5e9' })
    expect(screen.getByAltText('Cinéma')).toHaveAttribute('src', '/files/categories/CINEMA.png')
  })

  it('personnalisée sans couleur : fond gris #6b7280', () => {
    render(<CategoryBadge catKey="NUE" customCategories={API} />)
    expect(screen.getByTitle('Nue')).toHaveStyle({ backgroundColor: '#6b7280' })
  })

  it('intégrée recolorée/renommée : fond de l\'API, titre de l\'API, emoji conservé', () => {
    render(<CategoryBadge catKey="GEOGRAPHY" customCategories={API} />)
    const badge = screen.getByTitle('Géo')
    expect(badge).toHaveStyle({ backgroundColor: '#ff0000' })
    expect(badge.textContent).toContain('🌍')
  })
})
