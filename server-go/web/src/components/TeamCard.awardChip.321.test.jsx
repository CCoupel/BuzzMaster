import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import TeamCard from './TeamCard'

// #321 — chip de gain prévu (`awardChip`) : même emplacement que le chip MEMORY/« PRET », REVEALED seulement.
vi.mock('./TeamCard.css', () => ({}))

const r = (o = {}) => render(<TeamCard name="Bleus" color={[0, 0, 255]} score={0} buzzers={[]} gamePhase="REVEALED" {...o} />)

describe('TeamCard — awardChip (#321)', () => {
  it('affiche « +N pts » en REVEALED', () => {
    r({ awardChip: 3 })
    const chip = screen.getByText('+3 pts')
    expect(chip.className).toContain('memory-acquired-badge')
    expect(chip.className).not.toContain('award-chip-neg')
  })
  it('pénalité : « −2 pts » en classe award-chip-neg', () => {
    r({ awardChip: -2 })
    expect(screen.getByText('-2 pts').className).toContain('award-chip-neg')
  })
  it('null : aucun chip (PRET conservé si prêt)', () => {
    r({ awardChip: null, ready: true })
    expect(screen.queryByText(/pts$/)).toBeNull()
  })
  it('hors REVEALED : aucun chip', () => {
    r({ awardChip: 3, gamePhase: 'STOPPED' })
    expect(screen.queryByText('+3 pts')).toBeNull()
  })
  it('un chip dédié (MEMORY) garde la priorité', () => {
    r({ awardChip: 9, memoryStats: { pairs: 2, errors: 0, totalPairs: 4, pointsPerPair: 10 } })
    expect(screen.getByText('+20 pts')).toBeTruthy()
    expect(screen.queryByText('+9 pts')).toBeNull()
  })
  it('chip affiché : le petit award-preview « +N » redondant n’est pas rendu', () => {
    const { container } = r({ awardChip: 3, selectable: true, selected: true, awardPreview: 3 })
    expect(container.querySelectorAll('.award-chip').length).toBe(1)
    expect(container.querySelector('.award-preview')).toBeNull()
  })
  it('chip dédié (MEMORY) à sa place : award-preview conservé', () => {
    const { container } = r({ awardChip: 20, selectable: true, selected: true, awardPreview: 20, memoryStats: { pairs: 2, errors: 0, totalPairs: 4, pointsPerPair: 10 } })
    expect(container.querySelector('.award-preview')).not.toBeNull()
  })
  it('niveau JOUEUR : le montant reste sur la ligne du joueur (pas de chip d’équipe)', () => {
    const { container } = r({
      awardChip: null, selectable: false, buzzers: [{ mac: 'm1', name: 'Paul', score: 0 }],
      playerAward: { selected: () => true, onChange: () => {}, awarded: () => null, preview: () => 3 },
    })
    expect(container.querySelectorAll('.award-chip').length).toBe(0)
    expect(container.querySelector('.buzzer-mini .award-preview')).not.toBeNull()
  })
})
