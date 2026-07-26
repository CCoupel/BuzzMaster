import { describe, it, expect, vi } from 'vitest'
import { render } from '@testing-library/react'
import VPlayerHeader from './VPlayerHeader'
import { getRgbColor } from '../utils/colorUtils'
import { TEAM_COLORS } from '../constants/colors'

// ---------------------------------------------------------------------------
// Tests : VPlayerHeader — rendu de couleur via le getRgbColor partagé (#113, F4)
//
// Même exigence que Podium.test.jsx / TeamsPage : une équipe assignée à un
// VJoueur doit afficher exactement le RGB de sa couleur d'équipe, résolu via
// le getRgbColor partagé — pas d'approximation ni de rgb() brut local.
// ---------------------------------------------------------------------------

vi.mock('./VPlayerHeader.css', () => ({}))

describe('VPlayerHeader — couleur d\'équipe via getRgbColor (#113)', () => {
  it("l'avatar utilise exactement le getRgbColor partagé pour un ton vif de la palette", () => {
    const violet = TEAM_COLORS.find(c => c.key === 'violet')
    const team = { NAME: 'Les Violets', COLOR: violet.rgb }
    const bumper = { NAME: 'Alice', TEAM: 'Les Violets', SCORE: 10 }

    const { container } = render(<VPlayerHeader bumper={bumper} team={team} />)

    const avatar = container.querySelector('.vplayer-avatar')
    expect(avatar).toHaveStyle({ backgroundColor: getRgbColor(violet.rgb) })
    // Invariance (#113) : RGB affiché == RGB stocké pour les 16 couleurs de la palette.
    expect(avatar).toHaveStyle({ backgroundColor: `rgb(${violet.rgb.join(',')})` })
  })

  it("l'avatar utilise exactement le getRgbColor partagé pour un ton profond de la palette", () => {
    const grenat = TEAM_COLORS.find(c => c.key === 'rouge-profond')
    const team = { NAME: 'Les Grenats', COLOR: grenat.rgb }
    const bumper = { NAME: 'Bob', TEAM: 'Les Grenats', SCORE: 5 }

    const { container } = render(<VPlayerHeader bumper={bumper} team={team} />)

    const avatar = container.querySelector('.vplayer-avatar')
    expect(avatar).toHaveStyle({ backgroundColor: `rgb(${grenat.rgb.join(',')})` })
  })

  it("retombe sur le gris par défaut quand aucune équipe n'est assignée", () => {
    const bumper = { NAME: 'Charlie', TEAM: '', SCORE: 0 }

    const { container } = render(<VPlayerHeader bumper={bumper} team={null} />)

    const avatar = container.querySelector('.vplayer-avatar')
    expect(avatar).toHaveStyle({ backgroundColor: 'var(--gray-500)' })
    expect(container.querySelector('.vplayer-header')).toHaveClass('waiting')
  })
})
