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

// ---------------------------------------------------------------------------
// #171/F5, T6 — prop `medal` dédiée (distincte de `children`), rendue dans
// l'en-tête AVANT le nom. C'est le test qui aurait attrapé le défaut
// d'origine : avant #171, le rang (🏆) vivait dans `.anim-team-buzz-info`
// (à l'intérieur de `children`, donc APRÈS le nom, jamais avant) et sa
// présence dépendait de celle d'un voisin (`justify-content: space-between`
// dans `.anim-team-card-extra`) — deux défauts que la prop dédiée corrige
// structurellement : impossible d'insérer AVANT le nom via `children`, donc
// une prop séparée était la seule façon de le faire correctement.
// ---------------------------------------------------------------------------

describe('AnimTeamCard — médaille dédiée, avant le nom (#171/F5, T6)', () => {
  it('ne rend rien pour la médaille quand `medal` est absente', () => {
    const { container } = render(<AnimTeamCard name="Les Rouges" color={[255, 0, 0]} score={0} />)
    // Aucune régression : le nom reste bien rendu sans médaille.
    expect(screen.getByText('Les Rouges')).toBeInTheDocument()
    expect(container.querySelector('.anim-team-card-medal')).toBeNull()
  })

  it('rend la médaille AVANT le nom dans le DOM (pas via children, qui ne peut pas s\'insérer avant)', () => {
    const { container } = render(<AnimTeamCard name="Les Rouges" color={[255, 0, 0]} score={12} medal="🏆" />)
    const header = container.querySelector('.anim-team-card-header')
    const medalEl = header.querySelector('.anim-team-card-medal')
    const nameEl = header.querySelector('.anim-team-card-name')
    expect(medalEl).not.toBeNull()
    expect(medalEl.textContent).toBe('🏆')
    // compareDocumentPosition : DOCUMENT_POSITION_FOLLOWING (4) si nameEl
    // suit medalEl dans le document — c'est l'assertion structurelle "avant
    // le nom", indépendante de tout ordre de props ou de mise en page CSS.
    // eslint-disable-next-line no-bitwise
    expect(medalEl.compareDocumentPosition(nameEl) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  })

  it('la médaille et le score coexistent (score reste ancré à droite, indépendamment de la médaille)', () => {
    render(<AnimTeamCard name="Les Bleus" color={[0, 0, 255]} score={9} medal="🥈" />)
    expect(screen.getByText('🥈')).toBeInTheDocument()
    expect(screen.getByText('Les Bleus')).toBeInTheDocument()
    expect(screen.getByText('9')).toBeInTheDocument()
  })

  it('médaille ET children (crédit, temps de réaction) coexistent sans conflit', () => {
    render(
      <AnimTeamCard name="Les Verts" color={[0, 255, 0]} score={5} medal="🥉">
        <span>1.234s</span>
      </AnimTeamCard>
    )
    expect(screen.getByText('🥉')).toBeInTheDocument()
    expect(screen.getByText('1.234s')).toBeInTheDocument()
  })
})

// ---------------------------------------------------------------------------
// #159/F4 — props `active`/`dimmed` (opt-in, MEMORY_CURRENT_TEAM /
// MEMORY_PARTICIPATING_TEAMS côté AnimPage.jsx). Ni l'une ni l'autre ne
// change ce qui est affiché (nom/score/médaille/children INCHANGÉS) —
// seulement la classe posée sur la carte, un contour CSS pur (AnimTeamCard.css).
// ---------------------------------------------------------------------------

describe('AnimTeamCard — active/dimmed (#159/F4)', () => {
  it('ni active ni dimmed par défaut (appelants existants QCM/SPEEDY/ARDOISE, aucun changement)', () => {
    const { container } = render(<AnimTeamCard name="Les Rouges" color={[255, 0, 0]} score={0} />)
    expect(container.querySelector('.anim-team-card').className).not.toMatch(/anim-team-card-active/)
    expect(container.querySelector('.anim-team-card').className).not.toMatch(/anim-team-card-dimmed/)
  })

  it('active=true pose la classe anim-team-card-active', () => {
    const { container } = render(<AnimTeamCard name="Les Rouges" color={[255, 0, 0]} score={0} active />)
    expect(container.querySelector('.anim-team-card-active')).not.toBeNull()
  })

  it('dimmed=true pose la classe anim-team-card-dimmed', () => {
    const { container } = render(<AnimTeamCard name="Les Rouges" color={[255, 0, 0]} score={0} dimmed />)
    expect(container.querySelector('.anim-team-card-dimmed')).not.toBeNull()
  })

  it('active et dimmed sont indépendantes (peuvent coexister sur les deux classes si l\'appelant le décide)', () => {
    const { container } = render(<AnimTeamCard name="Les Rouges" color={[255, 0, 0]} score={0} active dimmed />)
    const card = container.querySelector('.anim-team-card')
    expect(card.className).toMatch(/anim-team-card-active/)
    expect(card.className).toMatch(/anim-team-card-dimmed/)
  })

  it('active/dimmed ne changent ni le nom, ni le score, ni la médaille, ni children', () => {
    render(
      <AnimTeamCard name="Les Rouges" color={[255, 0, 0]} score={12} medal="🏆" active dimmed>
        <span>1.234s</span>
      </AnimTeamCard>
    )
    expect(screen.getByText('Les Rouges')).toBeInTheDocument()
    expect(screen.getByText('12')).toBeInTheDocument()
    expect(screen.getByText('🏆')).toBeInTheDocument()
    expect(screen.getByText('1.234s')).toBeInTheDocument()
  })
})
