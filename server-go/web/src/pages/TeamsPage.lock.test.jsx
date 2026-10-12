import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, act } from '@testing-library/react'
import TeamsPage from './TeamsPage'
import { useGame } from '../hooks/GameContext'

// #249 — page Équipes : identité = clé, verrou pendant une question, TEAMS_UPDATE_REFUSED.

vi.mock('../hooks/GameContext', () => ({ useGame: vi.fn() }))
vi.mock('framer-motion', () => {
  const makeEl = (tag) => ({ children, initial, animate, exit, transition, whileHover, whileTap, ...props }) => {
    const Tag = tag
    return <Tag {...props}>{children}</Tag>
  }
  return { motion: { div: makeEl('div'), button: makeEl('button'), span: makeEl('span') }, AnimatePresence: ({ children }) => children }
})
vi.mock('../components/Button', () => ({
  default: ({ children, onClick, disabled, variant, size, fullWidth, ...rest }) => (
    <button onClick={onClick} disabled={disabled} {...rest}>{children}</button>
  ),
}))
vi.mock('../components/Card', () => ({
  default: ({ children, className, padding, variant, ...rest }) => <div className={className} {...rest}>{children}</div>,
}))
vi.mock('../components/TeamCard', () => ({ OtaModal: () => null }))
vi.mock('./TeamsPage.css', () => ({}))

const TEAMS = { 'Les Bleus': { NAME: 'Les Bleus', COLOR: [0, 0, 255], SCORE: 0 } }
const BUMPERS = { b1: { NAME: 'Paul', TEAM: 'Les Bleus', IS_VIRTUAL: true, IS_VPLAYER: true } }
const mk = (locked, extra = {}) => ({
  teams: TEAMS, bumpers: BUMPERS,
  gameState: { virtualPlayerCount: 0, enrollmentActive: false, TEAMS_LOCKED: locked },
  updateConfig: vi.fn(), showQRCode: vi.fn(), hideQRCode: vi.fn(), setVirtualPlayerLimit: vi.fn(),
  clearTeamsUpdateRefused: vi.fn(), teamsUpdateRefused: null,
  ...extra,
})

describe('TeamsPage — verrou pendant une question (#249)', () => {
  beforeEach(() => useGame.mockReset())

  it('renommage : NAME = nouvelle clé dans l\'objet équipe envoyé', () => {
    const g = mk(false)
    useGame.mockReturnValue(g)
    render(<TeamsPage />)
    const input = screen.getByLabelText("Nom de l'équipe Les Bleus")
    fireEvent.change(input, { target: { value: 'Les Azur' } })
    fireEvent.blur(input)
    const sent = g.updateConfig.mock.calls[0][0]
    expect(sent.teams['Les Bleus']).toBeUndefined()
    expect(sent.teams['Les Azur']).toMatchObject({ NAME: 'Les Azur', COLOR: [0, 0, 255] })
    expect(sent.bumpers.b1.TEAM).toBe('Les Azur')
  })

  it('hors question : champ et suppression actifs, pas de bandeau', () => {
    useGame.mockReturnValue(mk(false))
    render(<TeamsPage />)
    expect(screen.getByLabelText("Nom de l'équipe Les Bleus").disabled).toBe(false)
    expect(screen.getByLabelText('Supprimer Les Bleus').disabled).toBe(false)
    expect(screen.queryByText(/Une question est en cours/)).toBeNull()
  })

  it('pendant une question : champ de nom et ✕ désactivés avec infobulle, bandeau affiché', () => {
    const g = mk(true)
    useGame.mockReturnValue(g)
    render(<TeamsPage />)
    const input = screen.getByLabelText("Nom de l'équipe Les Bleus")
    const del = screen.getByLabelText('Supprimer Les Bleus')
    expect(input.disabled).toBe(true)
    expect(del.disabled).toBe(true)
    expect(input.title).toMatch(/Pendant une question, on ne peut ni renommer ni supprimer/)
    expect(del.title).toMatch(/Réessayez après la révélation de la réponse/)
    expect(screen.getByText(/renommer ou supprimer une équipe sera possible après la révélation de la réponse/)).toBeTruthy()
    fireEvent.click(del)
    expect(g.updateConfig).not.toHaveBeenCalled()
  })

  it('TEAMS_UPDATE_REFUSED : message, nom d\'origine réaffiché, refus consommé', () => {
    const g = mk(true)
    useGame.mockReturnValue(g)
    const { rerender } = render(<TeamsPage />)
    // course : renommage envoyé juste avant le verrou (champ encore actif dans cet onglet)
    useGame.mockReturnValue(mk(false, { clearTeamsUpdateRefused: g.clearTeamsUpdateRefused }))
    rerender(<TeamsPage />)
    const input = screen.getByLabelText("Nom de l'équipe Les Bleus")
    fireEvent.change(input, { target: { value: 'Les Azur' } })
    fireEvent.blur(input)
    useGame.mockReturnValue(mk(true, {
      clearTeamsUpdateRefused: g.clearTeamsUpdateRefused,
      teamsUpdateRefused: { reason: 'TEAMS_LOCKED_DURING_QUESTION', teams: ['Les Bleus'] },
    }))
    rerender(<TeamsPage />)
    expect(screen.getByRole('alert').textContent).toMatch(/Renommage refusé : une question est en cours\. « Les Bleus » n’a pas été renommée\./)
    expect(screen.getByLabelText("Nom de l'équipe Les Bleus").value).toBe('Les Bleus')
    expect(g.clearTeamsUpdateRefused).toHaveBeenCalled()
  })
})
