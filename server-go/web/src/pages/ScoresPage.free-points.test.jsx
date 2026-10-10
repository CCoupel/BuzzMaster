import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, act, waitFor } from '@testing-library/react'
import ScoresPage from './ScoresPage'
import { useGame } from '../hooks/GameContext'

// #252 — page Scores : montant libre, interrupteur « Afficher sur la TV », dernières attributions.

vi.mock('../hooks/GameContext', () => ({ useGame: vi.fn() }))
vi.mock('../components/Podium', () => ({ default: () => <div data-testid="podium" /> }))
vi.mock('./ScoresPage.css', () => ({}))
vi.mock('framer-motion', () => {
  const makeEl = (tag) => ({ children, initial, animate, exit, transition, layout, whileHover, whileTap, ...props }) => {
    const Tag = tag
    return <Tag {...props}>{children}</Tag>
  }
  return { motion: new Proxy({}, { get: (_, tag) => makeEl(tag) }), AnimatePresence: ({ children }) => children }
})

const mk = (seq = 0) => ({
  gameState: { SCORE_CHANGE: { SEQ: seq, DISPLAY: true, TEAMS: [], PLAYERS: [] } },
  teams: { 'Les Bleus': { SCORE: 42, COLOR: [0, 0, 255] } },
  bumpers: { b1: { NAME: 'Paul', TEAM: 'Les Bleus', SCORE: 12 } },
  setTeamPoints: vi.fn(), setBumperPoints: vi.fn(),
})
const HISTORY = [
  { EVENT_TYPE: 'POINTS_AWARDED', TIMESTAMP: 1, WINNER_TYPE: 'TEAM', TEAM_NAME: 'Les Bleus', POINTS: 3, QUESTION_ID: '9', DISPLAY: true },
  { EVENT_TYPE: 'POINTS_AWARDED', TIMESTAMP: 5, WINNER_TYPE: 'TEAM', TEAM_NAME: 'Les Verts', POINTS: -2, DISPLAY: false },
  { EVENT_TYPE: 'TEAM_DELETED', TIMESTAMP: 9, TEAM_NAME: 'Les Gris', POINTS: 0 },
  { EVENT_TYPE: 'POINTS_AWARDED', TIMESTAMP: 2, WINNER_TYPE: 'PLAYER', PLAYER_NAME: 'Paul', TEAM_NAME: 'Les Bleus', POINTS: 5 },
  ...[10, 11, 12, 13].map(t => ({ EVENT_TYPE: 'POINTS_AWARDED', TIMESTAMP: t, WINNER_TYPE: 'TEAM', TEAM_NAME: `T${t}`, POINTS: 1 })),
]
const free = () => screen.getByLabelText('Nombre de points libre (négatif autorisé)')
const addBtns = () => screen.getAllByRole('button', { name: /^Donner / })
const rmBtns = () => screen.getAllByRole('button', { name: /^Retirer / })

