import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import HistoryPage from './HistoryPage'

// #252 (v11.2) — scénario 11 : la page Historique marque « 🔇 correction cachée » les entrées POINTS_AWARDED/POINTS
// enregistrées avec DISPLAY:false (piste d'audit) ; les entrées visibles ne portent aucune marque.
vi.mock('./QuestionsPage', () => ({ CATEGORIES: [] }))
vi.mock('./HistoryPage.css', () => ({}))
vi.mock('framer-motion', () => {
  const makeEl = (tag) => ({ children, initial, animate, exit, transition, layout, whileHover, whileTap, ...props }) => {
    const Tag = tag
    return <Tag {...props}>{children}</Tag>
  }
  return { motion: new Proxy({}, { get: (_, tag) => makeEl(tag) }), AnimatePresence: ({ children }) => children }
})

const base = { EVENT_TYPE: 'POINTS', QUESTION_ID: '7', QUESTION_TEXT: 'Capitale ?', WINNER_TYPE: 'TEAM', TEAM_COLOR: [0, 0, 255], POINTS: 2 }
const VISIBLE = { ...base, TIMESTAMP: 1000, TEAM_NAME: 'Les Bleus' }
const HIDDEN = { ...base, TIMESTAMP: 2000, TEAM_NAME: 'Les Rouges', POINTS: 3, DISPLAY: false }
const EXPLICIT_TRUE = { ...base, TIMESTAMP: 1500, TEAM_NAME: 'Les Verts', DISPLAY: true }

describe('HistoryPage — corrections cachées (#252)', () => {
  beforeEach(() => {
    vi.stubGlobal('fetch', vi.fn(() => Promise.resolve({ ok: true, json: () => Promise.resolve([VISIBLE, HIDDEN, EXPLICIT_TRUE]) })))
  })
  afterEach(() => vi.unstubAllGlobals())

  it('Critical — seule l’entrée DISPLAY:false porte la marque 🔇 « correction cachée » (accessible)', async () => {
    const { container } = render(<HistoryPage />)
    await waitFor(() => expect(screen.getByText(/3 evenements enregistres/)).toBeTruthy())
    fireEvent.click(screen.getByText('Tout ouvrir'))

    const marks = container.querySelectorAll('[aria-label="Correction cachée"]')
    expect(marks.length).toBe(1)
    expect(marks[0].textContent).toMatch(/🔇/)
    expect(marks[0].getAttribute('title')).toMatch(/Correction cachée/)
    expect((container.textContent.match(/🔇/g) || []).length).toBe(1)
  })

  it('la marque est portée par la ligne de l’équipe concernée', async () => {
    const { container } = render(<HistoryPage />)
    await waitFor(() => expect(screen.getByText('Tout ouvrir')).toBeTruthy())
    fireEvent.click(screen.getByText('Tout ouvrir'))
    await waitFor(() => expect(screen.getByText(/Les Rouges/)).toBeTruthy())
    const mark = container.querySelector('[aria-label="Correction cachée"]')
    const row = mark.closest('tr')
    expect(row.textContent).toMatch(/Les Rouges/)
    expect(row.textContent).toMatch(/\+3 pts/)
    expect(row.textContent).not.toMatch(/Les Bleus/)
  })

  it('aucune marque quand aucune entrée n’est cachée (DISPLAY absent ou true)', async () => {
    vi.stubGlobal('fetch', vi.fn(() => Promise.resolve({ ok: true, json: () => Promise.resolve([VISIBLE, EXPLICIT_TRUE]) })))
    const { container } = render(<HistoryPage />)
    await waitFor(() => expect(screen.getByText(/2 evenements enregistres/)).toBeTruthy())
    fireEvent.click(screen.getByText('Tout ouvrir'))
    expect(container.querySelectorAll('[aria-label="Correction cachée"]').length).toBe(0)
  })
})
