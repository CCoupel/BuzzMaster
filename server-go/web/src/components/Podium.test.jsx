import { describe, it, expect, vi } from 'vitest'
import { render } from '@testing-library/react'
import Podium from './Podium'
import { getRgbColor } from '../utils/colorUtils'
import { TEAM_COLORS } from '../constants/colors'

// ---------------------------------------------------------------------------
// Tests : Podium — rendu de couleur via le getRgbColor partagé (#113, tâche F4)
//
// Plan (_work/reports/plan-20260726-121500.md, F4) : Podium doit utiliser le
// même getRgbColor (utils/colorUtils) que TeamsPage/CategoryPalmaresPage/
// VPlayerHeader, sans copie locale — c'est ce qui garantit qu'une même couleur
// d'équipe est rendue à l'identique partout (critère d'acceptation du plan).
// ---------------------------------------------------------------------------

vi.mock('./Podium.css', () => ({}))

vi.mock('framer-motion', () => {
  const makeEl = (tag) => ({ children, initial, animate, exit, transition, whileHover, whileTap, layout, ...props }) => {
    const Tag = tag
    return <Tag {...props}>{children}</Tag>
  }
  return {
    motion: { div: makeEl('div') },
  }
})

describe('Podium — couleur d\'équipe via getRgbColor (#113)', () => {
  it("l'avatar de la 1ère place utilise exactement le getRgbColor partagé pour un ton vif de la palette", () => {
    const bleu = TEAM_COLORS.find(c => c.key === 'bleu')
    const { container } = render(
      <Podium teams={[{ name: 'Les Bleus', score: 100, color: bleu.rgb, rank: 1 }]} />
    )

    const avatar = container.querySelector('.podium-avatar')
    expect(avatar).not.toBeNull()
    expect(avatar).toHaveStyle({ backgroundColor: getRgbColor(bleu.rgb) })
    // Invariance (#113) : pour ces couleurs, le RGB affiché == RGB stocké.
    expect(avatar).toHaveStyle({ backgroundColor: `rgb(${bleu.rgb.join(',')})` })
  })

  it("l'avatar utilise exactement le getRgbColor partagé pour un ton profond de la palette", () => {
    const marine = TEAM_COLORS.find(c => c.key === 'bleu-profond')
    const { container } = render(
      <Podium teams={[{ name: 'Les Marine', score: 80, color: marine.rgb, rank: 1 }]} />
    )

    const avatar = container.querySelector('.podium-avatar')
    expect(avatar).toHaveStyle({ backgroundColor: getRgbColor(marine.rgb) })
    expect(avatar).toHaveStyle({ backgroundColor: `rgb(${marine.rgb.join(',')})` })
  })

  it('non-régression : une ancienne couleur (hors palette #113) reste rendue via le même getRgbColor (pas de rgb() brut)', () => {
    const legacyColor = [239, 68, 68] // ancien PRESET_COLORS "Red"
    const { container } = render(
      <Podium teams={[{ name: 'Legacy', score: 50, color: legacyColor, rank: 1 }]} />
    )

    const avatar = container.querySelector('.podium-avatar')
    expect(avatar).toHaveStyle({ backgroundColor: getRgbColor(legacyColor) })
  })
})
