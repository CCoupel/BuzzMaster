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
})
