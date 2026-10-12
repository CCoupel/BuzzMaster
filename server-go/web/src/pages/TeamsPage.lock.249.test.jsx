import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, within } from '@testing-library/react'
import TeamsPage from './TeamsPage'
import { useGame } from '../hooks/GameContext'

// #249 (v11.2) — page Équipes : identité = clé, équipes verrouillées pendant une question.
// Maquette (en attente de validation au moment de l'écriture) : _work/mockup/v11.2/ui/admin_teams__lock_during_question.html ;
// contrats : websocket-actions.md §UPDATE / §TEAMS_UPDATE_REFUSED, game-state.md §TEAMS_LOCKED.
// Reproduction (statut `regression`) : harness246_rename_gamepage — handleRenameTeam conservait l'objet équipe
// (NAME = ancien nom), d'où des réponses ARDOISE introuvables après renommage.

vi.mock('../hooks/GameContext', () => ({ useGame: vi.fn() }))
vi.mock('framer-motion', () => {
  const makeEl = (tag) => ({ children, initial, animate, exit, transition, whileHover, whileTap, layout, ...props }) => {
    const Tag = tag
    return <Tag {...props}>{children}</Tag>
  }
  return { motion: { div: makeEl('div'), button: makeEl('button'), span: makeEl('span') }, AnimatePresence: ({ children }) => children }
})
vi.mock('../components/Button', () => ({
  default: ({ children, onClick, disabled, variant, size, fullWidth, ...rest }) => <button onClick={onClick} disabled={disabled} {...rest}>{children}</button>,
}))
vi.mock('../components/Card', () => ({ default: ({ children, className, padding, variant, ...rest }) => <div className={className} {...rest}>{children}</div> }))
vi.mock('../components/TeamCard', () => ({ OtaModal: () => null }))
vi.mock('./TeamsPage.css', () => ({}))

const TEAMS = {
  'Les Bleus': { NAME: 'Les Bleus', SCORE: 3, COLOR: [0, 0, 255], COLOR_NAME: 'blue' },
  'Les Rouges': { NAME: 'Les Rouges', SCORE: 0, COLOR: [255, 0, 0], COLOR_NAME: 'red' },
}
const BUMPERS = {
  vjP: { NAME: 'Paul', TEAM: 'Les Bleus', IS_VIRTUAL: true, IS_VPLAYER: true, CONNECTED: true },
  vjM: { NAME: 'Mamie', TEAM: 'Les Rouges', IS_VIRTUAL: true, IS_VPLAYER: true, CONNECTED: true },
}
const mk = (over = {}) => ({
  teams: TEAMS, bumpers: BUMPERS,
  gameState: { virtualPlayerCount: 0, enrollmentActive: false, TEAMS_LOCKED: false, ...(over.gameState || {}) },
  updateConfig: vi.fn(), showQRCode: vi.fn(), hideQRCode: vi.fn(), setVirtualPlayerLimit: vi.fn(),
  deleteBumper: vi.fn(), releaseBumperName: vi.fn(),
  teamsUpdateRefused: null, clearTeamsUpdateRefused: vi.fn(),
  ...over,
})
const nameInput = (c, team) => c.querySelector(`input[aria-label="Nom de l'équipe ${team}"]`)
const delBtn = (c, team) => c.querySelector(`button[aria-label="Supprimer ${team}"]`)

