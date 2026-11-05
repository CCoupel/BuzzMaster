import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import TeamAwardBar from './TeamAwardBar'

vi.mock('./TeamAwardBar.css', () => ({}))

const bar = (props) => render(
  <TeamAwardBar source="points de la question" penaltyMode={false} sending={false} blockedReason={null}
    hints={{ visible: false }} onSelectAll={() => {}} onSelectNone={() => {}} onGive={() => {}} {...props} />
)
const count = () => document.querySelector('.award-count')

describe('TeamAwardBar — 3 lignes (#289)', () => {
  it('niveau joueur, 2 joueurs de 2 équipes : « 2 joueurs · 2 équipes »', () => {
    bar({ count: 2, unit: 'joueur', teamCount: 2 })
    expect(count().textContent).toBe('2 joueurs · 2 équipes')
  })
  it('niveau joueur, 2 joueurs d\'une équipe : « 2 joueurs · 1 équipe » (singulier)', () => {
    bar({ count: 2, unit: 'joueur', teamCount: 1 })
    expect(count().textContent).toBe('2 joueurs · 1 équipe')
  })
  it('niveau joueur, 1 joueur : « 1 joueur · 1 équipe »', () => {
    bar({ count: 1, unit: 'joueur', teamCount: 1 })
    expect(count().textContent).toBe('1 joueur · 1 équipe')
  })
  it('niveau équipe : « 3 équipes » / « 1 équipe »', () => {
    const { unmount } = bar({ count: 3, unit: 'équipe', teamCount: 3 })
    expect(count().textContent).toBe('3 équipes')
    unmount()
    bar({ count: 1, unit: 'équipe', teamCount: 1 })
    expect(count().textContent).toBe('1 équipe')
  })
  it('aucune sélection : texte dédié, bouton désactivé', () => {
    bar({ count: 0, unit: 'équipe', teamCount: 0 })
    expect(count().textContent).toBe('Aucune sélection')
    expect(screen.getByRole('button', { name: 'Attribuer les points' })).toBeDisabled()
  })
  it('le compteur est un role=status ; le bouton n\'a plus de compteur dans son libellé, sur la ligne 3', () => {
    bar({ count: 2, unit: 'joueur', teamCount: 2 })
    expect(count()).toHaveAttribute('role', 'status')
    const give = screen.getByRole('button', { name: 'Attribuer les points' })
    expect(give.textContent).toBe('Attribuer les points')
    expect(give.parentElement).toHaveClass('award-l3')
  })
  it('pénalité : libellé « Appliquer les pénalités » sans compteur', () => {
    bar({ count: 2, unit: 'joueur', teamCount: 2, penaltyMode: true })
    expect(screen.getByRole('button', { name: 'Appliquer les pénalités' })).toHaveClass('neg')
  })
})