describe('ScoresPage — montant libre et affichage TV (#252)', () => {
  beforeEach(() => {
    useGame.mockReset()
    vi.stubGlobal('fetch', vi.fn(() => Promise.resolve({ ok: true, json: () => Promise.resolve(HISTORY) })))
  })
  afterEach(() => vi.unstubAllGlobals())

  it('prédéfini : boutons +1/−1, envoi sans champ DISPLAY (v11.1)', () => {
    const g = mk(); useGame.mockReturnValue(g)
    render(<ScoresPage />)
    fireEvent.click(addBtns()[0])
    expect(g.setTeamPoints).toHaveBeenCalledWith('Les Bleus', 1, { display: true })
    fireEvent.click(rmBtns()[1])
    expect(g.setBumperPoints).toHaveBeenCalledWith('b1', -1, { display: true })
  })

  it('montant libre 7 → boutons « −7 / +7 » équipes ET joueurs ; −4 → « −4 / +4 »', () => {
    const g = mk(); useGame.mockReturnValue(g)
    render(<ScoresPage />)
    fireEvent.change(free(), { target: { value: '7' } })
    expect(addBtns().map(b => b.textContent)).toEqual(['+7', '+7'])
    expect(rmBtns().map(b => b.textContent)).toEqual(['−7', '−7'])
    fireEvent.click(addBtns()[1])
    expect(g.setBumperPoints).toHaveBeenCalledWith('b1', 7, { display: true })
    fireEvent.change(free(), { target: { value: '-4' } })
    expect(addBtns()[0].textContent).toBe('+4')
    expect(rmBtns()[0].textContent).toBe('−4')
    fireEvent.click(rmBtns()[0])
    expect(g.setTeamPoints).toHaveBeenLastCalledWith('Les Bleus', -4, { display: true })
  })

  it.each(['2.5', '0', '1000', '-1000'])('saisie invalide « %s » : erreur visible, boutons désactivés, rien envoyé', (v) => {
    const g = mk(); useGame.mockReturnValue(g)
    render(<ScoresPage />)
    fireEvent.change(free(), { target: { value: v } })
    expect(screen.getByRole('alert').textContent).toBe('Entier de −999 à 999')
    ;[...addBtns(), ...rmBtns()].forEach(b => expect(b.disabled).toBe(true))
    fireEvent.click(addBtns()[0])
    expect(g.setTeamPoints).not.toHaveBeenCalled()
  })

  it('un bouton prédéfini efface le champ libre', () => {
    useGame.mockReturnValue(mk())
    render(<ScoresPage />)
    fireEvent.change(free(), { target: { value: '7' } })
    fireEvent.click(screen.getByRole('button', { name: '5' }))
    expect(free().value).toBe('')
    expect(addBtns()[0].textContent).toBe('+5')
  })

  it('interrupteur activé par défaut ; désactivé : bandeau, 🔇 sur les boutons, DISPLAY:false envoyé', () => {
    const g = mk(); useGame.mockReturnValue(g)
    render(<ScoresPage />)
    const sw = screen.getByRole('switch', { name: 'Afficher les attributions sur la TV' })
    expect(sw.getAttribute('aria-checked')).toBe('true')
    expect(screen.queryByText(/Correction cachée : les points sont enregistrés/)).toBeNull()
    fireEvent.click(sw)
    expect(sw.getAttribute('aria-checked')).toBe('false')
    expect(screen.getByText(/Correction cachée : les points sont enregistrés sans animation, sans son ni LED/)).toBeTruthy()
    expect(addBtns()[0].textContent).toBe('+1🔇')
    expect(addBtns()[0].getAttribute('title')).toMatch(/sans affichage TV/)
    fireEvent.click(addBtns()[0])
    expect(g.setTeamPoints).toHaveBeenCalledWith('Les Bleus', 1, { display: false })
  })

  it('réactivé à chaque ouverture de la page (état non sauvegardé)', () => {
    useGame.mockReturnValue(mk())
    const first = render(<ScoresPage />)
    fireEvent.click(screen.getByRole('switch'))
    expect(screen.getByRole('switch').getAttribute('aria-checked')).toBe('false')
    first.unmount()
    render(<ScoresPage />)
    expect(screen.getByRole('switch').getAttribute('aria-checked')).toBe('true')
  })

  it('Dernières attributions : 5 plus récentes, TEAM_DELETED exclu, 🔇 pour les corrections cachées', async () => {
    useGame.mockReturnValue(mk())
    render(<ScoresPage />)
    const section = (await screen.findByText('Dernières attributions')).closest('section')
    await waitFor(() => expect(section.querySelectorAll('li').length).toBe(5))
    const items = [...section.querySelectorAll('li')].map(li => li.textContent)
    expect(items.join('|')).not.toMatch(/Les Gris/)
    expect(items[0]).toBe('+1 T13')
    expect(items[4]).toMatch(/−2 Les Verts.*🔇 correction cachée/) // 5ᵉ plus récente, cachée
    expect(items.join('|')).not.toMatch(/Les Bleus \(question 9\)/) // 7ᵉ : hors des 5
  })

  it('Dernières attributions : une correction cachée récente porte la marque 🔇', async () => {
    fetch.mockImplementation(() => Promise.resolve({ ok: true, json: () => Promise.resolve(HISTORY.slice(0, 4)) }))
    useGame.mockReturnValue(mk())
    render(<ScoresPage />)
    const li = await screen.findByText(/−2 Les Verts/)
    expect(li.textContent).toMatch(/🔇 correction cachée/)
    expect(screen.getByText(/\+3 Les Bleus \(question 9\)/)).toBeTruthy()
    expect(screen.getByText(/\+5 Paul/)).toBeTruthy()
  })

  it('historique rechargé sur un nouveau SCORE_CHANGE.SEQ, pas à chaque UPDATE ; intervalle de 5 s conservé', async () => {
    vi.useFakeTimers()
    try {
      const g = mk(0)
      useGame.mockReturnValue(g)
      const { rerender } = render(<ScoresPage />)
      await act(async () => {})
      const n0 = fetch.mock.calls.length // chargement initial
      // UPDATE sans nouvelle attribution (nouveaux objets teams/bumpers) : aucun fetch
      useGame.mockReturnValue({ ...g, teams: { ...g.teams }, bumpers: { ...g.bumpers } })
      rerender(<ScoresPage />)
      await act(async () => {})
      expect(fetch.mock.calls.length).toBe(n0)
      // nouvelle attribution : un fetch
      useGame.mockReturnValue({ ...g, gameState: { SCORE_CHANGE: { SEQ: 1, DISPLAY: true, TEAMS: [], PLAYERS: [] } } })
      rerender(<ScoresPage />)
      await act(async () => {})
      expect(fetch.mock.calls.length).toBe(n0 + 1)
      // l'intervalle de 5 s tourne toujours, sans être réarmé par les UPDATE
      await act(async () => { vi.advanceTimersByTime(5000) })
      expect(fetch.mock.calls.length).toBe(n0 + 2)
    } finally {
      vi.useRealTimers()
    }
  })

  it('Dernières attributions : un crédit à 0 s\'affiche « 0 pt », jamais « −0 »', async () => {
    fetch.mockImplementation(() => Promise.resolve({ ok: true, json: () => Promise.resolve([
      { EVENT_TYPE: 'POINTS_AWARDED', TIMESTAMP: 7, WINNER_TYPE: 'TEAM', TEAM_NAME: 'Les Bleus', POINTS: 0, QUESTION_ID: '9' },
    ]) }))
    useGame.mockReturnValue(mk())
    render(<ScoresPage />)
    const li = await screen.findByText(/Les Bleus \(question 9\)/)
    expect(li.textContent).toBe('0 pt Les Bleus (question 9)')
    expect(li.textContent).not.toMatch(/−0/)
  })
})