describe('TeamsPage — renommage : identité = clé (#249)', () => {
  beforeEach(() => { useGame.mockReset(); vi.stubGlobal('confirm', () => true) })

  it('Critical — Regression : renommer envoie NAME = nouvelle clé (plus l’ancien nom), retire l’ancienne clé et déplace les joueurs', () => {
    const game = mk()
    useGame.mockReturnValue(game)
    const { container } = render(<TeamsPage />)

    const input = nameInput(container, 'Les Bleus')
    fireEvent.change(input, { target: { value: 'Les Azur' } })
    fireEvent.blur(input)

    expect(game.updateConfig).toHaveBeenCalledTimes(1)
    const sent = game.updateConfig.mock.calls[0][0]
    expect(Object.keys(sent.teams).sort()).toEqual(['Les Azur', 'Les Rouges'])
    expect(sent.teams['Les Azur'].NAME).toBe('Les Azur')
    expect(sent.teams['Les Azur'].SCORE).toBe(3) // le reste de l'objet équipe est conservé
    expect(sent.teams['Les Rouges'].NAME).toBe('Les Rouges')
    expect(sent.bumpers.vjP.TEAM).toBe('Les Azur')
    expect(sent.bumpers.vjM.TEAM).toBe('Les Rouges')
  })

  it('un nom vide ou inchangé n’envoie rien', () => {
    const game = mk()
    useGame.mockReturnValue(game)
    const { container } = render(<TeamsPage />)
    const input = nameInput(container, 'Les Bleus')
    fireEvent.blur(input)
    fireEvent.change(input, { target: { value: '  ' } })
    fireEvent.blur(input)
    expect(game.updateConfig).not.toHaveBeenCalled()
  })
})

describe('TeamsPage — équipes verrouillées pendant une question (#249, maquette)', () => {
  beforeEach(() => { useGame.mockReset(); vi.stubGlobal('confirm', () => true) })

  it('hors question : champs de nom et ✕ actifs, pas de bandeau', () => {
    useGame.mockReturnValue(mk())
    const { container } = render(<TeamsPage />)
    for (const team of Object.keys(TEAMS)) {
      expect(nameInput(container, team).disabled).toBe(false)
      expect(delBtn(container, team).disabled).toBe(false)
    }
    expect(container.textContent).not.toMatch(/Une question est en cours/)
  })

  it('Critical — TEAMS_LOCKED : champs de nom et ✕ désactivés pour chaque équipe, avec infobulle explicative', () => {
    useGame.mockReturnValue(mk({ gameState: { TEAMS_LOCKED: true } }))
    const { container } = render(<TeamsPage />)
    for (const team of Object.keys(TEAMS)) {
      const input = nameInput(container, team)
      const del = delBtn(container, team)
      expect(input.disabled, `nom de ${team}`).toBe(true)
      expect(del.disabled, `✕ de ${team}`).toBe(true)
      expect(input.getAttribute('title')).toMatch(/Pendant une question, on ne peut ni renommer ni supprimer une équipe\. Réessayez après la révélation de la réponse\./)
      expect(del.getAttribute('title')).toMatch(/Pendant une question, on ne peut ni renommer ni supprimer/)
    }
  })

  it('Critical — le bandeau explique le verrou et le champ de nom y renvoie (aria-describedby)', () => {
    useGame.mockReturnValue(mk({ gameState: { TEAMS_LOCKED: true } }))
    const { container } = render(<TeamsPage />)
    const banner = container.querySelector('#teams-lock-banner') || screen.getByText(/Une question est en cours/)
    expect(banner.textContent).toMatch(/Une question est en cours : renommer ou supprimer une équipe sera possible après la révélation de la réponse/)
    const describedBy = nameInput(container, 'Les Bleus').getAttribute('aria-describedby')
    expect(describedBy, 'aria-describedby').toBeTruthy()
    expect(container.querySelector(`#${describedBy}`)).toBe(banner)
  })

  it('verrouillé : une tentative de renommage (blur avec une nouvelle valeur) n’envoie rien', () => {
    const game = mk({ gameState: { TEAMS_LOCKED: true } })
    useGame.mockReturnValue(game)
    const { container } = render(<TeamsPage />)
    const input = nameInput(container, 'Les Bleus')
    fireEvent.change(input, { target: { value: 'Les Azur' } })
    fireEvent.blur(input)
    fireEvent.click(delBtn(container, 'Les Bleus'))
    expect(game.updateConfig).not.toHaveBeenCalled()
  })

  it('verrouillé : ajouter une équipe et changer une couleur restent possibles', () => {
    const game = mk({ gameState: { TEAMS_LOCKED: true } })
    useGame.mockReturnValue(game)
    const { container } = render(<TeamsPage />)

    const add = container.querySelector('.add-team-input')
    expect(add.disabled).toBe(false)
    fireEvent.change(add, { target: { value: 'Les Verts' } })
    fireEvent.submit(add.closest('form'))
    expect(game.updateConfig).toHaveBeenCalledTimes(1)
    expect(Object.keys(game.updateConfig.mock.calls[0][0].teams)).toContain('Les Verts')

    const colorButtons = container.querySelectorAll('.team-colors button')
    expect(colorButtons.length).toBeGreaterThan(0)
    for (const b of colorButtons) expect(b.disabled).toBe(false)
  })

  it('verrouillé : l’affectation des joueurs (glisser-déposer) reste possible — les joueurs ne sont pas désactivés', () => {
    useGame.mockReturnValue(mk({ gameState: { TEAMS_LOCKED: true } }))
    const { container } = render(<TeamsPage />)
    const draggable = [...container.querySelectorAll('[draggable="true"]')]
    expect(draggable.length).toBeGreaterThan(0)
  })

  it('le bandeau disparaît quand le verrou est levé (REVEALED)', () => {
    useGame.mockReturnValue(mk({ gameState: { TEAMS_LOCKED: true } }))
    const { container, rerender } = render(<TeamsPage />)
    expect(container.textContent).toMatch(/Une question est en cours/)
    useGame.mockReturnValue(mk({ gameState: { TEAMS_LOCKED: false } }))
    rerender(<TeamsPage />)
    expect(container.textContent).not.toMatch(/Une question est en cours/)
    expect(nameInput(container, 'Les Bleus').disabled).toBe(false)
  })
})

