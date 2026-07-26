import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import ConnectionBadge from './ConnectionBadge'

// ---------------------------------------------------------------------------
// Tests : ConnectionBadge — composant unique du badge de connexion 4 états
// (#109 — remplace les 3 implémentations SVG dupliquées de TeamCard/TeamsPage)
//
// Contrat (voir contracts/models.md à venir + plan
// _work/reports/planner-20260725-105503-final.md §2) : `state` reçoit
// directement `Bumper.CONN_STATE` : "" | "orange" | "red" | "green".
// ---------------------------------------------------------------------------

vi.mock('./ConnectionBadge.css', () => ({}))

describe('ConnectionBadge — état HIDDEN ("")', () => {
  it('ne rend rien quand state=""', () => {
    const { container } = render(<ConnectionBadge state="" />)
    expect(container).toBeEmptyDOMElement()
  })

  it('ne rend rien quand state est undefined', () => {
    const { container } = render(<ConnectionBadge />)
    expect(container).toBeEmptyDOMElement()
  })

  it('ne rend rien pour une valeur inconnue (défensif, futur firmware)', () => {
    const { container } = render(<ConnectionBadge state="unknown-future-state" />)
    expect(container).toBeEmptyDOMElement()
  })
})

describe('ConnectionBadge — état ORANGE (déconnecté)', () => {
  it('rend un badge avec la classe connection-badge-orange', () => {
    const { container } = render(<ConnectionBadge state="orange" />)
    const badge = container.querySelector('.connection-badge')
    expect(badge).not.toBeNull()
    expect(badge.className).toContain('connection-badge-orange')
  })

  it('a pour titre "Déconnecté"', () => {
    render(<ConnectionBadge state="orange" />)
    expect(screen.getByTitle('Déconnecté')).toBeInTheDocument()
  })

  it('contient une icône SVG', () => {
    const { container } = render(<ConnectionBadge state="orange" />)
    expect(container.querySelector('svg')).not.toBeNull()
  })
})

describe('ConnectionBadge — état RED (déconnecté, message perdu)', () => {
  it('rend un badge avec la classe connection-badge-red', () => {
    const { container } = render(<ConnectionBadge state="red" />)
    const badge = container.querySelector('.connection-badge')
    expect(badge.className).toContain('connection-badge-red')
  })

  it('a pour titre "Déconnecté — message(s) perdu(s)"', () => {
    render(<ConnectionBadge state="red" />)
    expect(screen.getByTitle('Déconnecté — message(s) perdu(s)')).toBeInTheDocument()
  })
})

describe('ConnectionBadge — état GREEN (reconnecté, fenêtre de grâce)', () => {
  it('rend un badge avec la classe connection-badge-green', () => {
    const { container } = render(<ConnectionBadge state="green" />)
    const badge = container.querySelector('.connection-badge')
    expect(badge.className).toContain('connection-badge-green')
  })

  it('a pour titre "Reconnecté"', () => {
    render(<ConnectionBadge state="green" />)
    expect(screen.getByTitle('Reconnecté')).toBeInTheDocument()
  })

  it('utilise une icône distincte (check) de celle des états orange/red', () => {
    const { container: greenContainer } = render(<ConnectionBadge state="green" />)
    const { container: orangeContainer } = render(<ConnectionBadge state="orange" />)
    const greenPath = greenContainer.querySelector('svg').innerHTML
    const orangePath = orangeContainer.querySelector('svg').innerHTML
    expect(greenPath).not.toEqual(orangePath)
  })
})

describe('ConnectionBadge — className additionnelle', () => {
  it('ajoute la classe fournie en props en plus des classes par défaut', () => {
    const { container } = render(<ConnectionBadge state="orange" className="extra-class" />)
    const badge = container.querySelector('.connection-badge')
    expect(badge.className).toContain('extra-class')
    expect(badge.className).toContain('connection-badge-orange')
  })
})

describe('ConnectionBadge — cohérence des 4 états (aucun état ne partage un titre)', () => {
  it('les 3 états visibles ont chacun un titre distinct', () => {
    const titles = ['orange', 'red', 'green'].map(state => {
      const { container, unmount } = render(<ConnectionBadge state={state} />)
      const title = container.querySelector('.connection-badge').getAttribute('title')
      unmount()
      return title
    })
    expect(new Set(titles).size).toBe(3)
  })
})
