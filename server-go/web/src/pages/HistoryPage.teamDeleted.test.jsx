import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import HistoryPage from './HistoryPage'

// #249 — une ligne d'audit TEAM_DELETED n'apparaît jamais dans l'historique des points.

vi.mock('./QuestionsPage', () => ({ CATEGORIES: [] }))
vi.mock('./HistoryPage.css', () => ({}))
vi.mock('framer-motion', () => {
  const makeEl = (tag) => ({ children, initial, animate, exit, transition, layout, whileHover, whileTap, ...props }) => {
    const Tag = tag
    return <Tag {...props}>{children}</Tag>
  }
  return { motion: new Proxy({}, { get: (_, tag) => makeEl(tag) }), AnimatePresence: ({ children }) => children }
})

const POINTS = {
  EVENT_TYPE: 'POINTS', QUESTION_ID: '7', QUESTION_TEXT: 'Capitale ?', TIMESTAMP: 1000,
  WINNER_TYPE: 'TEAM', TEAM_NAME: 'Les Bleus', TEAM_COLOR: [0, 0, 255], POINTS: 2,
}
const DELETED = { EVENT_TYPE: 'TEAM_DELETED', TEAM_NAME: 'Les Verts', TIMESTAMP: 2000, POINTS: 0 }

describe('HistoryPage — TEAM_DELETED ignoré (#249)', () => {
  beforeEach(() => {
    vi.stubGlobal('fetch', vi.fn(() => Promise.resolve({ ok: true, json: () => Promise.resolve([POINTS, DELETED]) })))
  })
  afterEach(() => vi.unstubAllGlobals())

  it('aucun groupe « Question #unknown », aucune ligne d\'audit, compteur sans l\'audit', async () => {
    const { container } = render(<HistoryPage />)
    await waitFor(() => expect(screen.getByText(/1 evenement enregistre/)).toBeTruthy())
    expect(container.textContent).not.toMatch(/unknown/)
    expect(container.textContent).not.toMatch(/Les Verts/)
    expect(container.querySelectorAll('.history-group')).toHaveLength(1)
  })
})