describe('TeamsPage — TEAMS_UPDATE_REFUSED (#249)', () => {
  beforeEach(() => { useGame.mockReset() })

  it('Critical — un refus du serveur affiche « Renommage refusé… » (équipe nommée) et le nom d’origine revient dans le champ', () => {
    const game = mk({ gameState: { TEAMS_LOCKED: true } })
    useGame.mockReturnValue(game)
    const { container, rerender } = render(<TeamsPage />)
    // course : le champ affichait une nouvelle valeur au moment du refus (état renvoyé par le serveur = équipes inchangées)
    const input = nameInput(container, 'Les Bleus')
    input.disabled = false
    fireEvent.change(input, { target: { value: 'Les Azur' } })

    const refused = mk({ gameState: { TEAMS_LOCKED: true }, teamsUpdateRefused: { reason: 'TEAMS_LOCKED_DURING_QUESTION', teams: ['Les Bleus'] } })
    useGame.mockReturnValue(refused)
    rerender(<TeamsPage />)

    const alert = screen.getByRole('alert')
    expect(alert.textContent).toMatch(/Renommage refusé : une question est en cours\./)
    expect(alert.textContent).toMatch(/« Les Bleus »/)
    expect(alert.textContent).toMatch(/n’a pas été renommée/)
    expect(nameInput(container, 'Les Bleus').value).toBe('Les Bleus')
    expect(refused.clearTeamsUpdateRefused).toHaveBeenCalled() // le refus est consommé une fois
  })

  it('course : une suppression envoyée juste avant le verrou, puis refusée, est dite « Suppression refusée »', () => {
    const game = mk() // l'interface n'est pas encore verrouillée au moment du clic
    useGame.mockReturnValue(game)
    vi.stubGlobal('confirm', () => true)
    const { container, rerender } = render(<TeamsPage />)
    fireEvent.click(delBtn(container, 'Les Rouges'))
    expect(game.updateConfig).toHaveBeenCalledTimes(1)

    useGame.mockReturnValue(mk({ gameState: { TEAMS_LOCKED: true }, teamsUpdateRefused: { reason: 'TEAMS_LOCKED_DURING_QUESTION', teams: ['Les Rouges'] } }))
    rerender(<TeamsPage />)

    expect(screen.getByRole('alert').textContent).toMatch(/Suppression refusée : une question est en cours\./)
    expect(screen.getByRole('alert').textContent).toMatch(/« Les Rouges »/)
  })

  it('pas de message sans refus', () => {
    useGame.mockReturnValue(mk())
    render(<TeamsPage />)
    expect(screen.queryByRole('alert')).toBeNull()
  })
})
