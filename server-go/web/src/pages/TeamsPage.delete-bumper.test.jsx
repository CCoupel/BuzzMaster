import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import TeamsPage from './TeamsPage'
import { useGame } from '../hooks/GameContext'

// ---------------------------------------------------------------------------
// Tests : TeamsPage — suppression d'un joueur via DELETE_BUMPER (#123, F1)
//
// Cause racine A1 (_work/reports/plan-20260730-094500.md) : l'interface
// n'émettait jamais DELETE_BUMPER — la suppression réelle passait par
// `updateConfig({ bumpers: newBumpers })`, un simple UPDATE de configuration
// amputé, qui ne notifie personne. Le handler serveur qui notifie
// (`handleDeleteBumper`, #120) était donc correct mais branché sur une
// action que personne n'envoyait. Ce test verrouille le chemin réellement
// emprunté par l'admin contre une régression future vers `updateConfig`.
//
// Mis à jour pour #122 (fast-follow cycle 2, _work/reports/
// plan-analysis-20260801-113000-122-verdict.md) : le bouton "×" d'un VJoueur
// n'appelle plus deleteBumper directement — il ouvre une confirmation
// (ReclaimConfirmModal) proposant Réinscription et Suppression totale,
// l'action la plus probable en premier selon le statut d'équipe. Un buzzer
// physique garde le comportement historique (confirm() natif), inchangé et
// toujours couvert ci-dessous.
// ---------------------------------------------------------------------------

vi.mock('../hooks/GameContext', () => ({
  useGame: vi.fn(),
}))

vi.mock('framer-motion', () => {
  const makeEl = (tag) => ({ children, initial, animate, exit, transition, whileHover, whileTap, ...props }) => {
    const Tag = tag
    return <Tag {...props}>{children}</Tag>
  }
  return {
    motion: { div: makeEl('div'), button: makeEl('button'), span: makeEl('span') },
    AnimatePresence: ({ children }) => children,
  }
})

vi.mock('../components/Button', () => ({
  default: ({ children, onClick, disabled, variant, size, fullWidth, ...rest }) => (
    <button onClick={onClick} disabled={disabled} {...rest}>{children}</button>
  ),
}))

vi.mock('../components/Card', () => ({
  default: ({ children, className, padding, variant, ...rest }) => (
    <div className={className} {...rest}>{children}</div>
  ),
}))

vi.mock('../components/TeamCard', () => ({
  OtaModal: () => null,
}))

vi.mock('./TeamsPage.css', () => ({}))

const makeGameMock = (bumpers = {}, teams = {}, gameStateOverrides = {}) => ({
  teams,
  bumpers,
  gameState: { virtualPlayerCount: 0, enrollmentActive: false, ...gameStateOverrides },
  updateConfig: vi.fn(),
  deleteBumper: vi.fn(),
  releaseBumperName: vi.fn(),
  showQRCode: vi.fn(),
  hideQRCode: vi.fn(),
  setVirtualPlayerLimit: vi.fn(),
})

