import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import CategoryBalance from './CategoryBalance'

// #285 — « CategoryBalance compte aussi les personnalisées » (contracts/http-endpoints.md §Catégories « Frontend »).
// Les catégories personnalisées arrivent de GET /api/categories par la prop `customCategories` (même nom que
// CategoryBadge / CategoryPicker).
vi.mock('./CategoryBalance.css', () => ({}))
vi.mock('framer-motion', () => ({
  motion: new Proxy({}, { get: (_t, tag) => ({ children, initial, animate, exit, transition, whileHover, ...props }) => {
    const Tag = tag
    return <Tag {...props}>{children}</Tag>
  } }),
  AnimatePresence: ({ children }) => children,
}))
vi.mock('../pages/QuestionsPage', () => ({
  CATEGORIES: { GEOGRAPHY: { label: 'Geographie', icon: '🌍', color: '#3b82f6' } },
}))

const questions = [
  { ID: '1', TYPE: 'SPEEDY', CATEGORY: 'GEOGRAPHY', POINTS: '1' },
  { ID: '2', TYPE: 'SPEEDY', CATEGORY: 'CINEMA', POINTS: '1' },
  { ID: '3', TYPE: 'SPEEDY', CATEGORY: 'CINEMA', POINTS: '1' },
]
const custom = [{ key: 'CINEMA', name: 'Cinéma', imageURL: '/files/categories/CINEMA.png', color: '#0ea5e9', isCustom: true }]

describe('CategoryBalance — catégories personnalisées (#285)', () => {
  it('une catégorie personnalisée est représentée sous son nom (pas « Autre » ni clé brute)', () => {
    const { container } = render(<CategoryBalance questions={questions} customCategories={custom} />)
    expect(container.textContent).toContain('Cinéma')
    expect(container.textContent).not.toContain('CINEMA')
  })
})
