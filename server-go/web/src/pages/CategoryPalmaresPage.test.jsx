import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import CategoryPalmaresPage from './CategoryPalmaresPage'
import { getRgbColor } from '../utils/colorUtils'
import { TEAM_COLORS } from '../constants/colors'

// ---------------------------------------------------------------------------
// Tests : CategoryPalmaresPage — rendu de couleur via le getRgbColor partagé
// (#113, tâche F4)
//
// Même exigence que Podium.test.jsx / VPlayerHeader.test.jsx : les couleurs
// d'équipe affichées dans le classement PALMARES (badges du résumé replié)
// doivent être résolues via le getRgbColor partagé — identiques à celles
// affichées sur le podium et l'admin pour la même couleur RGB.
// ---------------------------------------------------------------------------

vi.mock('framer-motion', () => {
  const makeEl = (tag) => ({ children, initial, animate, exit, transition, ...props }) => {
    const Tag = tag
    return <Tag {...props}>{children}</Tag>
  }
  return {
    motion: { div: makeEl('div'), button: makeEl('button') },
    AnimatePresence: ({ children }) => children,
  }
})

vi.mock('../components/Card', () => ({
  default: ({ children, className, ...rest }) => <div className={className} {...rest}>{children}</div>,
  CardHeader: ({ children, className, onClick }) => <div className={className} onClick={onClick}>{children}</div>,
  CardBody: ({ children, className }) => <div className={className}>{children}</div>,
}))

vi.mock('../components/Podium', () => ({ default: () => null }))
vi.mock('../components/QuestionCard', () => ({ CATEGORIES: {} }))
vi.mock('./CategoryPalmaresPage.css', () => ({}))

const grenat = TEAM_COLORS.find(c => c.key === 'rouge-profond')
const bleu = TEAM_COLORS.find(c => c.key === 'bleu')
const legacyColor = [239, 68, 68] // ancien PRESET_COLORS "Red", hors palette #113

const palmaresFixture = [
  {
    category: 'GEOGRAPHY',
    name: 'Geographie',
    imageURL: null,
    color: null,
    totalPoints: 240,
    teams: [
      { name: 'Les Bleus', points: 100, color: bleu.rgb },
      { name: 'Les Grenats', points: 90, color: grenat.rgb },
      { name: 'Legacy', points: 50, color: legacyColor },
    ],
    players: [],
  },
]

function mockFetchOnce(data) {
  global.fetch = vi.fn().mockResolvedValue({
    ok: true,
    json: async () => data,
  })
}

describe('CategoryPalmaresPage — couleur d\'équipe via getRgbColor (#113)', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it("le badge résumé d'une équipe (ton vif de la palette) utilise exactement le getRgbColor partagé", async () => {
    mockFetchOnce(palmaresFixture)
    render(<CategoryPalmaresPage />)

    const badge = await screen.findByText('Les Bleus')
    const summaryBadge = badge.closest('.summary-badge')
    expect(summaryBadge).toHaveStyle({ color: getRgbColor(bleu.rgb) })
    expect(summaryBadge).toHaveStyle({ color: `rgb(${bleu.rgb.join(',')})` })
  })

  it("le badge résumé d'une équipe (ton profond de la palette) utilise exactement le getRgbColor partagé", async () => {
    mockFetchOnce(palmaresFixture)
    render(<CategoryPalmaresPage />)

    const badge = await screen.findByText('Les Grenats')
    const summaryBadge = badge.closest('.summary-badge')
    expect(summaryBadge).toHaveStyle({ color: `rgb(${grenat.rgb.join(',')})` })
  })

  it('non-régression : une ancienne couleur (hors palette #113) reste rendue via le même getRgbColor', async () => {
    mockFetchOnce(palmaresFixture)
    render(<CategoryPalmaresPage />)

    const badge = await screen.findByText('Legacy')
    const summaryBadge = badge.closest('.summary-badge')
    expect(summaryBadge).toHaveStyle({ color: getRgbColor(legacyColor) })
  })
})