describe('TeamsPage — suppression d\'un joueur émet DELETE_BUMPER, pas UPDATE (#123, F1)', () => {
  beforeEach(() => {
    vi.spyOn(window, 'confirm').mockReturnValue(true)
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('clique sur "×" d\'un VJoueur SANS équipe → confirmation, Suppression totale par défaut, appelle deleteBumper(mac)', () => {
    const mock = makeGameMock({
      'AA:BB:CC:DD:EE:01': { NAME: 'Alice', IS_VIRTUAL: true },
    })
    useGame.mockReturnValue(mock)

    render(<TeamsPage />)
    fireEvent.click(screen.getByTitle('Supprimer le joueur'))

    // Sans équipe : Suppression totale est la première option proposée.
    const buttons = screen.getAllByRole('button', { name: /Suppression totale|Réinscription/ })
    expect(buttons[0].textContent).toBe('Suppression totale')

    fireEvent.click(screen.getByRole('button', { name: 'Suppression totale' }))

    expect(mock.deleteBumper).toHaveBeenCalledWith('AA:BB:CC:DD:EE:01')
    expect(mock.updateConfig).not.toHaveBeenCalled()
  })

  it('fermer la confirmation (croix du dialogue) n\'appelle ni deleteBumper ni releaseBumperName', () => {
    const mock = makeGameMock({
      'AA:BB:CC:DD:EE:01': { NAME: 'Alice', IS_VIRTUAL: true },
    })
    useGame.mockReturnValue(mock)

    render(<TeamsPage />)
    fireEvent.click(screen.getByTitle('Supprimer le joueur'))
    expect(screen.getByRole('button', { name: 'Suppression totale' })).toBeTruthy()

    fireEvent.click(screen.getByLabelText('Fermer'))

    expect(mock.deleteBumper).not.toHaveBeenCalled()
    expect(mock.releaseBumperName).not.toHaveBeenCalled()
    expect(mock.updateConfig).not.toHaveBeenCalled()
    expect(screen.queryByRole('button', { name: 'Suppression totale' })).toBeNull()
  })

  it('VJoueur AVEC équipe → Réinscription proposée par défaut, mais Suppression totale reste accessible', () => {
    const mock = makeGameMock(
      { 'AA:BB:CC:DD:EE:01': { NAME: 'Alice', IS_VIRTUAL: true, TEAM: 'Rouges' } },
      { Rouges: { COLOR: [255, 0, 0], SCORE: 0 } },
    )
    useGame.mockReturnValue(mock)

    render(<TeamsPage />)
    fireEvent.click(screen.getAllByTitle('Supprimer le joueur')[0])

    const buttons = screen.getAllByRole('button', { name: /Suppression totale|Réinscription/ })
    expect(buttons[0].textContent).toBe('Réinscription')
    // La suppression totale reste possible malgré l'équipe (nécessaire pour
    // libérer une place contre VirtualPlayerLimit, cf. plan-analysis
    // 20260801 §3) : elle est présente, pas retirée.
    expect(screen.getByRole('button', { name: 'Suppression totale' })).toBeTruthy()

    fireEvent.click(screen.getByRole('button', { name: 'Suppression totale' }))
    expect(mock.deleteBumper).toHaveBeenCalledWith('AA:BB:CC:DD:EE:01')
  })

  it('VJoueur AVEC équipe → cliquer Réinscription appelle releaseBumperName(mac), pas deleteBumper', () => {
    const mock = makeGameMock(
      { 'AA:BB:CC:DD:EE:01': { NAME: 'Alice', IS_VIRTUAL: true, TEAM: 'Rouges' } },
      { Rouges: { COLOR: [255, 0, 0], SCORE: 0 } },
    )
    useGame.mockReturnValue(mock)

    render(<TeamsPage />)
    fireEvent.click(screen.getAllByTitle('Supprimer le joueur')[0])
    fireEvent.click(screen.getByRole('button', { name: 'Réinscription' }))

    expect(mock.releaseBumperName).toHaveBeenCalledWith('AA:BB:CC:DD:EE:01')
    expect(mock.deleteBumper).not.toHaveBeenCalled()
  })

  it('avertissement "inscriptions fermées" affiché quand enrollmentActive est faux', () => {
    const mock = makeGameMock(
      { 'AA:BB:CC:DD:EE:01': { NAME: 'Alice', IS_VIRTUAL: true } },
      {},
      { enrollmentActive: false },
    )
    useGame.mockReturnValue(mock)

    render(<TeamsPage />)
    fireEvent.click(screen.getByTitle('Supprimer le joueur'))

    expect(screen.getByText(/Inscriptions fermées/)).toBeTruthy()
  })

  it('avertissement "inscriptions fermées" absent quand enrollmentActive est vrai', () => {
    const mock = makeGameMock(
      { 'AA:BB:CC:DD:EE:01': { NAME: 'Alice', IS_VIRTUAL: true } },
      {},
      { enrollmentActive: true },
    )
    useGame.mockReturnValue(mock)

    render(<TeamsPage />)
    fireEvent.click(screen.getByTitle('Supprimer le joueur'))

    expect(screen.queryByText(/Inscriptions fermées/)).toBeNull()
  })

  it('le bouton "×" est toujours visible sur un VJoueur, qu\'il soit RECLAIM_REQUESTED ou non (le signal ne conditionne plus l\'affichage)', () => {
    const mock = makeGameMock({
      'AA:BB:CC:DD:EE:01': { NAME: 'Alice', IS_VIRTUAL: true, RECLAIM_REQUESTED: false },
    })
    useGame.mockReturnValue(mock)

    render(<TeamsPage />)
    expect(screen.getByTitle('Supprimer le joueur')).toBeTruthy()
  })

  it('fonctionne également pour un buzzer physique (même action dédiée)', () => {
    const mock = makeGameMock({
      'AA:BB:CC:DD:EE:02': { NAME: 'Buzzer1', IS_VIRTUAL: false },
    })
    useGame.mockReturnValue(mock)

    render(<TeamsPage />)
    fireEvent.click(screen.getByTitle('Supprimer le joueur'))

    expect(mock.deleteBumper).toHaveBeenCalledWith('AA:BB:CC:DD:EE:02')
    expect(mock.updateConfig).not.toHaveBeenCalled()
  })
})
