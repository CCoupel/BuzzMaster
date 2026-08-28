import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import TeamCard from './TeamCard'

// ---------------------------------------------------------------------------
// TeamCard — badge RAFALE (`rafaleStats`), milestone v8.0.0 #16/#199,
// contrat contracts/rafale.md §6.
//
// Deux variantes exclusives, pilotées par `rafaleStats = { counter,
// suggestedPoints }` :
//   - suggestedPoints !== null (sous-phase ROUND_END, §6.2) : badge
//     "→ N pts sugg." — valeur suggérée = compteur retenu × barème,
//     AJUSTABLE avant clic (pas un score déjà attribué)
//   - suggestedPoints === null (pendant la manche, §6.1) : badge
//     "N bonne(s)" — un COMPTEUR, explicitement pas un score réel
// Pas de gate sur `gamePhase === 'REVEALED'` (RAFALE n'atteint jamais cette
// phase, §2.1) — contrairement au badge MEMORY voisin, gaté sur REVEALED.
//
// Composant déjà livré par dev-frontend (Batch 3, #199) au moment où ce
// fichier est écrit — tests écrits contre son API réelle.
// ---------------------------------------------------------------------------

vi.mock('./TeamCard.css', () => ({}))

const renderTeamCard = (overrides = {}) =>
  render(
    <TeamCard
      name="Équipe Test"
      color={[239, 68, 68]}
      score={0}
      buzzers={[]}
      {...overrides}
    />
  )

describe('TeamCard — RAFALE compteur live (pendant la manche, §6.1)', () => {
  it('rafaleStats={counter:3, suggestedPoints:null} : affiche "3 bonnes"', () => {
    renderTeamCard({ rafaleStats: { counter: 3, suggestedPoints: null } })
    expect(screen.getByText(/3 bonnes/)).toBeInTheDocument()
  })

  it('compteur=1 : accord singulier "1 bonne" (pas "1 bonnes")', () => {
    renderTeamCard({ rafaleStats: { counter: 1, suggestedPoints: null } })
    expect(screen.getByText('1 bonne')).toBeInTheDocument()
  })

  it('compteur=0 : badge affiché quand même (rafaleStats non-null, juste counter=0)', () => {
    renderTeamCard({ rafaleStats: { counter: 0, suggestedPoints: null } })
    expect(screen.getByText(/0 bonne/)).toBeInTheDocument()
  })

  it('le badge live porte la classe .rafale-counter-badge (pas .rafale-suggested-badge)', () => {
    const { container } = renderTeamCard({ rafaleStats: { counter: 2, suggestedPoints: null } })
    expect(container.querySelector('.rafale-counter-badge')).not.toBeNull()
    expect(container.querySelector('.rafale-suggested-badge')).toBeNull()
  })
})

describe('TeamCard — RAFALE valeur suggérée (fin de manche, ROUND_END, §6.2)', () => {
  it('rafaleStats={counter:4, suggestedPoints:40} : affiche "→ 40 pts sugg."', () => {
    renderTeamCard({ rafaleStats: { counter: 4, suggestedPoints: 40 } })
    expect(screen.getByText(/→ 40 pts sugg\./)).toBeInTheDocument()
    // Le compteur brut n'est PAS affiché en même temps que la suggestion —
    // les 2 badges sont mutuellement exclusifs.
    expect(screen.queryByText(/4 bonnes/)).not.toBeInTheDocument()
  })

  it('suggestedPoints=0 (aucune bonne réponse) : badge suggéré affiché tout de même ("→ 0 pts sugg."), pas de repli sur le compteur live', () => {
    renderTeamCard({ rafaleStats: { counter: 0, suggestedPoints: 0 } })
    expect(screen.getByText(/→ 0 pts sugg\./)).toBeInTheDocument()
  })

  it('le badge suggéré porte la classe .rafale-suggested-badge, et le titre explique qu\'il est ajustable', () => {
    const { container } = renderTeamCard({ rafaleStats: { counter: 4, suggestedPoints: 40 } })
    const badge = container.querySelector('.rafale-suggested-badge')
    expect(badge).not.toBeNull()
    expect(badge.title.toLowerCase()).toContain('ajustable')
  })
})

describe('TeamCard — RAFALE absent (non-régression, autres modes de jeu)', () => {
  it('rafaleStats=null (défaut) : aucun badge RAFALE rendu', () => {
    const { container } = renderTeamCard()
    expect(container.querySelector('.rafale-counter-badge')).toBeNull()
    expect(container.querySelector('.rafale-suggested-badge')).toBeNull()
  })

  it('rafaleStats absent : le badge MEMORY (memoryStats + gamePhase=REVEALED) continue de s\'afficher normalement — pas de régression du badge voisin', () => {
    renderTeamCard({
      memoryStats: { pairs: 3, totalPairs: 5, pointsPerPair: 10, errors: 0 },
      gamePhase: 'REVEALED',
    })
    expect(screen.getByText(/\+30 pts/)).toBeInTheDocument()
  })

  it('rafaleStats présent a priorité sur le badge MEMORY s\'ils sont fournis ensemble par erreur (RAFALE est vérifié EN PREMIER dans le composant)', () => {
    renderTeamCard({
      rafaleStats: { counter: 2, suggestedPoints: null },
      memoryStats: { pairs: 3, totalPairs: 5, pointsPerPair: 10, errors: 0 },
      gamePhase: 'REVEALED',
    })
    expect(screen.getByText(/2 bonnes/)).toBeInTheDocument()
    expect(screen.queryByText(/\+30 pts/)).not.toBeInTheDocument()
  })
})
