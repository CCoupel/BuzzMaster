import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import AnimTeamCard from './AnimTeamCard'

// ---------------------------------------------------------------------------
// AnimTeamCard — carte d'équipe de base, page animateur (#155, F4)
// ---------------------------------------------------------------------------

describe('AnimTeamCard', () => {
  it('affiche le nom et le score', () => {
    render(<AnimTeamCard name="Les Rouges" color={[255, 0, 0]} score={12} />)
    expect(screen.getByText('Les Rouges')).toBeInTheDocument()
    expect(screen.getByText('12')).toBeInTheDocument()
  })

  it('affiche 0 par défaut quand aucun score n\'est fourni', () => {
    render(<AnimTeamCard name="Les Verts" color={[0, 255, 0]} />)
    expect(screen.getByText('0')).toBeInTheDocument()
  })

  it('ne rend pas de bloc d\'extension quand children est absent (point d\'extension F6)', () => {
    const { container } = render(<AnimTeamCard name="Les Bleus" color={[0, 0, 255]} score={3} />)
    expect(container.querySelector('.anim-team-card-extra')).toBeNull()
  })

  it('rend le contenu additionnel dans le bloc d\'extension quand children est fourni', () => {
    render(
      <AnimTeamCard name="Les Jaunes" color={[255, 255, 0]} score={7}>
        <span>Rang 1</span>
      </AnimTeamCard>
    )
    expect(screen.getByText('Rang 1')).toBeInTheDocument()
  })
})
