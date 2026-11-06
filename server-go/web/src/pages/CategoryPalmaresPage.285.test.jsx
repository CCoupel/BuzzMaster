import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import CategoryPalmaresPage from './CategoryPalmaresPage'

// #285 (LOT-4) — le bucket historique UNKNOWN (QUESTION_CATEGORY == "", fusion d'une catégorie supprimée) est affiché
// « Sans catégorie » (et non plus « Inconnue »). contracts/http-endpoints.md §Catégories « Frontend ».

vi.mock('framer-motion', () => {
  const makeEl = (tag) => ({ children, initial, animate, exit, transition, ...props }) => {
    const Tag = tag
    return <Tag {...props}>{children}</Tag>
  }
  return { motion: { div: makeEl('div'), button: makeEl('button') }, AnimatePresence: ({ children }) => children }
})
vi.mock('../components/Card', () => ({
  default: ({ children, className, ...rest }) => <div className={className} {...rest}>{children}</div>,
  CardHeader: ({ children, className, onClick }) => <div className={className} onClick={onClick}>{children}</div>,
  CardBody: ({ children, className }) => <div className={className}>{children}</div>,
}))
vi.mock('../components/Podium', () => ({ default: () => null }))
vi.mock('../components/QuestionCard', () => ({ CATEGORIES: {} }))
vi.mock('./CategoryPalmaresPage.css', () => ({}))

const fixture = [
  { category: 'UNKNOWN', name: '', imageURL: null, color: null, totalPoints: 19, teams: [{ name: 'Les Bleus', points: 19, color: [0, 0, 255] }], players: [] },
  { category: 'CINEMA', name: 'Cinéma', imageURL: '/files/categories/CINEMA.png', color: '#0ea5e9', totalPoints: 5, teams: [{ name: 'Les Rouges', points: 5, color: [255, 0, 0] }], players: [] },
]

afterEach(() => vi.restoreAllMocks())

describe('CategoryPalmaresPage — « Sans catégorie » (#285)', () => {
  it('le bucket UNKNOWN s\'affiche « Sans catégorie »', async () => {
    global.fetch = vi.fn().mockResolvedValue({ ok: true, json: async () => fixture })
    render(<CategoryPalmaresPage />)
    expect(await screen.findByText('Sans catégorie')).toBeInTheDocument()
    expect(screen.queryByText('Inconnue')).not.toBeInTheDocument()
  })

  it('une catégorie résolue garde son nom', async () => {
    global.fetch = vi.fn().mockResolvedValue({ ok: true, json: async () => fixture })
    render(<CategoryPalmaresPage />)
    expect(await screen.findByText('Cinéma')).toBeInTheDocument()
  })
})
