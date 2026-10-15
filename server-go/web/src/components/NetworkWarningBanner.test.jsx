/**
 * Tests for NetworkWarningBanner — issue #96
 *
 * The component is purely presentational (no hooks). #267 : the hide/show logic moved to
 * NetworkLostOverlay (mounted by App) — tested in NetworkLostOverlay.states.test.jsx.
 */
import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import NetworkWarningBanner, { formatCheckedAgo } from './NetworkWarningBanner'

vi.mock('./NetworkLostOverlay.css', () => ({}))

describe('NetworkWarningBanner — issue #96', () => {
  it('renders its banner element', () => {
    const { container } = render(<NetworkWarningBanner />)
    expect(container.firstChild).not.toBeNull()
  })

  it('sans props : bandeau d\'origine sans « Perte détectée » ni « Détails »', () => {
    render(<NetworkWarningBanner />)
    expect(screen.getByRole('status')).toHaveClass('network-warning-banner')
    expect(screen.queryByText(/Perte détectée/)).toBeNull()
    expect(screen.queryByRole('button', { name: 'Détails' })).toBeNull()
  })

  it('avec checkedAt/now : affiche « Perte détectée il y a N s »', () => {
    render(<NetworkWarningBanner checkedAt={1_000_000} now={1_012_000} />)
    expect(screen.getByText('Perte détectée il y a 12 s')).toBeInTheDocument()
  })

  it('avec onDetails : le bouton « Détails » appelle le rappel', () => {
    const onDetails = vi.fn()
    render(<NetworkWarningBanner onDetails={onDetails} />)
    fireEvent.click(screen.getByRole('button', { name: 'Détails' }))
    expect(onDetails).toHaveBeenCalledTimes(1)
  })
})

describe('formatCheckedAgo (#267)', () => {
  it('secondes, minutes, jamais négatif, inconnu', () => {
    expect(formatCheckedAgo(1000, 13_000)).toBe('il y a 12 s')
    expect(formatCheckedAgo(1000, 1000)).toBe('il y a 0 s')
    expect(formatCheckedAgo(10_000, 1000)).toBe('il y a 0 s') // horloge serveur en avance
    expect(formatCheckedAgo(0, 99_999)).toBeNull()
    expect(formatCheckedAgo(1000, 1000 + 125_000)).toBe('il y a 2 min')
  })
})
